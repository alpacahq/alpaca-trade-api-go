package alpaca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// StreamTradeUpdates listens to trade updates as long as the given context is not cancelled
// and calls the given handler for each update.
func (c *client) StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	s := getStream(c.opts.BaseURL, c.opts.ApiKey, c.opts.ApiSecret)
	if err := s.Subscribe("trade_updates", func(msg interface{}) {
		handler(msg.(TradeUpdate))
	}); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	return nil
}

// StreamTradeUpdates listens to trade updates as long as the given context is not cancelled
// and calls the given handler for each update.
func StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	return DefaultClient.StreamTradeUpdates(ctx, handler)
}

////////////////////////////////////////////////////////////////////////////////
// This implementation is no longer exported!
// TODO: Replace it with a much easier SSE one

const (
	tradeUpdates   = "trade_updates"
	accountUpdates = "account_updates"
)

const (
	maxConnectionAttempts = 3
)

var (
	once sync.Once
	str  *wsStream
)

type wsStream struct {
	sync.Mutex
	sync.Once
	conn                  *websocket.Conn
	authenticated, closed atomic.Value
	handlers              sync.Map
	base                  string
	apiKey, apiSecret     string
}

// Subscribe to the specified Alpaca stream channel.
func (s *wsStream) Subscribe(channel string, handler func(msg interface{})) (err error) {
	switch {
	case channel == tradeUpdates:
		fallthrough
	case channel == accountUpdates:
		fallthrough
	case strings.HasPrefix(channel, "Q."):
		fallthrough
	case strings.HasPrefix(channel, "T."):
		fallthrough
	case strings.HasPrefix(channel, "AM."):
	default:
		err = fmt.Errorf("invalid stream (%s)", channel)
		return
	}
	if s.conn == nil {
		s.conn, err = s.openSocket()
		if err != nil {
			return
		}
	}

	if err = s.auth(); err != nil {
		return
	}
	s.Do(func() {
		go s.start()
	})

	s.handlers.Store(channel, handler)

	if err = s.sub(channel); err != nil {
		s.handlers.Delete(channel)
		return
	}
	return
}

// Unsubscribe the specified Polygon stream channel.
func (s *wsStream) Unsubscribe(channel string) (err error) {
	if s.conn == nil {
		err = errors.New("not yet subscribed to any channel")
		return
	}

	if err = s.auth(); err != nil {
		return
	}

	s.handlers.Delete(channel)

	err = s.unsub(channel)

	return
}

// Close gracefully closes the Alpaca stream.
func (s *wsStream) Close() error {
	s.Lock()
	defer s.Unlock()

	if s.conn == nil {
		return nil
	}

	if err := s.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	); err != nil {
		return err
	}

	// so we know it was gracefully closed
	s.closed.Store(true)

	return s.conn.Close()
}

func (s *wsStream) reconnect() error {
	s.authenticated.Store(false)
	conn, err := s.openSocket()
	if err != nil {
		return err
	}
	s.conn = conn
	if err := s.auth(); err != nil {
		return err
	}
	s.handlers.Range(func(key, value interface{}) bool {
		// there should be no errors if we've previously successfully connected
		s.sub(key.(string))
		return true
	})
	return nil
}

func (s *wsStream) findHandler(stream string) func(interface{}) {
	if v, ok := s.handlers.Load(stream); ok {
		return v.(func(interface{}))
	}
	if strings.HasPrefix(stream, "Q.") ||
		strings.HasPrefix(stream, "T.") ||
		strings.HasPrefix(stream, "AM.") {
		c := stream[:strings.Index(stream, ".")]
		if v, ok := s.handlers.Load(c + ".*"); ok {
			return v.(func(interface{}))
		}
	}
	return nil
}

func (s *wsStream) start() {
	for {
		msg := serverMsg{}

		if err := s.conn.ReadJSON(&msg); err == nil {
			handler := s.findHandler(msg.Stream)
			if handler != nil {
				msgBytes, _ := json.Marshal(msg.Data)
				switch {
				case msg.Stream == tradeUpdates:
					var tradeupdate TradeUpdate
					json.Unmarshal(msgBytes, &tradeupdate)
					handler(tradeupdate)
				case strings.HasPrefix(msg.Stream, "Q."):
					var quote streamQuote
					json.Unmarshal(msgBytes, &quote)
					handler(quote)
				case strings.HasPrefix(msg.Stream, "T."):
					var trade streamTrade
					json.Unmarshal(msgBytes, &trade)
					handler(trade)
				case strings.HasPrefix(msg.Stream, "AM."):
					var agg streamAgg
					json.Unmarshal(msgBytes, &agg)
					handler(agg)

				default:
					handler(msg.Data)
				}
			}
		} else {
			if websocket.IsCloseError(err) {
				// if this was a graceful closure, don't reconnect
				if s.closed.Load().(bool) {
					return
				}
			} else {
				log.Printf("alpaca stream read error (%v)", err)
			}

			err := s.reconnect()
			if err != nil {
				panic(err)
			}
		}
	}
}

func (s *wsStream) sub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := clientMsg{
		Action: "listen",
		Data: map[string]interface{}{
			"streams": []interface{}{
				channel,
			},
		},
	}

	if err = s.conn.WriteJSON(subReq); err != nil {
		return
	}

	return
}

func (s *wsStream) unsub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := clientMsg{
		Action: "unlisten",
		Data: map[string]interface{}{
			"streams": []interface{}{
				channel,
			},
		},
	}

	if err = s.conn.WriteJSON(subReq); err != nil {
		return
	}

	return
}

func (s *wsStream) isAuthenticated() bool {
	return s.authenticated.Load().(bool)
}

func (s *wsStream) auth() (err error) {
	s.Lock()
	defer s.Unlock()

	if s.isAuthenticated() {
		return
	}

	authRequest := clientMsg{
		Action: "authenticate",
		Data: map[string]interface{}{
			"key_id":     s.apiKey,
			"secret_key": s.apiSecret,
		},
	}

	if err = s.conn.WriteJSON(authRequest); err != nil {
		return
	}

	msg := serverMsg{}

	// ensure the auth response comes in a timely manner
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer s.conn.SetReadDeadline(time.Time{})

	if err = s.conn.ReadJSON(&msg); err != nil {
		return
	}

	m := msg.Data.(map[string]interface{})

	if !strings.EqualFold(m["status"].(string), "authorized") {
		return fmt.Errorf("failed to authorize alpaca stream")
	}

	s.authenticated.Store(true)

	return
}

func getStream(baseURL, apiKey, apiSecret string) *wsStream {
	once.Do(func() {
		str = &wsStream{
			authenticated: atomic.Value{},
			handlers:      sync.Map{},
			base:          baseURL,
			apiKey:        apiKey,
			apiSecret:     apiSecret,
		}

		str.authenticated.Store(false)
		str.closed.Store(false)
	})

	return str
}

func (s *wsStream) openSocket() (*websocket.Conn, error) {
	scheme := "wss"
	ub, _ := url.Parse(s.base)
	if ub.Scheme == "http" {
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: ub.Host, Path: "/stream"}
	connectionAttempts := 0
	for connectionAttempts < maxConnectionAttempts {
		connectionAttempts++
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err == nil {
			return c, nil
		}
		if connectionAttempts == maxConnectionAttempts {
			return nil, err
		}
		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("could not open Alpaca stream (max retries exceeded)")
}
