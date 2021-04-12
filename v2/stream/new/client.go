package new

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
// Error() returns a channel that the client sends an error to when it has terminated.
// A client can not be reused once it has terminated!
//
// SubscribeTo... and UbsubscribeFrom... can be used to modify subscriptions and
// the handler used to process incoming trades/quotes/bars. These block until an
// irrecoverable error occurs or if they succeed.
//
// Note that subscription changes can not be called concurrently.
type StreamV2Client interface {
	Connect(ctx context.Context) error
	Error() <-chan error
	SubscribeToTrades(handler func(trade Trade), symbols ...string) error
	UnsubscribeFromTrades(symbols ...string) error
	SubscribeToQuotes(handler func(quote Quote), symbols ...string) error
	UnsubscribeFromQuotes(symbols ...string) error
	SubscribeToBars(handler func(bar Bar), symbols ...string) error
	UnsubscribeFromBars(symbols ...string) error
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
	connectOnce    sync.Once
	connectCalled  bool
	hasTerminated  bool
	connErrChan    chan error
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
	pendingSubChangeMutex sync.Mutex
	pendingSubChange      *subChangeRequest
}

var _ StreamV2Client = (*client)(nil)

func NewClient(feed string, opts ...Option) StreamV2Client {
	c := client{
		feed:        feed,
		connErrChan: make(chan error, 1),
		subChanges:  make(chan []byte, 1),
	}
	o := defaultOptions()
	o.apply(opts...)
	c.configure(*o)
	return &c
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

var ErrConnectCalledMultipleTimes = errors.New("Connect called multiple times")

// Connect establishes a connection for c using its configuration and reestablishes it when errors occur.
// It blocks until the connection has been established for the first time (or it failed to do so).
// Should only be called once!
//
// It returns after the connection was established for the first time or if it couldn't be established
// after retrying.
func (c *client) Connect(ctx context.Context) error {
	err := ErrConnectCalledMultipleTimes
	c.connectOnce.Do(func() {
		err = c.connect(ctx)
		if err != nil {
			c.connErrChan <- err
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

	successCh := make(chan struct{})
	go c.maintainConnection(ctx, u, successCh)

	select {
	case <-successCh:
		return nil
	case err := <-c.connErrChan:
		return err
	}
}

func (c *client) Error() <-chan error {
	return c.connErrChan
}

// maintainConnection initializes a connection to u, starts the necessary goroutines
// and recreates them if there was an error as long as reconnectLimit consecutive
// connection initialization errors don't occur. It closes successCh upon the first
// successful connection intialization
func (c *client) maintainConnection(ctx context.Context, u url.URL, successCh chan<- struct{}) {
	var connError error
	failedAttemptsInARow := 0
	connectedAtLeastOnce := false

	defer func() {
		// If there is a pending sub change we should terminate that
		c.pendingSubChangeMutex.Lock()
		defer c.pendingSubChangeMutex.Unlock()
		if c.pendingSubChange != nil {
			c.pendingSubChange.result <- ErrSubChangeInterrupted
		}
		c.pendingSubChange = nil
		c.hasTerminated = true
	}()

	for {
		select {
		case <-ctx.Done():
			if !connectedAtLeastOnce {
				c.logger.Warnf("datav2stream: cancelled before connection could be established, last error: %v", connError)
				err := fmt.Errorf("cancelled before connection could be established, last error: %w", connError)
				c.connErrChan <- err
			}
			return
		default:
			if c.reconnectLimit != 0 && failedAttemptsInARow >= c.reconnectLimit {
				c.logger.Errorf("datav2stream: max reconnect limit has been reached, last error: %v", connError)
				c.connErrChan <- fmt.Errorf("max reconnect limit has been reached, last error: %w", connError)
				return
			}
			time.Sleep(time.Duration(failedAttemptsInARow) * c.reconnectDelay)
			failedAttemptsInARow++
			c.logger.Infof("datav2stream: connecting to %s, attempt %d/%d ...", u, failedAttemptsInARow, c.reconnectLimit)
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
				c.logger.Warnf("datav2stream: connection setup failed, error: %v", err)
				continue
			}
			c.logger.Infof("datav2stream: finished connection setup")
			connError = nil
			if !connectedAtLeastOnce {
				close(successCh)
			}
			connectedAtLeastOnce = true
			failedAttemptsInARow = 0

			c.in = make(chan []byte)
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
			c.logger.Warnf("datav2stream: connection lost")
		}
	}
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
				if !c.conn.isCloseError(err) && ctx.Err() == nil {
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
			if !c.conn.isCloseError(err) && ctx.Err() == nil {
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
				if !c.conn.isCloseError(err) && ctx.Err() == nil {
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
			c.logger.Errorf("processing msg: %s", msg)
			err := c.handleMessage(msg)
			if err != nil {
				c.logger.Warnf("datav2stream: could not handle message, error: %v", err)
			}
		}
	}
}
