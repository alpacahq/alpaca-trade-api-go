package stream

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

const (
	stocksTests     = "stocks"
	cryptoTests     = "crypto"
	cryptoPerpTests = "crypto-perp"
	perpBaseURL     = "wss://stream.data.alpaca.markets/v1beta1/crypto-perps"
)

var tests = []struct {
	name string
}{
	{name: stocksTests},
	{name: cryptoTests},
	{name: cryptoPerpTests},
}

type streamClient interface {
	Connect(ctx context.Context) error
	Terminated() <-chan error
}

func TestConnectFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(_ context.Context, _ url.URL) (conn, error) {
				return connection, nil
			}

			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(marketdata.IEX,
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
					marketdata.US,
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			case cryptoPerpTests:
				c = NewCryptoClient(
					marketdata.GLOBAL,
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			}

			// server connection can not be established
			connection.readCh <- serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "good",
				},
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			require.Error(t, err)
			require.ErrorIs(t, err, ErrNoConnected)
		})
	}
}

func TestConnectWithInvalidURL(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(
					marketdata.IEX,
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			case cryptoTests:
				c = NewCryptoClient(
					marketdata.US,
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			case cryptoPerpTests:
				c = NewCryptoClient(
					marketdata.GLOBAL,
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			require.Error(t, err)
		})
	}
}

func TestConnectImmediatelyFailsAfterIrrecoverableErrors(t *testing.T) {
	irrecoverableErrors := []struct {
		code int
		msg  string
		err  error
	}{
		{code: 402, msg: "auth failed", err: ErrInvalidCredentials},
		{code: 409, msg: "insufficient subscription", err: ErrInsufficientSubscription},
		{code: 410, msg: "invalid subscribe action for this feed", err: ErrSubscriptionChangeInvalidForFeed},
		{code: 411, msg: "insufficient scope", err: ErrInsufficientScope},
	}
	for _, tt := range tests {
		for _, ie := range irrecoverableErrors {
			t.Run(tt.name+"/"+ie.msg, func(t *testing.T) {
				connection := newMockConn()
				defer connection.close()
				connCreator := func(_ context.Context, _ url.URL) (conn, error) {
					return connection, nil
				}

				// if the error weren't irrecoverable then we would be retrying for quite a while
				// and the test would time out
				reconnectSettings := WithReconnectSettings(20, time.Second)

				var c streamClient
				switch tt.name {
				case stocksTests:
					c = NewStocksClient(marketdata.IEX, reconnectSettings, withConnCreator(connCreator))
				case cryptoTests:
					c = NewCryptoClient(marketdata.US, reconnectSettings, withConnCreator(connCreator))
				case cryptoPerpTests:
					c = NewCryptoClient(marketdata.GLOBAL, reconnectSettings, withConnCreator(connCreator))
				}

				// server welcomes the client
				connection.readCh <- serializeToMsgpack(t, []controlWithT{
					{
						Type: "success",
						Msg:  "connected",
					},
				})
				// server rejects the credentials
				connection.readCh <- serializeToMsgpack(t, []errorWithT{
					{
						Type: msgTypeError,
						Code: ie.code,
						Msg:  ie.msg,
					},
				})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				err := c.Connect(ctx)

				require.Error(t, err)
				require.ErrorIs(t, err, ie.err)
			})
		}
	}
}

func TestContextCancelledBeforeConnect(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(_ context.Context, _ url.URL) (conn, error) {
				return connection, nil
			}

			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(marketdata.IEX,
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
					marketdata.US,
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			case cryptoPerpTests:
				c = NewCryptoClient(
					marketdata.GLOBAL,
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := c.Connect(ctx)
			require.Error(t, err)
			assert.Error(t, <-c.Terminated())
		})
	}
}

func TestConnectSucceeds(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(_ context.Context, _ url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(marketdata.IEX, withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(marketdata.US, withConnCreator(connCreator))
			case cryptoPerpTests:
				c = NewCryptoClient(marketdata.GLOBAL, withConnCreator(connCreator))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			// Connect can't be called multiple times
			err = c.Connect(ctx)
			assert.Equal(t, ErrConnectCalledMultipleTimes, err)
		})
	}
}

func TestCallbacksCalledOnConnectAndDisconnect(t *testing.T) {
	for _, tt := range tests {
		numConnectCalls := 0
		connects := make(chan struct{})
		connectCallback := func() {
			numConnectCalls++
			connects <- struct{}{}
		}

		numDisconnectCalls := 0
		disconnects := make(chan struct{})
		disconnectCallback := func() {
			numDisconnectCalls++
			disconnects <- struct{}{}
		}

		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(_ context.Context, _ url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(marketdata.IEX,
					withConnCreator(connCreator),
					WithConnectCallback(connectCallback),
					WithDisconnectCallback(disconnectCallback))
			case cryptoTests:
				c = NewCryptoClient(marketdata.US,
					withConnCreator(connCreator),
					WithConnectCallback(connectCallback),
					WithDisconnectCallback(disconnectCallback))
			case cryptoPerpTests:
				c = NewCryptoClient(marketdata.GLOBAL,
					withConnCreator(connCreator),
					WithConnectCallback(connectCallback),
					WithDisconnectCallback(disconnectCallback))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			select {
			case <-connects:
			case <-time.After(time.Second):
				require.Fail(t, "connect callback was not called in time")
			}
			assert.Equal(t, 1, numConnectCalls)
			assert.Equal(t, 0, numDisconnectCalls)

			// Now force the stream to disconnect via context and assert disconnect callback is
			// called after waiting a small amount of time to wait for the stream to shut down.
			cancel()
			select {
			case <-disconnects:
			case <-time.After(time.Second):
				require.Fail(t, "disconnect callback was not called in time")
			}
			assert.Equal(t, 1, numConnectCalls)
			assert.Equal(t, 1, numDisconnectCalls)
		})
	}
}

func TestSubscribeBeforeConnectStocks(t *testing.T) {
	c := NewStocksClient(marketdata.IEX)

	err := c.SubscribeToTrades(func(_ Trade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(_ Quote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(_ Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToUpdatedBars(func(_ Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(_ Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToStatuses(func(_ TradingStatus) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToImbalances(func(_ Imbalance) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToLULDs(func(_ LULD) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromStatuses()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromImbalances()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromLULDs()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeBeforeConnectCrypto(t *testing.T) {
	c := NewCryptoClient(marketdata.US)

	err := c.SubscribeToTrades(func(_ CryptoTrade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(_ CryptoQuote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToUpdatedBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToOrderbooks(func(_ CryptoOrderbook) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToPerpPricing(func(_ CryptoPerpPricing) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromUpdatedBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromOrderbooks()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromPerpPricing()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeMultipleCallsStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewStocksClient(marketdata.IEX, withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
		return connection, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(_ Trade) {}, "ALPACA")
	}

	// calling two Subscribes at the same time and also calling a sub change
	// without modifying symbols (should succeed immediately)
	go subFunc()
	err = c.SubscribeToTrades(func(_ Trade) {})
	require.NoError(t, err)
	go subFunc()

	err = <-subErrCh
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeAlreadyInProgress)
}

func TestSubscribeCalledButClientTerminatesCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(marketdata.US,
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))

	ctx, cancel := context.WithCancel(context.Background())

	err := c.Connect(ctx)
	require.NoError(t, err)

	checkInitialMessagesSentByClient(t, connection, "my_key", "my_secret", c.sub)
	subErrCh := make(chan error, 1)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(_ CryptoTrade) {}, "PACOIN")
	}

	// calling Subscribe
	go subFunc()
	// making sure Subscribe got called
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])
	// terminating the client
	cancel()

	err = <-subErrCh
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeInterrupted)

	// Subscribing after the client has terminated results in an error
	err = c.SubscribeToQuotes(func(_ CryptoQuote) {}, "BTC/USD", "ETC/USD")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeAfterTerminated)
}

func TestSubscriptionTimeout(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	mockTimeAfterCh := make(chan time.Time)
	timeAfter = func(_ time.Duration) <-chan time.Time {
		return mockTimeAfterCh
	}
	defer func() {
		timeAfter = time.After
	}()

	c := NewStocksClient(marketdata.IEX,
		WithCredentials("a", "b"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "a", "b", subscriptions{})

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(_ Trade) {}, "ALPACA")
	}

	go subFunc()
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])

	mockTimeAfterCh <- time.Now()
	err = <-subErrCh
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeTimeout, "actual: %s", err)

	// after a timeout we should be able to send a new request
	go subFunc()
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])

	connection.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: []string{"ALPACA"},
		},
	})
	require.NoError(t, <-subErrCh)
}

func TestSubscriptionChangeInvalid(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewStocksClient(marketdata.IEX,
		WithCredentials("a", "b"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "a", "b", subscriptions{})

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(_ Trade) {}, "ALPACA")
	}

	go subFunc()
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 410,
			Msg:  "invalid subscribe action for this feed",
		},
	})
	err = <-subErrCh
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeInvalidForFeed, "actual: %s", err)
}

func TestSubscriptionAcrossConnectionIssues(t *testing.T) {
	conn1 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn1, subscriptions{})

	key := "testkey"
	secret := "testsecret"
	c := NewStocksClient(marketdata.IEX,
		WithCredentials(key, secret),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return conn1, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, conn1, key, secret, subscriptions{})

	// subscribing to something
	trades1 := []string{"AL", "PACA"}
	subRes := make(chan error)
	go func() {
		subRes <- c.SubscribeToTrades(func(_ Trade) {}, "AL", "PACA")
	}()
	sub := expectWrite(t, conn1)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// shutting down the first connection
	conn2 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn2, subscriptions{})
	c.connCreator = func(_ context.Context, _ url.URL) (conn, error) {
		return conn2, nil
	}
	conn1.close()

	// checking whether the client sent what we wanted it to (auth,sub1,sub2)
	checkInitialMessagesSentByClient(t, conn2, key, secret, subscriptions{})
	sub = expectWrite(t, conn2)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// responding to the subscription request
	conn2.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: trades1,
			Quotes: []string{},
			Bars:   []string{},
		},
	})
	require.NoError(t, <-subRes)
	require.ElementsMatch(t, trades1, c.sub.trades)

	// the connection is shut down and the new one isn't established for a while
	conn3 := newMockConn()
	defer conn3.close()
	c.connCreator = func(_ context.Context, _ url.URL) (conn, error) {
		time.Sleep(100 * time.Millisecond)
		writeInitialFlowMessagesToConn(t, conn3, subscriptions{trades: trades1})
		return conn3, nil
	}
	conn2.close()

	// call an unsubscribe with the connection being down
	unsubRes := make(chan error)
	go func() { unsubRes <- c.UnsubscribeFromTrades("AL") }()

	// connection starts up, proper messages (auth,sub,unsub)
	checkInitialMessagesSentByClient(t, conn3, key, secret, subscriptions{trades: trades1})
	unsub := expectWrite(t, conn3)
	require.Equal(t, "unsubscribe", unsub["action"])
	require.ElementsMatch(t, []string{"AL"}, unsub["trades"])

	// responding to the unsub request
	conn3.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: []string{"PACA"},
			Quotes: []string{},
			Bars:   []string{},
		},
	})

	// make sure the sub has returned by now (client changed)
	require.NoError(t, <-unsubRes)
	require.ElementsMatch(t, []string{"PACA"}, c.sub.trades)
}

func TestSubscriptionTwiceAcrossConnectionIssues(t *testing.T) {
	mockTimeAfterCh := make(chan time.Time)
	timeAfter = func(_ time.Duration) <-chan time.Time {
		return mockTimeAfterCh
	}
	defer func() {
		timeAfter = time.After
	}()

	conn1 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn1, subscriptions{})

	connected := make(chan struct{})
	connectCallback := func() {
		t.Log("connected")
		connected <- struct{}{}
	}

	disconnected := make(chan struct{})
	disconnectCallback := func() {
		t.Log("disconnected")
		disconnected <- struct{}{}
	}

	key := "testkey"
	secret := "testsecret"
	c := NewStocksClient(marketdata.IEX,
		WithCredentials(key, secret),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return conn1, nil
		}),
		WithReconnectSettings(0, 150*time.Millisecond),
		WithConnectCallback(connectCallback),
		WithDisconnectCallback(disconnectCallback),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	// wait connect callback
	<-connected
	checkInitialMessagesSentByClient(t, conn1, key, secret, subscriptions{})

	// subscribing to something
	trades1 := []string{"AL", "PACA"}
	subRes := make(chan error)
	subFunc := func() {
		subRes <- c.SubscribeToTrades(func(_ Trade) {}, "AL", "PACA")
	}
	go subFunc()
	sub := expectWrite(t, conn1)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])
	// server accepts subscription
	conn1.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: trades1,
		},
	})
	err = <-subRes
	require.NoError(t, err)

	// shutting down the first connection
	c.connCreator = func(_ context.Context, _ url.URL) (conn, error) {
		return nil, errors.New("connection failed")
	}
	conn1.close()
	// wait disconnect callback
	<-disconnected

	// request subscribe will be timed out during disconnection
	go subFunc()

	mockTimeAfterCh <- time.Now()
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeTimeout, "actual: %s", err)

	// after a timeout we should be able to get timed out again
	go subFunc()

	mockTimeAfterCh <- time.Now()
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeTimeout, "actual: %s", err)

	// establish 2nd connection
	conn2 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn2, subscriptions{trades: trades1})
	c.connCreator = func(_ context.Context, _ url.URL) (conn, error) {
		return conn2, nil
	}
	// wait connect callback
	<-connected

	// checking whether the client sent what we wanted it to (auth,sub1,sub2)
	checkInitialMessagesSentByClient(t, conn2, key, secret, subscriptions{trades: trades1})

	go subFunc()
	sub = expectWrite(t, conn2)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// responding to the subscription request
	conn2.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: trades1,
			Quotes: []string{},
			Bars:   []string{},
		},
	})
	require.NoError(t, <-subRes)
	require.ElementsMatch(t, trades1, c.sub.trades)

	// the connection is shut down and the new one isn't established for a while
	conn3 := newMockConn()
	defer conn3.close()
	c.connCreator = func(_ context.Context, _ url.URL) (conn, error) {
		time.Sleep(100 * time.Millisecond)
		writeInitialFlowMessagesToConn(t, conn3, subscriptions{trades: trades1})
		return conn3, nil
	}
	conn2.close()

	// call an unsubscribe with the connection being down
	unsubRes := make(chan error)
	go func() { unsubRes <- c.UnsubscribeFromTrades("AL") }()

	// connection starts up, proper messages (auth,sub,unsub)
	checkInitialMessagesSentByClient(t, conn3, key, secret, subscriptions{trades: trades1})
	unsub := expectWrite(t, conn3)
	require.Equal(t, "unsubscribe", unsub["action"])
	require.ElementsMatch(t, []string{"AL"}, unsub["trades"])

	// responding to the unsub request
	conn3.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: []string{"PACA"},
			Quotes: []string{},
			Bars:   []string{},
		},
	})

	// make sure the sub has returned by now (client changed)
	require.NoError(t, <-unsubRes)
	require.ElementsMatch(t, []string{"PACA"}, c.sub.trades)
}

func TestSubscribeFailsDueToError(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(marketdata.US,
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "my_key", "my_secret", subscriptions{})

	// attempting sub change
	subRes := make(chan error)
	subFunc := func() {
		subRes <- c.SubscribeToTrades(func(_ CryptoTrade) {}, "PACOIN")
	}
	go subFunc()
	// wait for message to be written
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request fails
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 405,
			Msg:  "symbol limit exceeded",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSymbolLimitExceeded)

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request interrupted by slow client
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 407,
			Msg:  "slow client",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSlowClient)

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request fails due to incorrect due to incorrect subscription for feed
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 410,
			Msg:  "invalid subscribe action for this feed",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeInvalidForFeed)
}

func assertBufferFills(t *testing.T, bufferFills, trades chan Trade, minID, maxID, minTrades int) {
	timer := time.NewTimer(100 * time.Millisecond)
	count := maxID - minID + 1
	minFills := count - minTrades - 1

	sumTrades := 0
	sumFills := 0
	for i := 0; i < minFills; i++ {
		select {
		case trade := <-bufferFills:
			sumFills++
			assert.LessOrEqual(t, int64(minID), trade.ID)
			assert.GreaterOrEqual(t, int64(maxID), trade.ID)
		case <-timer.C:
			require.Fail(t, "buffer fill timeout")
		}
	}

	for i := minFills; i < count; i++ {
		select {
		case trade := <-bufferFills:
			sumFills++
			assert.LessOrEqual(t, int64(minID), trade.ID)
			assert.GreaterOrEqual(t, int64(maxID), trade.ID)
		case trade := <-trades:
			sumTrades++
			assert.LessOrEqual(t, int64(minID), trade.ID)
			assert.GreaterOrEqual(t, int64(maxID), trade.ID)
		}
	}

	assert.LessOrEqual(t, minFills, sumFills)
	assert.LessOrEqual(t, minTrades, sumTrades)
}

func TestCallbacksCalledOnBufferFill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connection := newMockConn()
	defer connection.close()

	writeInitialFlowMessagesToConn(t, connection, subscriptions{trades: []string{"ALPACA"}})

	const bufferSize = 2
	bufferFills := make(chan Trade, 10)
	trades := make(chan Trade)

	c := NewStocksClient(marketdata.IEX,
		WithBufferSize(bufferSize),
		WithBufferFillCallback(func(msg []byte) {
			trades := []tradeWithT{}
			require.NoError(t, msgpack.Unmarshal(msg, &trades))
			bufferFills <- Trade{
				ID:     trades[0].ID,
				Symbol: trades[0].Symbol,
			}
		}),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) { return connection, nil }),
		WithTrades(func(t Trade) { trades <- t }, "ALPACA"),
	)
	require.NoError(t, c.Connect(ctx))

	// The buffer size is 2 but we send at least 4 (2 buffer size, 1
	// messageProcessor goroutine, 1 extra) trades to have a buffer fill. The
	// messageProcessor goroutines can read c.in while the rest of messages can
	// be queued in the buffered channel.
	for id := int64(1); id <= 4; id++ {
		connection.readCh <- serializeToMsgpack(t, []any{tradeWithT{Type: "t", Symbol: "ALPACA", ID: id}})
	}
	assertBufferFills(t, bufferFills, trades, 1, 4, bufferSize)

	for id := int64(5); id <= 10; id++ {
		connection.readCh <- serializeToMsgpack(t, []any{tradeWithT{Type: "t", Symbol: "ALPACA", ID: id}})
	}
	assertBufferFills(t, bufferFills, trades, 5, 10, bufferSize)
}

func TestPingFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(_ context.Context, _ url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			testTicker := newTestTicker()
			newPingTicker = func() ticker {
				return testTicker
			}

			var c streamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient(marketdata.IEX, WithReconnectSettings(1, 0), withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(marketdata.US,
					WithReconnectSettings(1, 0), withConnCreator(connCreator))
			case cryptoPerpTests:
				c = NewCryptoClient(marketdata.GLOBAL,
					WithReconnectSettings(1, 0), withConnCreator(connCreator))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			// replacing connCreator with a new one that returns an error
			// so connection will not be reestablished
			connErr := errors.New("no connection")
			//nolint:errcheck // False positive "error return value is not checked"
			switch tt.name {
			case stocksTests:
				c.(*StocksClient).connCreator = func(_ context.Context, _ url.URL) (conn, error) {
					return nil, connErr
				}
			case cryptoTests:
				c.(*CryptoClient).connCreator = func(_ context.Context, _ url.URL) (conn, error) {
					return nil, connErr
				}
			case cryptoPerpTests:
				c.(*CryptoClient).connCreator = func(_ context.Context, _ url.URL) (conn, error) {
					return nil, connErr
				}
			}
			// disabling ping (but not closing the connection altogether!)
			connection.pingDisabled = true
			// triggering a ping
			testTicker.Tick()

			err = <-c.Terminated()
			require.Error(t, err)
			require.ErrorIs(t, err, connErr)
		})
	}
}

func TestCoreFunctionalityStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:      []string{"ALPACA"},
		quotes:      []string{"ALPACA"},
		bars:        []string{"ALPACA"},
		updatedBars: []string{"ALPACA"},
		dailyBars:   []string{"LPACA"},
		statuses:    []string{"ALPACA"},
		imbalances:  []string{"ALPACA"},
		lulds:       []string{"ALPACA"},
	})

	trades := make(chan Trade, 10)
	quotes := make(chan Quote, 10)
	bars := make(chan Bar, 10)
	updatedBars := make(chan Bar, 10)
	dailyBars := make(chan Bar, 10)
	tradingStatuses := make(chan TradingStatus, 10)
	imbalances := make(chan Imbalance, 10)
	lulds := make(chan LULD, 10)
	cancelErrors := make(chan TradeCancelError, 10)
	corrections := make(chan TradeCorrection, 10)
	c := NewStocksClient(marketdata.IEX,
		WithTrades(func(t Trade) { trades <- t }, "ALPACA"),
		WithQuotes(func(q Quote) { quotes <- q }, "ALPCA"),
		WithBars(func(b Bar) { bars <- b }, "ALPACA"),
		WithUpdatedBars(func(b Bar) { updatedBars <- b }, "ALPACA"),
		WithDailyBars(func(b Bar) { dailyBars <- b }, "LPACA"),
		WithStatuses(func(ts TradingStatus) { tradingStatuses <- ts }, "ALPACA"),
		WithImbalances(func(i Imbalance) { imbalances <- i }, "ALPACA"),
		WithLULDs(func(l LULD) { lulds <- l }, "ALPACA"),
		WithCancelErrors(func(tce TradeCancelError) { cancelErrors <- tce }),
		WithCorrections(func(tc TradeCorrection) { corrections <- tc }),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending two bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		barWithT{
			Type:       "b",
			Symbol:     "ALPACA",
			Volume:     322,
			TradeCount: 3,
			VWAP:       3.1415,
		},
		barWithT{
			Type:       "d",
			Symbol:     "LPACA",
			Open:       35.1,
			High:       36.2,
			TradeCount: 25,
			VWAP:       35.64,
		},
		quoteWithT{
			Type:    "q",
			Symbol:  "ALPACA",
			BidSize: 42,
		},
	})
	// sending a trade
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradeWithT{
			Type:   "t",
			Symbol: "ALPACA",
			ID:     123,
		},
	})
	// sending an updated bar
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		barWithT{
			Type:   "u",
			Symbol: "ALPACA",
			High:   36.7,
		},
	})
	// sending a trading status
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradingStatusWithT{
			Type:       "s",
			Symbol:     "ALPACA",
			StatusCode: "H",
			ReasonCode: "T12",
			Tape:       "C",
		},
	})
	// sending an order imbalance
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		imbalanceWithT{
			Type:   "i",
			Symbol: "ALPACA",
			Price:  123.456,
			Tape:   "C",
		},
	})
	// sending a LULD
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		luldWithT{
			Type:           "l",
			Symbol:         "ALPACA",
			LimitUpPrice:   42.1789,
			LimitDownPrice: 32.2123,
			Indicator:      "B",
			Tape:           "C",
		},
	})
	// sending a trade cancel error
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradeCancelErrorWithT{
			Type:              "x",
			Symbol:            "ALPACA",
			ID:                123,
			CancelErrorAction: "C",
			Tape:              "C",
		},
	})
	// sending a correction
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		tradeCorrectionWithT{
			Type:                "c",
			Symbol:              "ALPACA",
			OriginalID:          123,
			OriginalPrice:       123.123,
			OriginalSize:        123,
			OriginalConditions:  []string{" ", "7", "V"},
			CorrectedID:         124,
			CorrectedPrice:      124.124,
			CorrectedSize:       124,
			CorrectedConditions: []string{" ", "7", "Z", "V"},
			Tape:                "C",
		},
	})

	// checking contents
	select {
	case bar := <-bars:
		assert.EqualValues(t, 322, bar.Volume)
		assert.EqualValues(t, 3, bar.TradeCount)
		assert.EqualValues(t, 3.1415, bar.VWAP)
	case <-time.After(time.Second):
		require.Fail(t, "no bar received in time")
	}

	select {
	case bar := <-updatedBars:
		assert.EqualValues(t, 36.7, bar.High)
	case <-time.After(time.Second):
		require.Fail(t, "no updated bar received in time")
	}

	select {
	case dailyBar := <-dailyBars:
		assert.EqualValues(t, 35.1, dailyBar.Open)
		assert.EqualValues(t, 36.2, dailyBar.High)
		assert.EqualValues(t, 25, dailyBar.TradeCount)
		assert.EqualValues(t, 35.64, dailyBar.VWAP)
	case <-time.After(time.Second):
		require.Fail(t, "no daily bar received in time")
	}

	select {
	case quote := <-quotes:
		assert.EqualValues(t, 42, quote.BidSize)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}

	select {
	case trade := <-trades:
		assert.EqualValues(t, 123, trade.ID)
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}

	select {
	case ts := <-tradingStatuses:
		assert.Equal(t, "T12", ts.ReasonCode)
	case <-time.After(time.Second):
		require.Fail(t, "no trading status received in time")
	}

	select {
	case oi := <-imbalances:
		assert.EqualValues(t, 123.456, oi.Price)
	case <-time.After(time.Second):
		require.Fail(t, "no imbalance received in time")
	}

	select {
	case l := <-lulds:
		assert.EqualValues(t, 42.1789, l.LimitUpPrice)
		assert.EqualValues(t, 32.2123, l.LimitDownPrice)
		assert.Equal(t, "B", l.Indicator)
		assert.Equal(t, "C", l.Tape)
	case <-time.After(time.Second):
		require.Fail(t, "no LULD received in time")
	}

	select {
	case tce := <-cancelErrors:
		assert.EqualValues(t, 123, tce.ID)
		assert.Equal(t, "C", tce.CancelErrorAction)
		assert.Equal(t, "C", tce.Tape)
	case <-time.After(time.Second):
		require.Fail(t, "no cancel error received in time")
	}

	select {
	case tc := <-corrections:
		assert.EqualValues(t, 123, tc.OriginalID)
		assert.EqualValues(t, 123.123, tc.OriginalPrice)
		assert.EqualValues(t, 123, tc.OriginalSize)
		assert.EqualValues(t, []string{" ", "7", "V"}, tc.OriginalConditions)
		assert.EqualValues(t, 124, tc.CorrectedID)
		assert.EqualValues(t, 124.124, tc.CorrectedPrice)
		assert.EqualValues(t, 124, tc.CorrectedSize)
		assert.EqualValues(t, []string{" ", "7", "Z", "V"}, tc.CorrectedConditions)
		assert.Equal(t, "C", tc.Tape)
	case <-time.After(time.Second):
		require.Fail(t, "no correction received in time")
	}
}

func TestCoreFunctionalityCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:      []string{"BTC/USD"},
		quotes:      []string{"ETH/USD"},
		bars:        []string{"LTC/USD"},
		updatedBars: []string{"BCH/USD"},
		dailyBars:   []string{"BCH/USD"},
		orderbooks:  []string{"SHIB/USD"},
	})

	trades := make(chan CryptoTrade, 10)
	quotes := make(chan CryptoQuote, 10)
	bars := make(chan CryptoBar, 10)
	updatedBars := make(chan CryptoBar, 10)
	dailyBars := make(chan CryptoBar, 10)
	orderbooks := make(chan CryptoOrderbook, 10)
	c := NewCryptoClient(marketdata.US,
		WithCryptoTrades(func(t CryptoTrade) { trades <- t }, "BTC/USD"),
		WithCryptoQuotes(func(q CryptoQuote) { quotes <- q }, "ETH/USD"),
		WithCryptoBars(func(b CryptoBar) { bars <- b }, "LTC/USD"),
		WithCryptoUpdatedBars(func(b CryptoBar) { updatedBars <- b }, "BCH/USD"),
		WithCryptoDailyBars(func(b CryptoBar) { dailyBars <- b }, "BCH/USD"),
		WithCryptoOrderbooks(func(ob CryptoOrderbook) { orderbooks <- ob }, "SHIB/USD"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending three bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoBarWithT{
			Type:       "b",
			Symbol:     "LTC/USD",
			Exchange:   "TEST",
			Volume:     10,
			TradeCount: 3,
			VWAP:       123.45,
		},
		cryptoBarWithT{
			Type:       "d",
			Symbol:     "LTC/USD",
			Exchange:   "TES7",
			Open:       196.05,
			High:       196.3,
			TradeCount: 32,
			VWAP:       196.21,
		},
		cryptoBarWithT{
			Type:       "u",
			Symbol:     "ECH/USD",
			Exchange:   "TES7",
			TradeCount: 33,
		},
		cryptoQuoteWithT{
			Type:     "q",
			Symbol:   "ETH/USD",
			AskPrice: 2848.53,
			AskSize:  3.12,
			BidPrice: 2712.82,
			BidSize:  3.982,
			Exchange: "TEST",
		},
	})
	// sending a trade
	ts := time.Date(2021, 6, 2, 15, 12, 4, 3534, time.UTC)
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoTradeWithT{
			Type:      "t",
			Symbol:    "BTC/USD",
			Timestamp: ts,
			Exchange:  "TST",
			Price:     4123.123,
			Size:      34.876,
			ID:        25,
			TakerSide: "S",
		},
	})
	// sending an orderbook
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoOrderbookWithT{
			Type:     "o",
			Symbol:   "SHIB/USD",
			Exchange: "TST",
			Bids: []cryptoOrderbookEntry{
				{Price: 111.1, Size: 222.2},
				{Price: 333.3, Size: 444.4},
			},
			Asks: []cryptoOrderbookEntry{
				{Price: 555.5, Size: 666.6},
				{Price: 777.7, Size: 888.8},
			},
		},
	})

	// checking contents
	select {
	case bar := <-bars:
		assert.EqualValues(t, 10, bar.Volume)
		assert.EqualValues(t, 3, bar.TradeCount)
		assert.EqualValues(t, 123.45, bar.VWAP)
		assert.Equal(t, "TEST", bar.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no bar received in time")
	}

	select {
	case bar := <-updatedBars:
		assert.EqualValues(t, 33, bar.TradeCount)
	case <-time.After(time.Second):
		require.Fail(t, "no updated bar received in time")
	}

	select {
	case dailyBar := <-dailyBars:
		assert.EqualValues(t, 196.05, dailyBar.Open)
		assert.EqualValues(t, 196.3, dailyBar.High)
		assert.EqualValues(t, 32, dailyBar.TradeCount)
		assert.EqualValues(t, 196.21, dailyBar.VWAP)
		assert.Equal(t, "TES7", dailyBar.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no daily bar received in time")
	}

	select {
	case quote := <-quotes:
		assert.Equal(t, "ETH/USD", quote.Symbol)
		assert.EqualValues(t, 2848.53, quote.AskPrice)
		assert.EqualValues(t, 3.12, quote.AskSize)
		assert.EqualValues(t, 3.982, quote.BidSize)
		assert.EqualValues(t, "TEST", quote.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}

	select {
	case trade := <-trades:
		assert.True(t, trade.Timestamp.Equal(ts))
		assert.EqualValues(t, "S", trade.TakerSide)
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}

	select {
	case ob := <-orderbooks:
		assert.Equal(t, "SHIB/USD", ob.Symbol)
		assert.Len(t, ob.Bids, 2)
		assert.Len(t, ob.Asks, 2)
	case <-time.After(time.Second):
		require.Fail(t, "no orderbook received in time")
	}
}

func TestCoreFunctionalityOption(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	const spx1 = "SPXW240308P05120000"
	const spx2 = "SPXW240308P05075000"
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades: []string{spx1},
		quotes: []string{spx2},
	})

	trades := make(chan OptionTrade, 10)
	quotes := make(chan OptionQuote, 10)
	c := NewOptionClient(marketdata.US,
		WithOptionTrades(func(t OptionTrade) { trades <- t }, spx1),
		WithOptionQuotes(func(q OptionQuote) { quotes <- q }, spx2),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	connection.readCh <- serializeToMsgpack(t, []interface{}{
		optionTradeWithT{
			Type:      "t",
			Symbol:    spx1,
			Exchange:  "C",
			Price:     5.06,
			Size:      1,
			Timestamp: time.Date(2024, 3, 8, 11, 41, 24, 727071744, time.UTC),
			Condition: "I",
		},
		optionQuoteWithT{
			Type:        "q",
			Symbol:      spx2,
			BidExchange: "C",
			BidPrice:    0.7,
			BidSize:     476,
			AskExchange: "C",
			AskPrice:    0.8,
			AskSize:     921,
			Timestamp:   time.Date(2024, 3, 8, 12, 3, 19, 245168896, time.UTC),
			Condition:   "B",
		},
	})

	// checking contents
	select {
	case trade := <-trades:
		assert.Equal(t, spx1, trade.Symbol)
		assert.Equal(t, "C", trade.Exchange)
		assert.Equal(t, 5.06, trade.Price)
		assert.Equal(t, "I", trade.Condition)
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}

	select {
	case quote := <-quotes:
		assert.Equal(t, spx2, quote.Symbol)
		assert.EqualValues(t, 0.8, quote.AskPrice)
		assert.EqualValues(t, 921, quote.AskSize)
		assert.EqualValues(t, 0.7, quote.BidPrice)
		assert.EqualValues(t, "C", quote.BidExchange)
		assert.Equal(t, "B", quote.Condition)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}
}

func TestCoreFunctionalityNews(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		news: []string{"AAPL"},
	})

	news := make(chan News, 10)
	c := NewNewsClient(
		WithNews(func(n News) { news <- n }, "AAPL"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	ts := time.Date(2021, 6, 2, 15, 12, 4, 3534, time.UTC)
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		newsWithT{
			Type:      "n",
			ID:        5,
			Author:    "author",
			CreatedAt: ts,
			UpdatedAt: ts,
			Headline:  "headline",
			Summary:   "summary",
			Content:   "",
			URL:       "http://example.com",
			Symbols:   []string{"AAPL", "TSLA"},
			NewField:  5,
		},
	})

	// checking contents
	select {
	case n := <-news:
		assert.Equal(t, "author", n.Author)
		assert.Equal(t, "headline", n.Headline)
		assert.Equal(t, "summary", n.Summary)
		assert.Equal(t, "", n.Content)
		assert.Equal(t, "http://example.com", n.URL)
		assert.Equal(t, []string{"AAPL", "TSLA"}, n.Symbols)
	case <-time.After(time.Second):
		require.Fail(t, "no news received in time")
	}
}

func writeInitialFlowMessagesToConn(
	t *testing.T, conn *mockConn, sub subscriptions,
) {
	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []controlWithT{
		{
			Type: "success",
			Msg:  "connected",
		},
	})
	// server accepts authentication
	conn.readCh <- serializeToMsgpack(t, []controlWithT{
		{
			Type: "success",
			Msg:  "authenticated",
		},
	})

	if sub.noSubscribeCallNecessary() {
		return
	}

	// server accepts subscription
	conn.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:         "subscription",
			Trades:       sub.trades,
			Quotes:       sub.quotes,
			Bars:         sub.bars,
			UpdatedBars:  sub.updatedBars,
			DailyBars:    sub.dailyBars,
			Statuses:     sub.statuses,
			Imbalances:   sub.imbalances,
			LULDs:        sub.lulds,
			CancelErrors: sub.trades, // Subscribe automatically.
			Corrections:  sub.trades, // Subscribe automatically.
			Orderbooks:   sub.orderbooks,
			News:         sub.news,
		},
	})
}

func checkInitialMessagesSentByClient(
	t *testing.T, m *mockConn, key, secret string, sub subscriptions,
) {
	// auth
	auth := expectWrite(t, m)
	require.Equal(t, "auth", auth["action"])
	require.Equal(t, key, auth["key"])
	require.Equal(t, secret, auth["secret"])

	if sub.noSubscribeCallNecessary() {
		return
	}

	// subscribe
	s := expectWrite(t, m)
	require.Equal(t, "subscribe", s["action"])
	require.ElementsMatch(t, sub.trades, s["trades"])
	require.ElementsMatch(t, sub.quotes, s["quotes"])
	require.ElementsMatch(t, sub.bars, s["bars"])
	require.ElementsMatch(t, sub.updatedBars, s["updatedBars"])
	require.ElementsMatch(t, sub.dailyBars, s["dailyBars"])
	require.ElementsMatch(t, sub.statuses, s["statuses"])
	require.ElementsMatch(t, sub.imbalances, s["imbalances"])
	require.ElementsMatch(t, sub.lulds, s["lulds"])
	require.NotContains(t, s, "cancelErrors")
	require.NotContains(t, s, "corrections")
	require.ElementsMatch(t, sub.orderbooks, s["orderbooks"])
}

func TestCryptoClientConstructURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		exchanges []string
		baseURL   string
		expected  string
	}{
		{
			name:     "wss_noexchange",
			baseURL:  "wss://test.example.com/test/crypto",
			expected: "wss://test.example.com/test/crypto/us",
		},
		{
			name:     "ws_noexchange",
			baseURL:  "ws://test.example.com/test/crypto",
			expected: "ws://test.example.com/test/crypto/us",
		},
		{
			name:     "http_noexchange",
			baseURL:  "http://test.example.com/test/crypto",
			expected: "ws://test.example.com/test/crypto/us",
		},
		{
			name:     "https_noexchange",
			baseURL:  "https://test.example.com/test/crypto",
			expected: "wss://test.example.com/test/crypto/us",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewCryptoClient(
				marketdata.US,
				WithBaseURL(test.baseURL),
			)
			got, err := c.constructURL()
			require.NoError(t, err)
			assert.EqualValues(t, test.expected, got.String())
		})
	}
}

func TestSubscribeBeforeConnectCryptoPerp(t *testing.T) {
	c := NewCryptoClient(marketdata.GLOBAL, WithBaseURL(perpBaseURL))

	err := c.SubscribeToTrades(func(_ CryptoTrade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(_ CryptoQuote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToUpdatedBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(_ CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToOrderbooks(func(_ CryptoOrderbook) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToPerpPricing(func(_ CryptoPerpPricing) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromUpdatedBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromOrderbooks()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromPerpPricing()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeCalledButClientTerminatesCryptoPerp(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(marketdata.GLOBAL,
		WithBaseURL(perpBaseURL),
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) { return connection, nil }))

	ctx, cancel := context.WithCancel(context.Background())

	err := c.Connect(ctx)
	require.NoError(t, err)

	checkInitialMessagesSentByClient(t, connection, "my_key", "my_secret", c.sub)
	subErrCh := make(chan error, 1)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(_ CryptoTrade) {}, "BTC-PERP")
	}

	// calling Subscribe
	go subFunc()
	// making sure Subscribe got called
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"BTC-PERP"}, subMsg["trades"])
	// terminating the client
	cancel()

	err = <-subErrCh
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeInterrupted)

	// Subscribing after the client has terminated results in an error
	err = c.SubscribeToQuotes(func(_ CryptoQuote) {}, "BTC-PERP")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeAfterTerminated)
}

func TestSubscribeFailsDueToErrorCryptoPerp(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(marketdata.GLOBAL,
		WithBaseURL(perpBaseURL),
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "my_key", "my_secret", c.sub)

	// attempting sub change
	subRes := make(chan error)
	subFunc := func() {
		subRes <- c.SubscribeToTrades(func(_ CryptoTrade) {}, "BTC-PERP")
	}
	go subFunc()
	// wait for message to be written
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"BTC-PERP"}, subMsg["trades"])

	// sub change request fails
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 405,
			Msg:  "symbol limit exceeded",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSymbolLimitExceeded)

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"BTC-PERP"}, subMsg["trades"])

	// sub change request interrupted by slow client
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 407,
			Msg:  "slow client",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSlowClient)

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"BTC-PERP"}, subMsg["trades"])

	// sub change request fails due to incorrect due to incorrect subscription for feed
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: msgTypeError,
			Code: 410,
			Msg:  "invalid subscribe action for this feed",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.ErrorIs(t, err, ErrSubscriptionChangeInvalidForFeed)
}

func TestCoreFunctionalityCryptoPerp(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:      []string{"BTC-PERP"},
		quotes:      []string{"BTC-PERP"},
		bars:        []string{"BTC-PERP"},
		updatedBars: []string{"BTC-PERP"},
		dailyBars:   []string{"BTC-PERP"},
		orderbooks:  []string{"BTC-PERP"},
		pricing:     []string{"BTC-PERP"},
	})

	trades := make(chan CryptoTrade, 10)
	quotes := make(chan CryptoQuote, 10)
	bars := make(chan CryptoBar, 10)
	updatedBars := make(chan CryptoBar, 10)
	dailyBars := make(chan CryptoBar, 10)
	orderbooks := make(chan CryptoOrderbook, 10)
	pricing := make(chan CryptoPerpPricing, 10)
	c := NewCryptoClient(marketdata.GLOBAL,
		WithBaseURL(perpBaseURL),
		WithCryptoTrades(func(t CryptoTrade) { trades <- t }, "BTC-PERP"),
		WithCryptoQuotes(func(q CryptoQuote) { quotes <- q }, "BTC-PERP"),
		WithCryptoBars(func(b CryptoBar) { bars <- b }, "BTC-PERP"),
		WithCryptoUpdatedBars(func(b CryptoBar) { updatedBars <- b }, "BTC-PERP"),
		WithCryptoDailyBars(func(b CryptoBar) { dailyBars <- b }, "BTC-PERP"),
		WithCryptoOrderbooks(func(ob CryptoOrderbook) { orderbooks <- ob }, "BTC-PERP"),
		WithCryptoPerpPricing(func(p CryptoPerpPricing) { pricing <- p }, "BTC-PERP"),
		withConnCreator(func(_ context.Context, _ url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending three bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoBarWithT{
			Type:       "b",
			Symbol:     "BTC-PERP",
			Exchange:   "PERP",
			Volume:     10,
			TradeCount: 3,
			VWAP:       123.45,
		},
		cryptoBarWithT{
			Type:       "d",
			Symbol:     "BTC-PERP",
			Exchange:   "PERP",
			Open:       196.05,
			High:       196.3,
			TradeCount: 32,
			VWAP:       196.21,
		},
		cryptoBarWithT{
			Type:       "u",
			Symbol:     "BTC-PERP",
			Exchange:   "PERP",
			TradeCount: 33,
		},
		cryptoQuoteWithT{
			Type:     "q",
			Symbol:   "BTC-PERP",
			AskPrice: 2848.53,
			AskSize:  3.12,
			BidPrice: 2712.82,
			BidSize:  3.982,
			Exchange: "PERP",
		},
	})
	// sending a trade
	ts := time.Date(2021, 6, 2, 15, 12, 4, 3534, time.UTC)
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoTradeWithT{
			Type:      "t",
			Symbol:    "BTC-PERP",
			Timestamp: ts,
			Exchange:  "PERP",
			Price:     4123.123,
			Size:      34.876,
			ID:        25,
			TakerSide: "S",
		},
	})
	// sending an orderbook
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoOrderbookWithT{
			Type:     "o",
			Symbol:   "BTC-PERP",
			Exchange: "PERP",
			Bids: []cryptoOrderbookEntry{
				{Price: 111.1, Size: 222.2},
				{Price: 333.3, Size: 444.4},
			},
			Asks: []cryptoOrderbookEntry{
				{Price: 555.5, Size: 666.6},
				{Price: 777.7, Size: 888.8},
			},
		},
	})
	// sending perp pricing
	nft := time.Date(2021, 6, 2, 20, 12, 4, 3534, time.UTC)
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoPerpPricingWithT{
			Type:            "p",
			Symbol:          "BTC-PERP",
			Exchange:        "PERP",
			Timestamp:       ts,
			IndexPrice:      0.72817,
			MarkPrice:       0.727576296,
			FundingRate:     -0.000439065,
			OpenInterest:    46795,
			NextFundingTime: nft,
		},
	})

	// checking contents
	select {
	case bar := <-bars:
		assert.EqualValues(t, 10, bar.Volume)
		assert.EqualValues(t, 3, bar.TradeCount)
		assert.EqualValues(t, 123.45, bar.VWAP)
		assert.Equal(t, "PERP", bar.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no bar received in time")
	}

	select {
	case bar := <-updatedBars:
		assert.EqualValues(t, 33, bar.TradeCount)
	case <-time.After(time.Second):
		require.Fail(t, "no updated bar received in time")
	}

	select {
	case dailyBar := <-dailyBars:
		assert.EqualValues(t, 196.05, dailyBar.Open)
		assert.EqualValues(t, 196.3, dailyBar.High)
		assert.EqualValues(t, 32, dailyBar.TradeCount)
		assert.EqualValues(t, 196.21, dailyBar.VWAP)
		assert.Equal(t, "PERP", dailyBar.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no daily bar received in time")
	}

	select {
	case quote := <-quotes:
		assert.Equal(t, "BTC-PERP", quote.Symbol)
		assert.EqualValues(t, 2848.53, quote.AskPrice)
		assert.EqualValues(t, 3.12, quote.AskSize)
		assert.EqualValues(t, 3.982, quote.BidSize)
		assert.EqualValues(t, "PERP", quote.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no quote received in time")
	}

	select {
	case trade := <-trades:
		assert.True(t, trade.Timestamp.Equal(ts))
		assert.EqualValues(t, "S", trade.TakerSide)
	case <-time.After(time.Second):
		require.Fail(t, "no trade received in time")
	}

	select {
	case ob := <-orderbooks:
		assert.Equal(t, "BTC-PERP", ob.Symbol)
		assert.Len(t, ob.Bids, 2)
		assert.Len(t, ob.Asks, 2)
	case <-time.After(time.Second):
		require.Fail(t, "no orderbook received in time")
	}

	select {
	case p := <-pricing:
		assert.Equal(t, "BTC-PERP", p.Symbol)
		assert.EqualValues(t, 0.72817, p.IndexPrice)
		assert.EqualValues(t, 0.727576296, p.MarkPrice)
		assert.EqualValues(t, -0.000439065, p.FundingRate)
		assert.EqualValues(t, 46795, p.OpenInterest)
		assert.EqualValues(t, "PERP", p.Exchange)
	case <-time.After(time.Second):
		require.Fail(t, "no perp pricing received in time")
	}
}

func TestCryptoPerpClientConstructURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		exchanges []string
		baseURL   string
		expected  string
	}{
		{
			name:     "wss_noexchange",
			baseURL:  "wss://test.example.com/test/crypto-perps",
			expected: "wss://test.example.com/test/crypto-perps/global",
		},
		{
			name:     "ws_noexchange",
			baseURL:  "ws://test.example.com/test/crypto-perps",
			expected: "ws://test.example.com/test/crypto-perps/global",
		},
		{
			name:     "http_noexchange",
			baseURL:  "http://test.example.com/test/crypto-perps",
			expected: "ws://test.example.com/test/crypto-perps/global",
		},
		{
			name:     "https_noexchange",
			baseURL:  "https://test.example.com/test/crypto-perps",
			expected: "wss://test.example.com/test/crypto-perps/global",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewCryptoClient(
				marketdata.GLOBAL,
				WithBaseURL(test.baseURL),
			)
			got, err := c.constructURL()
			require.NoError(t, err)
			assert.EqualValues(t, test.expected, got.String())
		})
	}
}
