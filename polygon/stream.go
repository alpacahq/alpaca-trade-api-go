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
	once    sync.Once
)

type Stream struct {
	conn *nats.Conn
}

func GetStream() *Stream {
	once.Do(func() {
		str = &Stream{}
	})

	return str
}

// Subscribe to the specified NATS stream with the supplied handler
func (s *Stream) Subscribe(channel string, handler func(msg interface{})) (err error) {
	if s.conn == nil {
		servers, ok := os.LookupEnv("POLYGON_NATS_SERVERS")
		if !ok {
			servers = "nats://nats1.polygon.io:31101, nats://nats2.polygon.io:31102, nats://nats3.polygon.io:31103"
		}
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
