package polygon

import (
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	nats "github.com/nats-io/go-nats"
)

var servers string

func init() {
	if s := os.Getenv("POLYGON_NATS_SERVERS"); s != "" {
		servers = s
	} else {
		servers = "nats://nats1.polygon.io:30401, nats://nats2.polygon.io:30402, nats://nats3.polygon.io:30403"
	}
}

// Subscribe to the specified NATS stream with the supplied handler
func Subscribe(channel string, handler func(msg *nats.Msg)) error {
	c, err := nats.Connect(servers, nats.Token(common.Credentials().ID))
	if err != nil {
		return err
	}

	_, err = c.Subscribe(channel, handler)

	return err
}
