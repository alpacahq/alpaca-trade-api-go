package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// StreamV2Client is a client that connects to an Alpaca Data V2 stream server
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
type StreamV2Client interface {
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
	SubscribeToTrades(handler func(trade Trade), symbols ...string) error
	UnsubscribeFromTrades(symbols ...string) error
	SubscribeToQuotes(handler func(quote Quote), symbols ...string) error
	UnsubscribeFromQuotes(symbols ...string) error
	SubscribeToBars(handler func(bar Bar), symbols ...string) error
	UnsubscribeFromBars(symbols ...string) error
	SubscribeToDailyBars(handler func(bar Bar), symbols ...string) error
	UnsubscribeFromDailyBars(symbols ...string) error
}

type client struct {
	logger Logger

	feed   string
	host   string
	key    string
	secret string

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

	handlerMutex          sync.RWMutex
	trades                []string
	tradeHandler          func(trade Trade)
	quotes                []string
	quoteHandler          func(quote Quote)
	bars                  []string
	barHandler            func(bar Bar)
	dailyBars             []string
	dailyBarHandler       func(bar Bar)
	pendingSubChangeMutex sync.Mutex
	pendingSubChange      *subChangeRequest
}

var _ StreamV2Client = (*client)(nil)

// NewClient returns a new StreamV2Client that will connect to feed data feed
// and whose default configurations are modified by opts.
func NewClient(feed string, opts ...Option) StreamV2Client {
	c := client{
		feed:           feed,
		terminatedChan: make(chan error, 1),
		subChanges:     make(chan []byte, 1),
	}
	o := defaultOptions()
	o.apply(opts...)
	c.configure(*o)
	return &c
}

func (c *client) configure(o options) {
	c.logger = o.logger
	c.host = o.host
	c.key = o.key
	c.secret = o.secret
	c.reconnectLimit = o.reconnectLimit
	c.reconnectDelay = o.reconnectDelay
	c.processorCount = o.processorCount
	c.bufferSize = o.bufferSize
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()
	c.trades = o.trades
	c.tradeHandler = o.tradeHandler
	c.quotes = o.quotes
	c.quoteHandler = o.quoteHandler
	c.bars = o.bars
	c.barHandler = o.barHandler
	c.dailyBars = o.dailyBars
	c.dailyBarHandler = o.dailyBarHandler
}

var connCreator = func(ctx context.Context, u url.URL) (conn, error) {
	return newNhooyrWebsocketConn(ctx, u)
}

func constructURL(host, feed string) (url.URL, error) {
	scheme := "wss"
	ub, err := url.Parse(host)
	if err != nil {
		return url.URL{}, err
	}
	switch ub.Scheme {
	case "http", "ws":
		scheme = "ws"
	}

	return url.URL{Scheme: scheme, Host: ub.Host, Path: "/v2/" + feed}, nil
}

// ErrConnectCalledMultipleTimes is returned when Connect has been called multiple times on a single client
var ErrConnectCalledMultipleTimes = errors.New("tried to call Connect multiple times")

func (c *client) Connect(ctx context.Context) error {
	err := ErrConnectCalledMultipleTimes
	c.connectOnce.Do(func() {
		err = c.connect(ctx)
		if err != nil {
			c.terminatedChan <- err
			close(c.terminatedChan)
		}
		c.connectCalled = true
	})
	return err
}

func (c *client) connect(ctx context.Context) error {
	u, err := constructURL(c.host, c.feed)
	if err != nil {
		return err
	}

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
			conn, err := connCreator(ctx, u)
			if err != nil {
				connError = err
				c.logger.Warnf("datav2stream: failed to connect, error: %v", err)
				continue
			}
			c.conn = conn

			c.logger.Infof("datav2stream: established connection")
			if err := c.initialize(ctx); err != nil {
				connError = err
				c.conn.close()
				if isErrorIrrecoverable(err) {
					c.logger.Errorf("datav2stream: irrecoverable error during connection initialization: %v", err)
					e := fmt.Errorf("irrecoverable error during connection initialization: %w", err)
					sendError(e)
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
	return errors.Is(err, ErrInvalidCredentials)
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
					c.logger.Errorf("datav2stream: ping failed, error: %v", err)
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
				c.logger.Errorf("datav2stream: reading from conn failed, error: %v", err)
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
					c.logger.Errorf("datav2stream: writing to conn failed, error: %v", err)
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
