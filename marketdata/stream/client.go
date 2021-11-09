package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type StreamClient interface {
	// Connect establishes a connection and **reestablishes it when errors occur**
	// as long as the configured number of retires has not been exceeded.
	//
	// It blocks until the connection has been established for the first time (or it failed to do so).
	//
	// **Should only be called once!**
	Connect(ctx context.Context) error
	// Terminated returns a channel that the client sends an error to when it has terminated.
	// The channel is also closed upon termination.
	Terminated() <-chan error
}

// StocksClient is a client that connects to an Alpaca Data V2 stream server
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
type StocksClient interface {
	StreamClient
	SubscribeToTrades(handler func(trade Trade), symbols ...string) error
	UnsubscribeFromTrades(symbols ...string) error
	SubscribeToQuotes(handler func(quote Quote), symbols ...string) error
	UnsubscribeFromQuotes(symbols ...string) error
	SubscribeToBars(handler func(bar Bar), symbols ...string) error
	UnsubscribeFromBars(symbols ...string) error
	SubscribeToDailyBars(handler func(bar Bar), symbols ...string) error
	UnsubscribeFromDailyBars(symbols ...string) error
	SubscribeToStatuses(handler func(ts TradingStatus), symbols ...string) error
	UnsubscribeFromStatuses(symbols ...string) error
	SubscribeToLULDs(handler func(luld LULD), symbols ...string) error
	UnsubscribeFromLULDs(symbols ...string) error
}

// CryptoClient is a client that connects to an Alpaca Data V2 stream server
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
type CryptoClient interface {
	StreamClient
	SubscribeToTrades(handler func(trade CryptoTrade), symbols ...string) error
	UnsubscribeFromTrades(symbols ...string) error
	SubscribeToQuotes(handler func(quote CryptoQuote), symbols ...string) error
	UnsubscribeFromQuotes(symbols ...string) error
	SubscribeToBars(handler func(bar CryptoBar), symbols ...string) error
	UnsubscribeFromBars(symbols ...string) error
	SubscribeToDailyBars(handler func(bar CryptoBar), symbols ...string) error
	UnsubscribeFromDailyBars(symbols ...string) error
}

type client struct {
	logger Logger

	baseURL string
	key     string
	secret  string

	reconnectLimit int
	reconnectDelay time.Duration
	processorCount int
	bufferSize     int
	connectOnce    sync.Once
	connectCalled  bool
	hasTerminated  bool
	terminatedChan chan error
	conn           conn
	in             chan []byte
	subChanges     chan []byte

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
	c.processorCount = o.processorCount
	c.bufferSize = o.bufferSize
	c.sub = o.sub
	c.connCreator = o.connCreator
}

type stocksClient struct {
	*client

	feed    string
	handler *stocksMsgHandler
}

var _ StocksClient = (*stocksClient)(nil)

// NewStocksClient returns a new StocksClient that will connect to feed data feed
// and whose default configurations are modified by opts.
func NewStocksClient(feed string, opts ...StockOption) StocksClient {
	sc := stocksClient{
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

func (sc *stocksClient) configure(o stockOptions) {
	sc.client.configure(o.options)
	sc.handler.tradeHandler = o.tradeHandler
	sc.handler.quoteHandler = o.quoteHandler
	sc.handler.barHandler = o.barHandler
	sc.handler.dailyBarHandler = o.dailyBarHandler
	sc.handler.tradingStatusHandler = o.tradingStatusHandler
	sc.handler.luldHandler = o.luldHandler
}

func (sc *stocksClient) Connect(ctx context.Context) error {
	u, err := sc.constructURL()
	if err != nil {
		return err
	}
	return sc.connect(ctx, u)
}

func (sc *stocksClient) constructURL() (url.URL, error) {
	scheme := "wss"
	ub, err := url.Parse(sc.baseURL)
	if err != nil {
		return url.URL{}, err
	}
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}

	return url.URL{Scheme: scheme, Host: ub.Host, Path: ub.Path + "/" + sc.feed}, nil
}

type cryptoClient struct {
	*client

	exchanges []string
	handler   *cryptoMsgHandler
}

var _ CryptoClient = (*cryptoClient)(nil)

// NewCryptoClient returns a new CryptoClient that will connect to the crypto feed
// and whose default configurations are modified by opts.
func NewCryptoClient(opts ...CryptoOption) CryptoClient {
	cc := cryptoClient{
		client:  newClient(),
		handler: &cryptoMsgHandler{},
	}
	cc.client.handler = cc.handler
	o := defaultCryptoOptions()
	o.applyCrypto(opts...)
	cc.configure(*o)
	return &cc
}

func (cc *cryptoClient) configure(o cryptoOptions) {
	cc.client.configure(o.options)
	cc.handler.tradeHandler = o.tradeHandler
	cc.handler.quoteHandler = o.quoteHandler
	cc.handler.barHandler = o.barHandler
	cc.handler.dailyBarHandler = o.dailyBarHandler
	cc.exchanges = o.exchanges
}

func (cc *cryptoClient) Connect(ctx context.Context) error {
	u, err := cc.constructURL()
	if err != nil {
		return err
	}
	return cc.connect(ctx, u)
}

func (cc *cryptoClient) constructURL() (url.URL, error) {
	scheme := "wss"
	ub, err := url.Parse(cc.baseURL)
	if err != nil {
		return url.URL{}, err
	}
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}

	var rawQuery string
	if len(cc.exchanges) > 0 {
		rawQuery = "exchanges=" + strings.Join(cc.exchanges, ",")
	}

	return url.URL{Scheme: scheme, Host: ub.Host, Path: ub.Path, RawQuery: rawQuery}, nil
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

func (c *client) Terminated() <-chan error {
	return c.terminatedChan
}

// maintainConnection initializes a connection to u, starts the necessary goroutines
// and recreates them if there was an error as long as reconnectLimit consecutive
// connection initialization errors don't occur. It sends the first connection
// initialization's result to initialResultCh.
func (c *client) maintainConnection(ctx context.Context, u url.URL, initialResultCh chan<- error) {
	var connError error
	failedAttemptsInARow := 0
	connectedAtLeastOnce := false

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
	}()

	sendError := func(err error) {
		if !connectedAtLeastOnce {
			initialResultCh <- err
		} else {
			c.terminatedChan <- err
		}
	}

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
			c.logger.Infof("datav2stream: connecting to %s, attempt %d/%d ...", u.String(), failedAttemptsInARow, c.reconnectLimit)
			conn, err := c.connCreator(ctx, u)
			if err != nil {
				connError = err
				if isHttp4xx(err) {
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
				if isErrorIrrecoverable(err) {
					c.logger.Errorf("datav2stream: %v", wrapIrrecoverable(err))
					sendError(wrapIrrecoverable(err))
					return
				}
				c.logger.Warnf("datav2stream: connection setup failed, error: %v", err)
				continue
			}
			c.logger.Infof("datav2stream: finished connection setup")
			connError = nil
			if !connectedAtLeastOnce {
				initialResultCh <- nil
				connectedAtLeastOnce = true
			}
			failedAttemptsInARow = 0

			c.in = make(chan []byte, c.bufferSize)
			wg := sync.WaitGroup{}
			wg.Add(c.processorCount + 3)
			closeCh := make(chan struct{})
			for i := 0; i < c.processorCount; i++ {
				go c.messageProcessor(ctx, &wg)
			}
			go c.connPinger(ctx, &wg, closeCh)
			go c.connReader(ctx, &wg, closeCh)
			go c.connWriter(ctx, &wg, closeCh)
			wg.Wait()
			if ctx.Err() != nil {
				c.logger.Infof("datav2stream: disconnected")
			} else {
				c.logger.Warnf("datav2stream: connection lost")
			}
		}
	}
}

// isErrorIrrecoverable returns whether the error is irrecoverable and further retries should
// not take place
func isErrorIrrecoverable(err error) bool {
	return errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrInsufficientSubscription) ||
		errors.Is(err, ErrInsufficientScope)
}

func isHttp4xx(err error) bool {
	// Unfortunately the nhoory error is a simple formatted string, created by fmt.Errorf,
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
		close(c.in)
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

		c.in <- msg
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
