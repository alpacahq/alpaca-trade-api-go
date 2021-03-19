package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/vmihailenco/msgpack/v5"
	"nhooyr.io/websocket"
)

var (
	// DataStreamURL is the URL for the data websocket stream.
	// The DATA_PROXY_WS environment variable overrides it.
	DataStreamURL = "https://stream.data.alpaca.markets"

	// MaxConnectionAttempts is the maximum number of retries for connecting to the websocket
	MaxConnectionAttempts = 3

	messageBufferSize = 1000
)

var (
	stream *datav2stream
)

type datav2stream struct {
	// opts
	feed string

	// connection flow
	conn          *websocket.Conn
	authenticated atomic.Value
	closed        atomic.Value

	// handlers
	tradeHandlers map[string]func(trade Trade)
	quoteHandlers map[string]func(quote Quote)
	barHandlers   map[string]func(bar Bar)

	// concurrency
	readerOnce    sync.Once
	wsWriteMutex  sync.Mutex
	wsReadMutex   sync.Mutex
	handlersMutex sync.RWMutex
}

func newDatav2Stream() *datav2stream {
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		DataStreamURL = s
	}
	stream = &datav2stream{
		feed:          "iex",
		authenticated: atomic.Value{},
		tradeHandlers: make(map[string]func(trade Trade)),
		quoteHandlers: make(map[string]func(quote Quote)),
		barHandlers:   make(map[string]func(bar Bar)),
	}

	stream.authenticated.Store(false)
	stream.closed.Store(false)

	return stream
}

func (s *datav2stream) useFeed(feed string) error {
	feed = strings.ToLower(feed)
	switch feed {
	case "iex", "sip":
	default:
		return errors.New("unsupported feed: " + feed)
	}
	if s.feed == feed {
		return nil
	}
	s.feed = feed
	if s.conn == nil {
		return nil
	}
	// we are already connected to the wrong feed
	// to restart it we close the stream and readForever will do the reconnect
	return s.close(false)
}

func (s *datav2stream) subscribeTrades(handler func(trade Trade), symbols ...string) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}

	if err := s.sub(symbols, nil, nil); err != nil {
		return err
	}

	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()

	for _, symbol := range symbols {
		s.tradeHandlers[symbol] = handler
	}

	return nil
}

func (s *datav2stream) subscribeQuotes(handler func(quote Quote), symbols ...string) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}

	if err := s.sub(nil, symbols, nil); err != nil {
		return err
	}

	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()

	for _, symbol := range symbols {
		s.quoteHandlers[symbol] = handler
	}

	return nil
}

func (s *datav2stream) subscribeBars(handler func(bar Bar), symbols ...string) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}

	if err := s.sub(nil, nil, symbols); err != nil {
		return err
	}

	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()

	for _, symbol := range symbols {
		s.barHandlers[symbol] = handler
	}

	return nil
}

func (s *datav2stream) unsubscribe(trades []string, quotes []string, bars []string) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}

	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()

	for _, trade := range trades {
		delete(s.tradeHandlers, trade)
	}
	for _, quote := range quotes {
		delete(s.quoteHandlers, quote)
	}
	for _, bar := range bars {
		delete(s.barHandlers, bar)
	}

	if err := s.unsub(trades, quotes, bars); err != nil {
		return err
	}

	return nil
}

func (s *datav2stream) close(final bool) error {
	if s.conn == nil {
		return nil
	}

	s.wsWriteMutex.Lock()
	defer s.wsWriteMutex.Unlock()

	if final {
		s.closed.Store(true)
	}

	if err := s.conn.Close(websocket.StatusNormalClosure, ""); err != nil {
		return err
	}
	s.conn = nil
	return nil
}

func (s *datav2stream) ensureRunning() error {
	if s.conn != nil {
		return nil
	}

	if err := s.connect(); err != nil {
		return err
	}
	s.readerOnce.Do(func() {
		go s.readForever()
	})
	return nil
}

func (s *datav2stream) connect() error {
	// first close any previous connections
	s.close(false)

	s.authenticated.Store(false)
	conn, err := openSocket(s.feed)
	if err != nil {
		return err
	}
	s.conn = conn
	if err := s.auth(); err != nil {
		return err
	}
	trades := make([]string, 0, len(s.tradeHandlers))
	for trade := range s.tradeHandlers {
		trades = append(trades, trade)
	}
	quotes := make([]string, 0, len(s.quoteHandlers))
	for quote := range s.quoteHandlers {
		quotes = append(quotes, quote)
	}
	bars := make([]string, 0)
	for bar := range s.barHandlers {
		bars = append(bars, bar)
	}
	return s.sub(trades, quotes, bars)
}

func (s *datav2stream) readForever() {
	msgs := make(chan []byte, messageBufferSize)
	defer close(msgs)
	go s.handleMessages(msgs)

	for {
		s.wsReadMutex.Lock()
		msgType, b, err := s.conn.Read(context.TODO())
		s.wsReadMutex.Unlock()

		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				// if this was a graceful closure, don't reconnect
				if s.closed.Load().(bool) {
					return
				}
			} else {
				log.Printf("alpaca stream read error (%v)", err)
			}

			err := s.connect()
			if err != nil {
				panic(err)
			}
		}
		if msgType != websocket.MessageBinary {
			continue
		}
		msgs <- b
	}
}

func (s *datav2stream) handleMessages(msgs <-chan []byte) {
	for msg := range msgs {
		if err := s.handleMessage(msg); err != nil {
			log.Printf("error handling incoming message: %v", err)
		}
	}
}

func (s *datav2stream) handleMessage(b []byte) error {
	d := msgpack.GetDecoder()
	defer msgpack.PutDecoder(d)

	reader := bytes.NewReader(b)
	d.Reset(reader)

	arrLen, err := d.DecodeArrayLen()
	if err != nil || arrLen < 1 {
		return err
	}

	for i := 0; i < arrLen; i++ {
		var n int
		n, err = d.DecodeMapLen()
		if err != nil {
			return err
		}
		if n < 1 {
			continue
		}

		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		if key != "T" {
			return fmt.Errorf("first key is not T but: %s", key)
		}
		T, err := d.DecodeString()
		if err != nil {
			return err
		}
		n-- // T already processed

		switch T {
		case "t":
			err = s.handleTrade(d, n)
		case "q":
			err = s.handleQuote(d, n)
		case "b":
			err = s.handleBar(d, n)
		default:
			err = s.handleOther(d, n)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *datav2stream) handleTrade(d *msgpack.Decoder, n int) error {
	trade := Trade{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "i":
			trade.ID, err = d.DecodeInt64()
		case "S":
			trade.Symbol, err = d.DecodeString()
		case "x":
			trade.Exchange, err = d.DecodeString()
		case "p":
			trade.Price, err = d.DecodeFloat64()
		case "s":
			trade.Size, err = d.DecodeUint32()
		case "t":
			trade.Timestamp, err = d.DecodeTime()
		case "c":
			var condCount int
			if condCount, err = d.DecodeArrayLen(); err != nil {
				return err
			}
			trade.Conditions = make([]string, condCount)
			for c := 0; c < condCount; c++ {
				if cond, err := d.DecodeString(); err != nil {
					return err
				} else {
					trade.Conditions[c] = cond
				}
			}
		case "z":
			trade.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	s.handlersMutex.RLock()
	defer s.handlersMutex.RUnlock()
	handler, ok := s.tradeHandlers[trade.Symbol]
	if !ok {
		if handler, ok = s.tradeHandlers["*"]; !ok {
			return nil
		}
	}
	handler(trade)
	return nil
}

func (s *datav2stream) handleQuote(d *msgpack.Decoder, n int) error {
	quote := Quote{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			quote.Symbol, err = d.DecodeString()
		case "bx":
			quote.BidExchange, err = d.DecodeString()
		case "bp":
			quote.BidPrice, err = d.DecodeFloat64()
		case "bs":
			quote.BidSize, err = d.DecodeUint32()
		case "ax":
			quote.AskExchange, err = d.DecodeString()
		case "ap":
			quote.AskPrice, err = d.DecodeFloat64()
		case "as":
			quote.AskSize, err = d.DecodeUint32()
		case "t":
			quote.Timestamp, err = d.DecodeTime()
		case "c":
			var condCount int
			if condCount, err = d.DecodeArrayLen(); err != nil {
				return err
			}
			quote.Conditions = make([]string, condCount)
			for c := 0; c < condCount; c++ {
				if cond, err := d.DecodeString(); err != nil {
					return err
				} else {
					quote.Conditions[c] = cond
				}
			}
		case "z":
			quote.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	s.handlersMutex.RLock()
	defer s.handlersMutex.RUnlock()
	handler, ok := s.quoteHandlers[quote.Symbol]
	if !ok {
		if handler, ok = s.quoteHandlers["*"]; !ok {
			return nil
		}
	}
	handler(quote)
	return nil
}

func (s *datav2stream) handleBar(d *msgpack.Decoder, n int) error {
	bar := Bar{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			bar.Symbol, err = d.DecodeString()
		case "o":
			bar.Open, err = d.DecodeFloat64()
		case "h":
			bar.High, err = d.DecodeFloat64()
		case "l":
			bar.Low, err = d.DecodeFloat64()
		case "c":
			bar.Close, err = d.DecodeFloat64()
		case "v":
			bar.Volume, err = d.DecodeUint64()
		case "t":
			bar.Timestamp, err = d.DecodeTime()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	s.handlersMutex.RLock()
	defer s.handlersMutex.RUnlock()
	handler, ok := s.barHandlers[bar.Symbol]
	if !ok {
		if handler, ok = s.barHandlers["*"]; !ok {
			return nil
		}
	}
	handler(bar)
	return nil
}

func (s *datav2stream) handleOther(d *msgpack.Decoder, n int) error {
	for i := 0; i < n; i++ {
		// key
		if err := d.Skip(); err != nil {
			return err
		}
		// value
		if err := d.Skip(); err != nil {
			return err
		}
	}
	return nil
}

func (s *datav2stream) sub(trades []string, quotes []string, bars []string) error {
	return s.handleSubscription(true, trades, quotes, bars)
}

func (s *datav2stream) unsub(trades []string, quotes []string, bars []string) error {
	return s.handleSubscription(false, trades, quotes, bars)
}

func (s *datav2stream) handleSubscription(subscribe bool, trades []string, quotes []string, bars []string) error {
	if len(trades)+len(quotes)+len(bars) == 0 {
		return nil
	}

	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}

	msg, err := msgpack.Marshal(map[string]interface{}{
		"action": action,
		"trades": trades,
		"quotes": quotes,
		"bars":   bars,
	})
	if err != nil {
		return err
	}

	s.wsWriteMutex.Lock()
	defer s.wsWriteMutex.Unlock()

	if err := s.conn.Write(context.TODO(), websocket.MessageBinary, msg); err != nil {
		return err
	}

	return nil
}

func (s *datav2stream) isAuthenticated() bool {
	return s.authenticated.Load().(bool)
}

func (s *datav2stream) auth() (err error) {
	if s.isAuthenticated() {
		return
	}

	msg, err := msgpack.Marshal(map[string]string{
		"action": "auth",
		"key":    common.Credentials().ID,
		"secret": common.Credentials().Secret,
	})
	if err != nil {
		return err
	}

	s.wsWriteMutex.Lock()
	defer s.wsWriteMutex.Unlock()

	if err := s.conn.Write(context.TODO(), websocket.MessageBinary, msg); err != nil {
		return err
	}

	var resps []map[string]interface{}

	// ensure the auth response comes in a timely manner
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	s.wsReadMutex.Lock()
	defer s.wsReadMutex.Unlock()

	_, b, err := s.conn.Read(ctx)
	if err != nil {
		return err
	}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) < 1 {
		return errors.New("received empty array")
	}
	if resps[0]["T"] == "error" {
		errString, ok := resps[0]["msg"].(string)
		if ok {
			return errors.New("failed to authorize: " + errString)
		}
	}
	if resps[0]["T"] != "success" || resps[0]["msg"] != "authenticated" {
		return errors.New("failed to authorize alpaca stream")
	}

	s.authenticated.Store(true)

	return
}

func openSocket(feed string) (*websocket.Conn, error) {
	scheme := "wss"
	ub, _ := url.Parse(DataStreamURL)
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: ub.Host, Path: "/v2/" + strings.ToLower(feed)}
	for attempts := 1; attempts <= MaxConnectionAttempts; attempts++ {
		c, _, err := websocket.Dial(context.TODO(), u.String(), &websocket.DialOptions{
			CompressionMode: websocket.CompressionContextTakeover,
			HTTPHeader: http.Header{
				"Content-Type": []string{"application/msgpack"},
			},
		})
		if err == nil {
			return c, readConnected(c)
		}
		log.Printf("failed to open Alpaca data stream: %v", err)
		if attempts == MaxConnectionAttempts {
			return nil, err
		}
		time.Sleep(time.Second)
	}
	return nil, errors.New("could not open Alpaca data stream (max retries exceeded)")
}

func readConnected(conn *websocket.Conn) error {
	_, b, err := conn.Read(context.TODO())
	if err != nil {
		return err
	}
	var resps []map[string]interface{}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) < 1 {
		return errors.New("received empty array")
	}
	if resps[0]["T"] != "success" || resps[0]["msg"] != "connected" {
		return errors.New("missing connected message")
	}
	return nil
}
