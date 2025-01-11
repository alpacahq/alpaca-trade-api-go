package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

type client struct {
	logger Logger

	baseURL string
	key     string
	secret  string

	reconnectLimit     int
	reconnectDelay     time.Duration
	connectCallback    func()
	disconnectCallback func()
	processorCount     int
	bufferSize         int
	connectOnce        sync.Once
	connectCalled      bool
	hasTerminated      bool
	terminatedChan     chan error
	conn               conn
	in                 chan []byte
	subChanges         chan []byte

	bufferFillCallback func([]byte)
	lastBufferFill     time.Time
	droppedMsgCount    int

	sub subscriptions

	handler msgHandler

	pendingSubChangeMutex sync.Mutex
	pendingSubChange      *subChangeRequest

	connCreator func(ctx context.Context, u url.URL) (conn, error)
}

func newClient() *client {
	return &client{
		terminatedChan: make(chan error, 1),
		subChanges:     make(chan []byte, 1),
	}
}

func (c *client) configure(o options) {
	c.logger = o.logger
	c.baseURL = o.baseURL
	c.key = o.key
	c.secret = o.secret
	c.reconnectLimit = o.reconnectLimit
	c.reconnectDelay = o.reconnectDelay
	c.connectCallback = o.connectCallback
	c.bufferFillCallback = o.bufferFillCallback
	c.disconnectCallback = o.disconnectCallback
	c.processorCount = o.processorCount
	c.bufferSize = o.bufferSize
	c.sub = o.sub
	c.connCreator = o.connCreator
}

// StocksClient is a client that connects to the Alpaca stream server
// and handles communication both ways.
//
// After constructing, Connect() must be called before any subscription changes
// are called. Connect keeps the connection alive and reestablishes it until
// a configured number of retries has not been exceeded.
//
// Terminated() returns a channel that the client sends an error to when it has terminated.
// A client can not be reused once it has terminated!
//
// SubscribeTo... and UnsubscribeFrom... can be used to modify subscriptions and
// the handler used to process incoming trades/quotes/bars/etc. These block until an
// irrecoverable error occurs or if they succeed.
//
// Note that subscription changes can not be called concurrently.
type StocksClient struct {
	*client

	feed    marketdata.Feed
	handler *stocksMsgHandler
}

// NewStocksClient returns a new StocksClient that will connect to feed data feed
// and whose default configurations are modified by opts.
func NewStocksClient(feed marketdata.Feed, opts ...StockOption) *StocksClient {
	sc := StocksClient{
		client:  newClient(),
		feed:    feed,
		handler: &stocksMsgHandler{},
	}
	sc.client.handler = sc.handler
	o := defaultStockOptions()
	o.applyStock(opts...)
	sc.configure(*o)
	return &sc
}

func (sc *StocksClient) configure(o stockOptions) {
	sc.client.configure(o.options)
	sc.handler.tradeHandler = o.tradeHandler
	sc.handler.quoteHandler = o.quoteHandler
	sc.handler.barHandler = o.barHandler
	sc.handler.updatedBarHandler = o.updatedBarHandler
	sc.handler.dailyBarHandler = o.dailyBarHandler
	sc.handler.tradingStatusHandler = o.tradingStatusHandler
	sc.handler.imbalanceHandler = o.imbalanceHandler
	sc.handler.luldHandler = o.luldHandler
	sc.handler.cancelErrorHandler = o.cancelErrorHandler
	sc.handler.correctionHandler = o.correctionHandler
}

// Connect establishes a connection and **reestablishes it when errors occur**
// as long as the configured number of retries has not been exceeded.
//
// It blocks until the connection has been established for the first time (or it failed to do so).
//
// **Should only be called once!**
func (sc *StocksClient) Connect(ctx context.Context) error {
	u, err := sc.constructURL()
	if err != nil {
		return err
	}
	return sc.connect(ctx, u)
}

func (sc *StocksClient) constructURL() (url.URL, error) {
	return constructURL(sc.baseURL, sc.feed)
}

func constructURL(base, feed string) (url.URL, error) {
	scheme := "wss"
	ub, err := url.Parse(base + "/" + feed)
	if err != nil {
		return url.URL{}, err
	}
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}
	return url.URL{Scheme: scheme, Host: ub.Host, Path: ub.Path}, nil
}

// CryptoClient is a client that connects to an Alpaca stream server
// and handles communication both ways.
//
// After constructing, Connect() must be called before any subscription changes
// are called. Connect keeps the connection alive and reestablishes it until
// a configured number of retries has not been exceeded.
//
// Terminated() returns a channel that the client sends an error to when it has terminated.
// A client can not be reused once it has terminated!
//
// SubscribeTo... and UnsubscribeFrom... can be used to modify subscriptions and
// the handler used to process incoming trades/quotes/bars. These block until an
// irrecoverable error occurs or if they succeed.
//
// Note that subscription changes can not be called concurrently.
type CryptoClient struct {
	*client

	feed    marketdata.CryptoFeed
	handler *cryptoMsgHandler
}

// NewCryptoClient returns a new CryptoClient that will connect to the crypto feed
// and whose default configurations are modified by opts.
func NewCryptoClient(feed marketdata.CryptoFeed, opts ...CryptoOption) *CryptoClient {
	cc := CryptoClient{}
	cc.init(feed, opts...)

	return &cc
}

func (cc *CryptoClient) init(feed marketdata.CryptoFeed, opts ...CryptoOption) {
	cc.client = newClient()
	cc.feed = feed
	cc.handler = &cryptoMsgHandler{}

	cc.client.handler = cc.handler
	o := defaultCryptoOptions()
	o.applyCrypto(opts...)
	cc.configure(*o)
}

func (cc *CryptoClient) configure(o cryptoOptions) {
	cc.client.configure(o.options)
	cc.handler.tradeHandler = o.tradeHandler
	cc.handler.quoteHandler = o.quoteHandler
	cc.handler.barHandler = o.barHandler
	cc.handler.updatedBarHandler = o.updatedBarHandler
	cc.handler.dailyBarHandler = o.dailyBarHandler
	cc.handler.orderbookHandler = o.orderbookHandler
	cc.handler.futuresPricingHandler = o.pricingHandler
}

// Connect establishes a connection and **reestablishes it when errors occur**
// as long as the configured number of retries has not been exceeded.
//
// It blocks until the connection has been established for the first time (or it failed to do so).
//
// **Should only be called once!**
func (cc *CryptoClient) Connect(ctx context.Context) error {
	u, err := cc.constructURL()
	if err != nil {
		return err
	}
	return cc.connect(ctx, u)
}

func (cc *CryptoClient) constructURL() (url.URL, error) {
	return constructURL(cc.baseURL, cc.feed)
}

// OptionClient is a client that connects to an Alpaca stream server
// and handles communication both ways.
//
// After constructing, Connect() must be called before any subscription changes
// are called. Connect keeps the connection alive and reestablishes it until
// a configured number of retries has not been exceeded.
//
// Terminated() returns a channel that the client sends an error to when it has terminated.
// A client can not be reused once it has terminated!
//
// SubscribeTo... and UnsubscribeFrom... can be used to modify subscriptions and
// the handler used to process incoming trades/quotes/bars. These block until an
// irrecoverable error occurs or if they succeed.
//
// Note that subscription changes can not be called concurrently.
type OptionClient struct {
	*client

	feed    marketdata.OptionFeed
	handler *optionsMsgHandler
}

// NewOptionClient returns a new OptionClient that will connect to the option feed
// and whose default configurations are modified by opts.
func NewOptionClient(feed marketdata.OptionFeed, opts ...OptionOption) *OptionClient {
	cc := OptionClient{
		client:  newClient(),
		feed:    feed,
		handler: &optionsMsgHandler{},
	}
	cc.client.handler = cc.handler
	o := defaultOptionOptions()
	o.applyOption(opts...)
	cc.configure(*o)
	return &cc
}

func (cc *OptionClient) configure(o optionOptions) {
	cc.client.configure(o.options)
	cc.handler.tradeHandler = o.tradeHandler
	cc.handler.quoteHandler = o.quoteHandler
}

// Connect establishes a connection and **reestablishes it when errors occur**
// as long as the configured number of retries has not been exceeded.
//
// It blocks until the connection has been established for the first time (or it failed to do so).
//
// **Should only be called once!**
func (cc *OptionClient) Connect(ctx context.Context) error {
	u, err := cc.constructURL()
	if err != nil {
		return err
	}
	return cc.connect(ctx, u)
}

func (cc *OptionClient) constructURL() (url.URL, error) {
	return constructURL(cc.baseURL, cc.feed)
}

type NewsClient struct {
	*client

	handler *newsMsgHandler
}

// NewNewsClient returns a new NewsClient that will connect the news stream.
func NewNewsClient(opts ...NewsOption) *NewsClient {
	nc := NewsClient{
		client:  newClient(),
		handler: &newsMsgHandler{},
	}
	nc.client.handler = nc.handler
	o := defaultNewsOptions()
	o.applyNews(opts...)
	nc.configure(*o)
	return &nc
}

func (nc *NewsClient) configure(o newsOptions) {
	nc.client.configure(o.options)
	nc.handler.newsHandler = o.newsHandler
}

// Connect establishes a connection and **reestablishes it when errors occur**
// as long as the configured number of retries has not been exceeded.
//
// It blocks until the connection has been established for the first time (or it failed to do so).
//
// **Should only be called once!**
func (nc *NewsClient) Connect(ctx context.Context) error {
	u, err := nc.constructURL()
	if err != nil {
		return err
	}
	return nc.connect(ctx, u)
}

func (nc *NewsClient) constructURL() (url.URL, error) {
	scheme := "wss"
	ub, err := url.Parse(nc.baseURL)
	if err != nil {
		return url.URL{}, err
	}
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}

	return url.URL{Scheme: scheme, Host: ub.Host, Path: ub.Path}, nil
}

func (c *client) connect(ctx context.Context, u url.URL) error {
	err := ErrConnectCalledMultipleTimes
	c.connectOnce.Do(func() {
		err = c.connectAndMaintainConnection(ctx, u)
		if err != nil {
			c.terminatedChan <- err
			close(c.terminatedChan)
		}
		c.connectCalled = true
	})
	return err
}

func (c *client) connectAndMaintainConnection(ctx context.Context, u url.URL) error {
	initialResultCh := make(chan error)
	go c.maintainConnection(ctx, u, initialResultCh)
	return <-initialResultCh
}

// Terminated returns a channel that the client sends an error to when it has terminated.
// The channel is also closed upon termination.
func (c *client) Terminated() <-chan error {
	return c.terminatedChan
}

// maintainConnection initializes a connection to u, starts the necessary goroutines
// and recreates them if there was an error as long as reconnectLimit consecutive
// connection initialization errors don't occur. It sends the first connection
// initialization's result to initialResultCh.
func (c *client) maintainConnection(ctx context.Context, u url.URL, initialResultCh chan<- error) { //nolint:funlen,gocognit,lll // TODO: Refactor this.
	var connError error
	failedAttemptsInARow := 0
	connectedAtLeastOnce := false

	callbackWaitGroup := sync.WaitGroup{}
	defer func() {
		// If there is a pending sub change we should terminate that
		c.pendingSubChangeMutex.Lock()
		defer c.pendingSubChangeMutex.Unlock()
		if c.pendingSubChange != nil {
			c.pendingSubChange.result <- ErrSubscriptionChangeInterrupted
		}
		c.pendingSubChange = nil
		c.hasTerminated = true
		// if we haven't connected at least once then Connected should close the channel
		if connectedAtLeastOnce {
			close(c.terminatedChan)
		}
		// If a disconnect/connect callback is running then wait until it finishes or times out.
		timeout := time.Second
		if waitTimeout(&callbackWaitGroup, timeout) {
			c.logger.Warnf("datav2stream: timed out after waiting %s for connect/disconnect callbacks to return", timeout)
		}
	}()

	sendError := func(err error) {
		if !connectedAtLeastOnce {
			initialResultCh <- err
		} else {
			c.terminatedChan <- err
		}
	}

	c.in = make(chan []byte, c.bufferSize)
	pwg := sync.WaitGroup{}
	pwg.Add(c.processorCount)
	for i := 0; i < c.processorCount; i++ {
		go c.messageProcessor(ctx, &pwg)
	}
	defer func() {
		close(c.in)
		pwg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			if !connectedAtLeastOnce {
				c.logger.Warnf("datav2stream: cancelled before connection could be established, last error: %v", connError)
				err := fmt.Errorf("cancelled before connection could be established, last error: %w", connError)
				initialResultCh <- err
			} else {
				c.terminatedChan <- nil
			}
			return
		default:
			if c.reconnectLimit != 0 && failedAttemptsInARow >= c.reconnectLimit {
				c.logger.Errorf("datav2stream: max reconnect limit has been reached, last error: %v", connError)
				e := fmt.Errorf("max reconnect limit has been reached, last error: %w", connError)
				sendError(e)
				return
			}
			time.Sleep(time.Duration(failedAttemptsInARow) * c.reconnectDelay)
			failedAttemptsInARow++
			c.logger.Infof("datav2stream: connecting to %s, attempt %d/%d ...",
				u.String(), failedAttemptsInARow, c.reconnectLimit)
			conn, err := c.connCreator(ctx, u)
			if err != nil {
				connError = err
				if isHTTP4xx(err) {
					c.logger.Errorf("datav2stream: %v", wrapIrrecoverable(err))
					sendError(wrapIrrecoverable(err))
					return
				}
				c.logger.Warnf("datav2stream: failed to connect, error: %v", err)
				continue
			}
			c.conn = conn

			c.logger.Infof("datav2stream: established connection")
			if err := c.initialize(ctx); err != nil {
				connError = err
				c.conn.close()
				if isErrorIrrecoverableAtInit(err) {
					c.logger.Errorf("datav2stream: %v", wrapIrrecoverable(err))
					sendError(wrapIrrecoverable(err))
					return
				}
				c.logger.Warnf("datav2stream: connection setup failed, error: %v", err)
				continue
			}
			c.logger.Infof("datav2stream: finished connection setup")

			if c.connectCallback != nil {
				callbackWaitGroup.Add(1)
				go func() {
					defer callbackWaitGroup.Done()
					c.connectCallback()
				}()
			}

			connError = nil
			if !connectedAtLeastOnce {
				initialResultCh <- nil
				connectedAtLeastOnce = true
			}
			failedAttemptsInARow = 0

			wg := sync.WaitGroup{}
			wg.Add(3)
			closeCh := make(chan struct{})
			go c.connPinger(ctx, &wg, closeCh)
			go c.connReader(ctx, &wg, closeCh)
			go c.connWriter(ctx, &wg, closeCh)
			wg.Wait()

			if ctx.Err() != nil {
				c.logger.Infof("datav2stream: disconnected")
			} else {
				c.logger.Warnf("datav2stream: connection lost")
			}

			if c.disconnectCallback != nil {
				callbackWaitGroup.Add(1)
				go func() {
					defer callbackWaitGroup.Done()
					c.disconnectCallback()
				}()
			}
		}
	}
}

// waitTimeout waits for the WaitGroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// irrecoverableErrorsAtInit contains errors that are considered irrecoverable when initializing
// the connection.
//
// ErrSubscriptionChangeInvalidForFeed means that the subscribe command failed and normally
// that "only" results in less messages from the server. However at startup this fails the whole
// flow, and can not be helped by a retry. So irrecoverableErrorsAtInit should only be used at init,
// once the connection was successfully made, ErrSubscriptionChangeInvalidForFeed should not be
// considered fatal.
var irrecoverableErrorsAtInit = []error{
	ErrInvalidCredentials,
	ErrInsufficientSubscription,
	ErrInsufficientScope,
	ErrSubscriptionChangeInvalidForFeed,
}

// isErrorIrrecoverableAtInit returns whether the error is irrecoverable and further retries should
// not take place at initialisation.
func isErrorIrrecoverableAtInit(err error) bool {
	for _, irrErr := range irrecoverableErrorsAtInit {
		if errors.Is(err, irrErr) {
			return true
		}
	}
	return false
}

func isHTTP4xx(err error) bool {
	// Unfortunately the coder/websocket error is a simple formatted string, created by fmt.Errorf,
	// so the only check we can do is string matching
	pattern := `expected handshake response status code 101 but got 4\d\d`
	ok, _ := regexp.MatchString(pattern, err.Error())
	return ok
}

func wrapIrrecoverable(err error) error {
	return fmt.Errorf("irrecoverable error: %w", err)
}

var newPingTicker = func() ticker {
	return &timeTicker{ticker: time.NewTicker(pingPeriod)}
}

// connPinger periodically calls c.conn.Ping to ensure the connection is still alive
func (c *client) connPinger(ctx context.Context, wg *sync.WaitGroup, closeCh <-chan struct{}) {
	pingTicker := newPingTicker()
	defer func() {
		pingTicker.Stop()
		c.conn.close()
		wg.Done()
	}()

	for {
		select {
		case <-closeCh:
			return
		case <-ctx.Done():
			return
		case <-pingTicker.C():
			if err := c.conn.ping(ctx); err != nil {
				if ctx.Err() == nil {
					c.logger.Warnf("datav2stream: ping failed, error: %v", err)
				}
				return
			}
		}
	}
}

// connReader reads from c.conn and sends those messages to c.in.
// It is also responsible for closing closeCh that terminates the other worker
// goroutines and also for closing c.in which terminates messageProcessors.
func (c *client) connReader(
	ctx context.Context,
	wg *sync.WaitGroup,
	closeCh chan<- struct{},
) {
	defer func() {
		close(closeCh)
		c.conn.close()
		wg.Done()
	}()

	for {
		msg, err := c.conn.readMessage(ctx)
		if err != nil {
			if ctx.Err() == nil {
				c.logger.Warnf("datav2stream: reading from conn failed, error: %v", err)
			}
			return
		}

		select {
		case c.in <- msg:
		default:
			c.droppedMsgCount++
			now := time.Now()
			// Reduce the number of logs to 1 msg/sec if client buffer is full
			if now.Add(-1 * time.Second).After(c.lastBufferFill) {
				c.logger.Warnf("datav2stream: writing to buffer failed, error: buffer full, dropped: %d", c.droppedMsgCount)
				c.droppedMsgCount = 0
				c.lastBufferFill = now
			}
			if c.bufferFillCallback != nil {
				c.bufferFillCallback(msg)
			}
		}
	}
}

// connWriter handles writing messages from c.subChanges to c.conn
func (c *client) connWriter(ctx context.Context, wg *sync.WaitGroup, closeCh <-chan struct{}) {
	defer func() {
		c.conn.close()
		wg.Done()
	}()

	// We need to make sure that the pending sub change is handled
	// Goal: make sure the message is in c.subChanges
	c.pendingSubChangeMutex.Lock()
	if c.pendingSubChange != nil {
		select {
		case <-c.subChanges:
		default:
		}
		c.subChanges <- c.pendingSubChange.msg
	}
	c.pendingSubChangeMutex.Unlock()

	for {
		select {
		case <-closeCh:
			return
		case <-ctx.Done():
			return
		case msg := <-c.subChanges:
			if err := c.conn.writeMessage(ctx, msg); err != nil {
				if ctx.Err() == nil {
					c.logger.Warnf("datav2stream: writing to conn failed, error: %v", err)
				}
				return
			}
		}
	}
}

// messageProcessor reads from c.in (while it's open) and processes the messages
func (c *client) messageProcessor(
	ctx context.Context,
	wg *sync.WaitGroup,
) {
	defer func() {
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.in:
			if !ok {
				return
			}
			err := c.handleMessage(msg)
			if err != nil {
				c.logger.Errorf("datav2stream: could not handle message, error: %v", err)
			}
		}
	}
}
