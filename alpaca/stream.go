package alpaca

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/gorilla/websocket"
)

const (
	TradeUpdates   = "trade_updates"
	AccountUpdates = "account_updates"
)

const (
	MaxConnectionAttempts = 3
)

var (
	once      sync.Once
	str       *Stream
	streamUrl = ""

	dataOnce sync.Once
	dataStr  *Stream
)

type Stream struct {
	sync.Mutex
	sync.Once
	conn                  *websocket.Conn
	authenticated, closed atomic.Value
	handlers              sync.Map
	base                  string
}

// Subscribe to the specified Alpaca stream channel.
func (s *Stream) Subscribe(channel string, handler func(msg interface{})) (err error) {
	switch {
	case channel == TradeUpdates:
		fallthrough
	case channel == AccountUpdates:
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
func (s *Stream) Unsubscribe(channel string) (err error) {
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
func (s *Stream) Close() error {
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

func (s *Stream) reconnect() error {
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

func (s *Stream) findHandler(stream string) func(interface{}) {
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

func (s *Stream) start() {
	for {
		msg := ServerMsg{}

		if err := s.conn.ReadJSON(&msg); err == nil {
			handler := s.findHandler(msg.Stream)
			if handler != nil {
				msgBytes, _ := json.Marshal(msg.Data)
				switch {
				case msg.Stream == TradeUpdates:
					var tradeupdate TradeUpdate
					json.Unmarshal(msgBytes, &tradeupdate)
					handler(tradeupdate)
				case strings.HasPrefix(msg.Stream, "Q."):
					var quote StreamQuote
					json.Unmarshal(msgBytes, &quote)
					handler(quote)
				case strings.HasPrefix(msg.Stream, "T."):
					var trade StreamTrade
					json.Unmarshal(msgBytes, &trade)
					handler(trade)
				case strings.HasPrefix(msg.Stream, "AM."):
					var agg StreamAgg
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

func (s *Stream) sub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := ClientMsg{
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

func (s *Stream) unsub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := ClientMsg{
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

func (s *Stream) isAuthenticated() bool {
	return s.authenticated.Load().(bool)
}

func (s *Stream) auth() (err error) {
	s.Lock()
	defer s.Unlock()

	if s.isAuthenticated() {
		return
	}

	authRequest := ClientMsg{
		Action: "authenticate",
		Data: map[string]interface{}{
			"key_id":     common.Credentials().ID,
			"secret_key": common.Credentials().Secret,
		},
	}

	if err = s.conn.WriteJSON(authRequest); err != nil {
		return
	}

	msg := ServerMsg{}

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

// GetStream returns the singleton Alpaca stream structure.
func GetStream() *Stream {
	once.Do(func() {
		str = &Stream{
			authenticated: atomic.Value{},
			handlers:      sync.Map{},
			base:          base,
		}

		str.authenticated.Store(false)
		str.closed.Store(false)
	})

	return str
}

func GetDataStream() *Stream {
	dataOnce.Do(func() {
		if s := os.Getenv("DATA_PROXY_WS"); s != "" {
			streamUrl = s
		} else {
			streamUrl = dataUrl
		}
		dataStr = &Stream{
			authenticated: atomic.Value{},
			handlers:      sync.Map{},
			base:          streamUrl,
		}

		dataStr.authenticated.Store(false)
		dataStr.closed.Store(false)
	})

	return dataStr
}

func (s *Stream) openSocket() (*websocket.Conn, error) {
	scheme := "wss"
	ub, _ := url.Parse(s.base)
	if ub.Scheme == "http" {
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: ub.Host, Path: "/stream"}
	connectionAttempts := 0
	for connectionAttempts < MaxConnectionAttempts {
		connectionAttempts++
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err == nil {
			return c, nil
		}
		if connectionAttempts == MaxConnectionAttempts {
			return nil, err
		}
		time.Sleep(1 * time.Second)
	}
	return nil, fmt.Errorf("Error: Could not open Alpaca stream (max retries exceeded).")
}
