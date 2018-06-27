package polygon

import (
	"os"
	"sync"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	nats "github.com/nats-io/go-nats"
)

var (
	servers string
	str     *Stream
)

type Stream struct {
	conn     *nats.Conn
	handlers sync.Map
}

func init() {
	if s := os.Getenv("POLYGON_NATS_SERVERS"); s != "" {
		servers = s
	} else {
		servers = "nats://nats1.polygon.io:30401, nats://nats2.polygon.io:30402, nats://nats3.polygon.io:30403"
	}

	str = &Stream{handlers: sync.Map{}}
}

func GetStream() *Stream {
	return str
}

// Subscribe to the specified NATS stream with the supplied handler
func (s *Stream) Subscribe(channel string, handler func(msg interface{})) (err error) {
	if s.conn == nil {
		c, err := nats.Connect(servers, nats.Token(common.Credentials().ID))
		if err != nil {
			return err
		}
		s.conn = c
	}
	h := func(msg *nats.Msg) {
		handler(msg)
	}

	_, err = s.conn.Subscribe(channel, h)

	return
}

// Close the polygon NATS connection gracefully
func (s *Stream) Close() error {
	s.conn.Close()
	return nil
}
