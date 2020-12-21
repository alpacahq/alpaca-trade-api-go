package polygon

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/gorilla/websocket"
)

const (
	MinuteAggs = "AM"
	SecondAggs = "A"
	Trades     = "T"
	Quotes     = "Q"
)

const (
	MaxConnectionAttempts = 3
)

var (
	once sync.Once
	str  *Stream
)

type Stream struct {
	sync.Mutex
	sync.Once
	conn                  *websocket.Conn
	authenticated, closed atomic.Value
	handlers              sync.Map
}

// Subscribe to the specified Polygon stream channel.
func (s *Stream) Subscribe(channel string, handler func(msg interface{})) (err error) {
	if s.conn == nil {
		s.conn = openSocket()
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

// Close gracefully closes the Polygon stream.
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

func (s *Stream) reconnect() {
	s.authenticated.Store(false)
	s.conn = openSocket()
	if err := s.auth(); err != nil {
		return
	}
	s.handlers.Range(func(key, value interface{}) bool {
		// there should be no errors if we've previously successfully connected
		s.sub(key.(string))
		return true
	})
}

func (s *Stream) handleError(err error) {
	if websocket.IsCloseError(err) {
		// if this was a graceful closure, don't reconnect
		if s.closed.Load().(bool) {
			return
		}
	} else {
		log.Printf("polygon stream read error (%v)", err)
	}

	s.reconnect()
}

func (s *Stream) start() {
	for {
		if _, arrayBytes, err := s.conn.ReadMessage(); err == nil {
			msgArray := []interface{}{}
			if err := json.Unmarshal(arrayBytes, &msgArray); err == nil {
				for _, msg := range msgArray {
					msgMap := msg.(map[string]interface{})
					channel := fmt.Sprintf("%s.%s", msgMap["ev"], msgMap["sym"])
					handler, ok := s.handlers.Load(channel)
					if !ok {
						// see if an "all symbols" handler was registered
						handler, ok = s.handlers.Load(fmt.Sprintf("%s.*", msgMap["ev"]))
					}
					if ok {
						msgBytes, _ := json.Marshal(msg)
						switch msgMap["ev"] {
						case SecondAggs:
							fallthrough
						case MinuteAggs:
							var minuteAgg StreamAggregate
							if err := json.Unmarshal(msgBytes, &minuteAgg); err == nil {
								h := handler.(func(msg interface{}))
								h(minuteAgg)
							} else {
								s.handleError(err)
							}
						case Quotes:
							var quoteUpdate StreamQuote
							if err := json.Unmarshal(msgBytes, &quoteUpdate); err == nil {
								h := handler.(func(msg interface{}))
								h(quoteUpdate)
							} else {
								s.handleError(err)
							}
						case Trades:
							var tradeUpdate StreamTrade
							if err := json.Unmarshal(msgBytes, &tradeUpdate); err == nil {
								h := handler.(func(msg interface{}))
								h(tradeUpdate)
							} else {
								s.handleError(err)
							}
						}
					} else {

					}
				}
			} else {
				s.handleError(err)
			}
		} else {
			s.handleError(err)
		}
	}
}

func (s *Stream) sub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := PolygonClientMsg{
		Action: "subscribe",
		Params: channel,
	}

	if err = s.conn.WriteJSON(subReq); err != nil {
		return
	}

	return
}

func (s *Stream) unsub(channel string) (err error) {
	s.Lock()
	defer s.Unlock()

	subReq := PolygonClientMsg{
		Action: "unsubscribe",
		Params: channel,
	}

	err = s.conn.WriteJSON(subReq)

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

	authRequest := PolygonClientMsg{
		Action: "auth",
		Params: common.Credentials().PolygonKeyID,
	}

	if err = s.conn.WriteJSON(authRequest); err != nil {
		return
	}

	msg := []PolygonAuthMsg{}

	// ensure the auth response comes in a timely manner
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer s.conn.SetReadDeadline(time.Time{})

	if err = s.conn.ReadJSON(&msg); err != nil {
		return
	}

	if !strings.EqualFold(msg[0].Status, "auth_success") {
		return fmt.Errorf("failed to authorize Polygon stream")
	}

	s.authenticated.Store(true)

	return
}

// GetStream returns the singleton Polygon stream structure.
func GetStream() *Stream {
	once.Do(func() {
		str = &Stream{
			authenticated: atomic.Value{},
			handlers:      sync.Map{},
		}

		str.authenticated.Store(false)
		str.closed.Store(false)
	})

	return str
}

func openSocket() *websocket.Conn {
	polygonStreamEndpoint, ok := os.LookupEnv("POLYGON_WS_URL")
	if !ok {
		polygonStreamEndpoint = "wss://socket.polygon.io/stocks"
	}
	connectionAttempts := 0
	for connectionAttempts < MaxConnectionAttempts {
		connectionAttempts++
		c, _, err := websocket.DefaultDialer.Dial(polygonStreamEndpoint, nil)
		if err != nil {
			if connectionAttempts == MaxConnectionAttempts {
				panic(err)
			}
		} else {
			// consume connection message
			msg := []PolgyonServerMsg{}
			if err = c.ReadJSON(&msg); err == nil {
				return c
			}
		}
		time.Sleep(1 * time.Second)
	}
	panic(fmt.Errorf("Error: Could not open Polygon stream (max retries exceeded)."))
}
