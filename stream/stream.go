package stream

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/alpacahq/alpaca-trade-api-go/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/polygon"
)

var (
	once sync.Once
	u    *Unified

	dataStreamName string = "alpaca"
)

func SetDataStream(streamName string) {
	switch streamName {
	case "alpaca":
	case "polygon":
		dataStreamName = streamName
	default:
		fmt.Fprintf(os.Stderr, "invalid data stream name %s\n", streamName)
	}
}

// Register a handler for a given stream, Alpaca or Polygon.
func Register(stream string, handler func(msg interface{})) (err error) {
	once.Do(func() {
		if u == nil {

			var dataStream Stream
			if dataStreamName == "alpaca" {
				dataStream = alpaca.GetDataStream()
			} else if dataStreamName == "polygon" {
				dataStream = polygon.GetStream()
			}
			u = &Unified{
				alpaca: alpaca.GetStream(),
				data:   dataStream,
			}
		}
	})

	switch stream {
	case alpaca.TradeUpdates:
		fallthrough
	case alpaca.AccountUpdates:
		err = u.alpaca.Subscribe(stream, handler)
	default:
		// data stream
		err = u.data.Subscribe(stream, handler)
	}

	return
}

// Deregister a handler for a given stream, Alpaca or Polygon.
func Deregister(stream string) (err error) {
	once.Do(func() {
		if u == nil {
			err = errors.New("not yet subscribed to any channel")
			return
		}
	})

	switch stream {
	case alpaca.TradeUpdates:
		fallthrough
	case alpaca.AccountUpdates:
		err = u.alpaca.Unsubscribe(stream)
	default:
		// data stream
		err = u.data.Unsubscribe(stream)
	}

	return
}

// Close gracefully closes all streams
func Close() error {
	// close alpaca connection
	err1 := u.alpaca.Close()
	// close polygon connection
	err2 := u.data.Close()

	if err1 != nil {
		return err1
	}
	return err2
}

// Unified is the unified streaming structure combining the
// interfaces from polygon and alpaca.
type Unified struct {
	alpaca, data Stream
}

// Stream is the generic streaming interface implemented by
// both alpaca and polygon.
type Stream interface {
	Subscribe(key string, handler func(msg interface{})) error
	Unsubscribe(key string) error
	Close() error
}
