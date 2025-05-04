package stream

import (
	"context"
	"net/url"
	"os"
	"time"
)

// StockOption is a configuration option for the StockClient
type StockOption interface {
	applyStock(*stockOptions)
}

// CryptoOption is a configuration option for the CryptoClient
type CryptoOption interface {
	applyCrypto(*cryptoOptions)
}

type OptionOption interface {
	applyOption(*optionOptions)
}

type NewsOption interface {
	applyNews(*newsOptions)
}

// Option is a configuration option that can be used for both StockClient and CryptoClient
type Option interface {
	StockOption
	CryptoOption
	OptionOption
	NewsOption
}

type options struct {
	logger             Logger
	baseURL            string
	key                string
	secret             string
	reconnectLimit     int
	reconnectDelay     time.Duration
	connectCallback    func()
	bufferFillCallback func([]byte)
	disconnectCallback func()
	processorCount     int
	bufferSize         int
	sub                subscriptions

	// for testing only
	connCreator func(ctx context.Context, u url.URL) (conn, error)
}

type funcOption struct {
	f func(*options)
}

func (fo *funcOption) applyCrypto(o *cryptoOptions) {
	fo.f(&o.options)
}

func (fo *funcOption) applyStock(o *stockOptions) {
	fo.f(&o.options)
}

func (fo *funcOption) applyOption(o *optionOptions) {
	fo.f(&o.options)
}

func (fo *funcOption) applyNews(o *newsOptions) {
	fo.f(&o.options)
}

func newFuncOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

// WithLogger configures the logger
func WithLogger(logger Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

// WithBaseURL configures the base URL
func WithBaseURL(url string) Option {
	return newFuncOption(func(o *options) {
		o.baseURL = url
	})
}

// WithCredentials configures the key and secret to use
func WithCredentials(key, secret string) Option {
	return newFuncOption(func(o *options) {
		if key != "" {
			o.key = key
		}
		if secret != "" {
			o.secret = secret
		}
	})
}

// WithReconnectSettings configures how many consecutive connection
// errors should be accepted and the delay (that is multiplied by the number of consecutive errors)
// between retries. limit = 0 means the client will try restarting indefinitely unless it runs into
// an irrecoverable error (such as invalid credentials).
func WithReconnectSettings(limit int, delay time.Duration) Option {
	return newFuncOption(func(o *options) {
		o.reconnectLimit = limit
		o.reconnectDelay = delay
	})
}

// WithConnectCallback runs the callback function after the streaming connection is setup.
// If the stream terminates and can't reconnect, the connect callback will timeout one second
// after reaching the end of the stream's maintenance (if it is still running). This is to avoid
// the callback blocking the parent thread.
func WithConnectCallback(callback func()) Option {
	return newFuncOption(func(o *options) {
		o.connectCallback = callback
	})
}

// WithBufferFillCallback runs the callback function whenever the buffer is full
// and msg cannot be delivered. This usually happens when trade/quote handlers
// process the messages slowly and they cannot keep up with the pace how messages
// are received. This callback should run fast, so avoid any blocking
// instructions in the callback.
func WithBufferFillCallback(callback func(msg []byte)) Option {
	return newFuncOption(func(o *options) {
		o.bufferFillCallback = callback
	})
}

// WithDisconnectCallback runs the callback function after the streaming connection disconnects.
// If the stream is terminated and can't reconnect, the disconnect callback will timeout one second
// after reaching the end of the stream's maintenance (if it is still running). This is to avoid
// the callback blocking the parent thread.
func WithDisconnectCallback(callback func()) Option {
	return newFuncOption(func(o *options) {
		o.disconnectCallback = callback
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

func withConnCreator(connCreator func(ctx context.Context, u url.URL) (conn, error)) Option {
	return newFuncOption(func(o *options) {
		o.connCreator = connCreator
	})
}

type stockOptions struct {
	options
	tradeHandler         func(Trade)
	quoteHandler         func(Quote)
	barHandler           func(Bar)
	updatedBarHandler    func(Bar)
	dailyBarHandler      func(Bar)
	tradingStatusHandler func(TradingStatus)
	imbalanceHandler     func(Imbalance)
	luldHandler          func(LULD)
	cancelErrorHandler   func(TradeCancelError)
	correctionHandler    func(TradeCorrection)
}

// defaultStockOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultStockOptions() *stockOptions {
	baseURL := "https://stream.data.alpaca.markets/v2"
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		baseURL = s
	}

	return &stockOptions{
		options: options{
			logger:         DefaultLogger(),
			baseURL:        baseURL,
			key:            os.Getenv("APCA_API_KEY_ID"),
			secret:         os.Getenv("APCA_API_SECRET_KEY"),
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100000,
			sub: subscriptions{
				trades:       []string{},
				quotes:       []string{},
				bars:         []string{},
				updatedBars:  []string{},
				dailyBars:    []string{},
				statuses:     []string{},
				imbalances:   []string{},
				lulds:        []string{},
				cancelErrors: []string{},
				corrections:  []string{},
			},
			connCreator: newCoderWebsocketConn,
		},
		tradeHandler:         func(_ Trade) {},
		quoteHandler:         func(_ Quote) {},
		barHandler:           func(_ Bar) {},
		updatedBarHandler:    func(_ Bar) {},
		dailyBarHandler:      func(_ Bar) {},
		tradingStatusHandler: func(_ TradingStatus) {},
		imbalanceHandler:     func(_ Imbalance) {},
		luldHandler:          func(_ LULD) {},
		cancelErrorHandler:   func(_ TradeCancelError) {},
		correctionHandler:    func(_ TradeCorrection) {},
	}
}

func (o *stockOptions) applyStock(opts ...StockOption) {
	for _, opt := range opts {
		opt.applyStock(o)
	}
}

type funcStockOption struct {
	f func(*stockOptions)
}

func (fo *funcStockOption) applyStock(o *stockOptions) {
	fo.f(o)
}

func newFuncStockOption(f func(*stockOptions)) StockOption {
	return &funcStockOption{
		f: f,
	}
}

// WithTrades configures initial trade symbols to subscribe to and the handler
func WithTrades(handler func(Trade), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.trades = symbols
		o.tradeHandler = handler
	})
}

// WithQuotes configures initial quote symbols to subscribe to and the handler
func WithQuotes(handler func(Quote), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.quotes = symbols
		o.quoteHandler = handler
	})
}

// WithBars configures initial bar symbols to subscribe to and the handler
func WithBars(handler func(Bar), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.bars = symbols
		o.barHandler = handler
	})
}

// WithUpdatedBars configures initial updated bar symbols to subscribe to and the handler
func WithUpdatedBars(handler func(Bar), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.updatedBars = symbols
		o.updatedBarHandler = handler
	})
}

// WithDailyBars configures initial daily bar symbols to subscribe to and the handler
func WithDailyBars(handler func(Bar), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.dailyBars = symbols
		o.dailyBarHandler = handler
	})
}

// WithStatuses configures initial trading status symbols to subscribe to and the handler
func WithStatuses(handler func(TradingStatus), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.statuses = symbols
		o.tradingStatusHandler = handler
	})
}

// WithImbalances configures initial imbalance handler.
func WithImbalances(handler func(Imbalance), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.imbalances = symbols
		o.imbalanceHandler = handler
	})
}

// WithLULDs configures initial LULD symbols to subscribe to and the handler
func WithLULDs(handler func(LULD), symbols ...string) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.sub.lulds = symbols
		o.luldHandler = handler
	})
}

// WithCancelErrors configures initial trade cancel errors handler. This does
// not create any new subscriptions because cancel errors are subscribed
// automatically together with trades. No need to pass in symbols.
func WithCancelErrors(handler func(TradeCancelError)) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.cancelErrorHandler = handler
	})
}

// WithCorrections configures initial trade corrections handler. This does
// not create any new subscriptions because corrections are subscribed
// automatically together with trades. No need to pass in symbols.
func WithCorrections(handler func(TradeCorrection)) StockOption {
	return newFuncStockOption(func(o *stockOptions) {
		o.correctionHandler = handler
	})
}

type cryptoOptions struct {
	options
	tradeHandler      func(CryptoTrade)
	quoteHandler      func(CryptoQuote)
	barHandler        func(CryptoBar)
	updatedBarHandler func(CryptoBar)
	dailyBarHandler   func(CryptoBar)
	orderbookHandler  func(CryptoOrderbook)
	pricingHandler    func(CryptoPerpPricing)
}

// defaultCryptoOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultCryptoOptions() *cryptoOptions {
	baseURL := "https://stream.data.alpaca.markets/v1beta3/crypto"
	// Should this override option be removed?
	if s := os.Getenv("DATA_CRYPTO_PROXY_WS"); s != "" {
		baseURL = s
	}

	return &cryptoOptions{
		options: options{
			logger:         DefaultLogger(),
			baseURL:        baseURL,
			key:            os.Getenv("APCA_API_KEY_ID"),
			secret:         os.Getenv("APCA_API_SECRET_KEY"),
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100000,
			sub: subscriptions{
				trades:      []string{},
				quotes:      []string{},
				bars:        []string{},
				updatedBars: []string{},
				dailyBars:   []string{},
				orderbooks:  []string{},
			},
			connCreator: newCoderWebsocketConn,
		},
		tradeHandler:      func(_ CryptoTrade) {},
		quoteHandler:      func(_ CryptoQuote) {},
		barHandler:        func(_ CryptoBar) {},
		updatedBarHandler: func(_ CryptoBar) {},
		dailyBarHandler:   func(_ CryptoBar) {},
		orderbookHandler:  func(_ CryptoOrderbook) {},
		pricingHandler:    func(_ CryptoPerpPricing) {},
	}
}

func (o *cryptoOptions) applyCrypto(opts ...CryptoOption) {
	for _, opt := range opts {
		opt.applyCrypto(o)
	}
}

type funcCryptoOption struct {
	f func(*cryptoOptions)
}

func (fo *funcCryptoOption) applyCrypto(o *cryptoOptions) {
	fo.f(o)
}

func newFuncCryptoOption(f func(*cryptoOptions)) *funcCryptoOption {
	return &funcCryptoOption{
		f: f,
	}
}

// WithCryptoTrades configures initial trade symbols to subscribe to and the handler
func WithCryptoTrades(handler func(CryptoTrade), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.trades = symbols
		o.tradeHandler = handler
	})
}

// WithCryptoPerpPricing configures initial pricing symbols to subscribe to and the handler
func WithCryptoPerpPricing(handler func(CryptoPerpPricing), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.pricing = symbols
		o.pricingHandler = handler
	})
}

// WithCryptoQuotes configures initial quote symbols to subscribe to and the handler
func WithCryptoQuotes(handler func(CryptoQuote), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.quotes = symbols
		o.quoteHandler = handler
	})
}

// WithCryptoBars configures initial bar symbols to subscribe to and the handler
func WithCryptoBars(handler func(CryptoBar), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.bars = symbols
		o.barHandler = handler
	})
}

// WithCryptoUpdatedBars configures initial updated bar symbols to subscribe to and the handler
func WithCryptoUpdatedBars(handler func(CryptoBar), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.updatedBars = symbols
		o.updatedBarHandler = handler
	})
}

// WithCryptoDailyBars configures initial daily bar symbols to subscribe to and the handler
func WithCryptoDailyBars(handler func(CryptoBar), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.dailyBars = symbols
		o.dailyBarHandler = handler
	})
}

// WithCryptoOrderbooks configures initial orderbook symbols to subscribe to and the handler
func WithCryptoOrderbooks(handler func(CryptoOrderbook), symbols ...string) CryptoOption {
	return newFuncCryptoOption(func(o *cryptoOptions) {
		o.sub.orderbooks = symbols
		o.orderbookHandler = handler
	})
}

type optionOptions struct {
	options
	tradeHandler func(OptionTrade)
	quoteHandler func(OptionQuote)
}

// defaultOptionOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultOptionOptions() *optionOptions {
	baseURL := "https://stream.data.alpaca.markets/v1beta1"
	// Should this override option be removed?
	if s := os.Getenv("DATA_PROXY_WS"); s != "" {
		baseURL = s
	}

	return &optionOptions{
		options: options{
			logger:         DefaultLogger(),
			baseURL:        baseURL,
			key:            os.Getenv("APCA_API_KEY_ID"),
			secret:         os.Getenv("APCA_API_SECRET_KEY"),
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100000,
			sub: subscriptions{
				trades:      []string{},
				quotes:      []string{},
				bars:        []string{},
				updatedBars: []string{},
				dailyBars:   []string{},
			},
			connCreator: newCoderWebsocketConn,
		},
		tradeHandler: func(_ OptionTrade) {},
		quoteHandler: func(_ OptionQuote) {},
	}
}

func (o *optionOptions) applyOption(opts ...OptionOption) {
	for _, opt := range opts {
		opt.applyOption(o)
	}
}

type funcOptionOption struct {
	f func(*optionOptions)
}

func (fo *funcOptionOption) applyOption(o *optionOptions) {
	fo.f(o)
}

func newFuncOptionOption(f func(*optionOptions)) *funcOptionOption {
	return &funcOptionOption{
		f: f,
	}
}

// WithOptionTrades configures initial trade symbols to subscribe to and the handler
func WithOptionTrades(handler func(OptionTrade), symbols ...string) OptionOption {
	return newFuncOptionOption(func(o *optionOptions) {
		o.sub.trades = symbols
		o.tradeHandler = handler
	})
}

// WithOptionQuotes configures initial quote symbols to subscribe to and the handler
func WithOptionQuotes(handler func(OptionQuote), symbols ...string) OptionOption {
	return newFuncOptionOption(func(o *optionOptions) {
		o.sub.quotes = symbols
		o.quoteHandler = handler
	})
}

type newsOptions struct {
	options
	newsHandler func(News)
}

// defaultNewsOptions are the default options for a client.
// Don't change this in a backward incompatible way!
func defaultNewsOptions() *newsOptions {
	return &newsOptions{
		options: options{
			logger:         DefaultLogger(),
			baseURL:        "https://stream.data.alpaca.markets/v1beta1/news",
			key:            os.Getenv("APCA_API_KEY_ID"),
			secret:         os.Getenv("APCA_API_SECRET_KEY"),
			reconnectLimit: 20,
			reconnectDelay: 150 * time.Millisecond,
			processorCount: 1,
			bufferSize:     100,
			sub: subscriptions{
				news: []string{},
			},
			connCreator: newCoderWebsocketConn,
		},
		newsHandler: func(_ News) {},
	}
}

func (o *newsOptions) applyNews(opts ...NewsOption) {
	for _, opt := range opts {
		opt.applyNews(o)
	}
}

type funcNewsOption struct {
	f func(*newsOptions)
}

func (fo *funcNewsOption) applyNews(o *newsOptions) {
	fo.f(o)
}

func newFuncNewsOption(f func(*newsOptions)) *funcNewsOption {
	return &funcNewsOption{
		f: f,
	}
}

// WithNew configures initial symbols to subscribe to and the handler
func WithNews(handler func(News), symbols ...string) NewsOption {
	return newFuncNewsOption(func(o *newsOptions) {
		o.sub.news = symbols
		o.newsHandler = handler
	})
}
