package new

import (
	"os"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
)

type options struct {
	logger         Logger
	host           string
	key            string
	secret         string
	reconnectLimit int
	reconnectDelay time.Duration
	processorCount int
	trades         []string
	tradeHandler   func(Trade)
	quotes         []string
	quoteHandler   func(Quote)
	bars           []string
	barHandler     func(Bar)
}

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt.apply(o)
	}
}

type Option interface {
	apply(*options)
}

type funcOption struct {
	f func(*options)
}

func (fo *funcOption) apply(o *options) {
	fo.f(o)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// defaultOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultOptions() *options {
	host := "https://stream.data.alpaca.markets"
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		host = s
	}

	return &options{
		logger:         newStdLog(),
		host:           host,
		key:            common.Credentials().ID,
		secret:         common.Credentials().Secret,
		reconnectLimit: 15,
		reconnectDelay: 100 * time.Millisecond,
		processorCount: 1,
		trades:         []string{},
		tradeHandler:   func(t Trade) {},
		quotes:         []string{},
		quoteHandler:   func(q Quote) {},
		bars:           []string{},
		barHandler:     func(b Bar) {},
	}
}

func (c *client) configure(o options) {
	c.logger = o.logger
	c.host = o.host
	c.key = o.key
	c.secret = o.secret
	c.reconnectLimit = o.reconnectLimit
	c.reconnectDelay = o.reconnectDelay
	c.processorCount = o.processorCount
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.trades = o.trades
	c.tradeHandler = o.tradeHandler
	c.quotes = o.quotes
	c.quoteHandler = o.quoteHandler
	c.bars = o.bars
	c.barHandler = o.barHandler
}

func WithLogger(logger Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

func WithHost(host string) Option {
	return newFuncOption(func(o *options) {
		o.host = host
	})
}

func WithCredentials(key, secret string) Option {
	return newFuncOption(func(o *options) {
		o.key = key
		o.secret = secret
	})
}

func WithReconnectSettings(limit int, delay time.Duration) Option {
	return newFuncOption(func(o *options) {
		o.reconnectLimit = limit
		o.reconnectDelay = delay
	})
}

func WithProcessors(count int) Option {
	return newFuncOption(func(o *options) {
		o.processorCount = count
	})
}

func WithTrades(handler func(Trade), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.trades = symbols
		o.tradeHandler = handler
	})
}

func WithQuotes(handler func(Quote), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.quotes = symbols
		o.quoteHandler = handler
	})
}

func WithBars(handler func(Bar), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.bars = symbols
		o.barHandler = handler
	})
}
