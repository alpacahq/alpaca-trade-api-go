package stream

import (
	"log"
	"sync"

	"github.com/alpacahq/alpaca-trade-api-go/alpaca"
)

var (
	once         sync.Once
	dataStream   *datav2stream
	alpacaStream *alpaca.Stream
)

func initStreamsOnce() {
	once.Do(func() {
		if dataStream == nil {
			dataStream = newDatav2Stream()
		}
		if alpacaStream == nil {
			alpacaStream = alpaca.GetStream()
		}
	})
}

// UseFeed sets the feed used by the data v2 stream. Supported feeds: iex, sip.
func UseFeed(feed string) error {
	initStreamsOnce()
	return dataStream.useFeed(feed)
}

// SubscribeTrades issues a subscribe command to the given symbols and
// registers the handler to be called for each trade.
func SubscribeTrades(handler func(trade Trade), symbols ...string) error {
	initStreamsOnce()
	return dataStream.subscribeTrades(handler, symbols...)
}

// SubscribeQuotes issues a subscribe command to the given symbols and
// registers the handler to be called for each quote.
func SubscribeQuotes(handler func(quote Quote), symbols ...string) error {
	initStreamsOnce()
	return dataStream.subscribeQuotes(handler, symbols...)
}

// SubscribeBars issues a subscribe command to the given symbols and
// registers the handler to be called for each bar.
func SubscribeBars(handler func(bar Bar), symbols ...string) error {
	initStreamsOnce()
	return dataStream.subscribeBars(handler, symbols...)
}

// SubscribeTradeUpdates issues a subscribe command to the user's trade updates and
// registers the handler to be called for each update.
func SubscribeTradeUpdates(handler func(update alpaca.TradeUpdate)) error {
	initStreamsOnce()
	return alpacaStream.Subscribe(alpaca.TradeUpdates, func(msg interface{}) {
		update, ok := msg.(alpaca.TradeUpdate)
		if !ok {
			log.Printf("unexpected trade update: %v", msg)
			return
		}
		handler(update)
	})
}

// UnsubscribeTrades issues an unsubscribe command for the given trade symbols
func UnsubscribeTrades(symbols ...string) error {
	initStreamsOnce()
	return dataStream.unsubscribe(symbols, nil, nil)
}

// UnsubscribeQuotes issues an unsubscribe command for the given quote symbols
func UnsubscribeQuotes(symbols ...string) error {
	initStreamsOnce()
	return dataStream.unsubscribe(nil, symbols, nil)
}

// UnsubscribeBars issues an unsubscribe command for the given bar symbols
func UnsubscribeBars(symbols ...string) error {
	initStreamsOnce()
	return dataStream.unsubscribe(nil, nil, symbols)
}

// UnsubscribeTradeUpdates issues an unsubscribe command for the user's trade updates
func UnsubscribeTradeUpdates() error {
	initStreamsOnce()
	return alpacaStream.Unsubscribe(alpaca.TradeUpdates)
}

// Close gracefully closes all streams
func Close() error {
	var alpacaErr, dataErr error
	if alpacaStream != nil {
		alpacaErr = alpacaStream.Close()
	}
	if dataStream != nil {
		dataErr = dataStream.close(true)
	}
	if alpacaErr != nil {
		return alpacaErr
	}
	return dataErr
}
