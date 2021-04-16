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
	bufferSize     int
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
		reconnectLimit: 20,
		reconnectDelay: 150 * time.Millisecond,
		processorCount: 1,
		bufferSize:     100000,
		trades:         []string{},
		tradeHandler:   func(t Trade) {},
		quotes:         []string{},
		quoteHandler:   func(q Quote) {},
		bars:           []string{},
		barHandler:     func(b Bar) {},
	}
}

// WithLogger configures the logger
func WithLogger(logger Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

// WithHost configures the host
func WithHost(host string) Option {
	return newFuncOption(func(o *options) {
		o.host = host
	})
}

// WithCredentials configures the key and secret to use
func WithCredentials(key, secret string) Option {
	return newFuncOption(func(o *options) {
		o.key = key
		o.secret = secret
	})
}

// WithReconnectSettings configures how many consecutive connection
// errors should be accepted and the delay (that is multipled by the number of consecutive errors)
// between retries
func WithReconnectSettings(limit int, delay time.Duration) Option {
	return newFuncOption(func(o *options) {
		o.reconnectLimit = limit
		o.reconnectDelay = delay
	})
}

// WithProcessors configures how many goroutines should process incoming
// messages. Increasing this past 1 means that the order of processing is not
// necessarily the same as the order of arrival the from server.
func WithProcessors(count int) Option {
	return newFuncOption(func(o *options) {
		o.processorCount = count
	})
}

// WithBufferSize sets the size for the buffer that is used for messages received
// from the server
func WithBufferSize(size int) Option {
	return newFuncOption(func(o *options) {
		o.bufferSize = size
	})
}

// WithTrades configures inital trade symbols to subscribe to and the handler
func WithTrades(handler func(Trade), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.trades = symbols
		o.tradeHandler = handler
	})
}

// WithQuotes configures inital quote symbols to subscribe to and the handler
func WithQuotes(handler func(Quote), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.quotes = symbols
		o.quoteHandler = handler
	})
}

// WithBars configures inital bar symbols to subscribe to and the handler
func WithBars(handler func(Bar), symbols ...string) Option {
	return newFuncOption(func(o *options) {
		o.bars = symbols
		o.barHandler = handler
	})
}
