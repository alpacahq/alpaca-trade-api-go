package stream

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	stocksTests = "stocks"
	cryptoTests = "crypto"
)

var tests = []struct {
	name string
}{
	{name: stocksTests},
	{name: cryptoTests},
}

func TestConnectFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithReconnectSettings(1, 0),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
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

			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrNoConnected))
		})
	}
}

func TestConnectWithInvalidURL(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			case cryptoTests:
				c = NewCryptoClient(
					WithBaseURL("http://192.168.0.%31/"),
					WithReconnectSettings(1, 0))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)

			assert.Error(t, err)
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
		{code: 411, msg: "insufficient scope", err: ErrInsufficientScope},
	}
	for _, tt := range tests {
		for _, ie := range irrecoverableErrors {
			t.Run(tt.name+"/"+ie.msg, func(t *testing.T) {
				connection := newMockConn()
				defer connection.close()
				connCreator := func(ctx context.Context, u url.URL) (conn, error) {
					return connection, nil
				}

				// if the error weren't irrecoverable then we would be retrying for quite a while
				// and the test would time out
				reconnectSettings := WithReconnectSettings(20, time.Second)

				var c StreamClient
				switch tt.name {
				case stocksTests:
					c = NewStocksClient("iex", reconnectSettings, withConnCreator(connCreator))
				case cryptoTests:
					c = NewCryptoClient(reconnectSettings, withConnCreator(connCreator))
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
						Type: "error",
						Code: ie.code,
						Msg:  ie.msg,
					},
				})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				err := c.Connect(ctx)

				assert.Error(t, err)
				assert.True(t, errors.Is(err, ie.err))
			})
		}
	}
}

func TestContextCancelledBeforeConnect(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex",
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(
					WithBaseURL("http://test.paca/v2"),
					withConnCreator(connCreator))
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := c.Connect(ctx)
			assert.Error(t, err)
			assert.Error(t, <-c.Terminated())
		})
	}
}

func TestConnectSucceeds(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex", withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(withConnCreator(connCreator))
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

func TestSubscribeBeforeConnectStocks(t *testing.T) {
	c := NewStocksClient("iex")

	err := c.SubscribeToTrades(func(trade Trade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(quote Quote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(bar Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(bar Bar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToStatuses(func(ts TradingStatus) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToLULDs(func(luld LULD) {})
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
	err = c.UnsubscribeFromLULDs()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeBeforeConnectCrypto(t *testing.T) {
	c := NewCryptoClient()

	err := c.SubscribeToTrades(func(trade CryptoTrade) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(quote CryptoQuote) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(bar CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.SubscribeToDailyBars(func(bar CryptoBar) {})
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
	err = c.UnsubscribeFromDailyBars()
	assert.Equal(t, ErrSubscriptionChangeBeforeConnect, err)
}

func TestSubscribeMultipleCallsStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewStocksClient("iex", withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}

	// calling two Subscribes at the same time and also calling a sub change
	// without modifying symbols (should succeed immediately)
	go subFunc()
	err = c.SubscribeToTrades(func(trade Trade) {})
	assert.NoError(t, err)
	go subFunc()

	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeAlreadyInProgress))
}

func TestSubscribeCalledButClientTerminatesCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))

	ctx, cancel := context.WithCancel(context.Background())

	err := c.Connect(ctx)
	require.NoError(t, err)

	checkInitialMessagesSentByClient(t, connection, "my_key", "my_secret", c.(*cryptoClient).sub)
	subErrCh := make(chan error, 1)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade CryptoTrade) {}, "PACOIN")
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
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeInterrupted))

	// Subscribing after the client has terminated results in an error
	err = c.SubscribeToQuotes(func(quote CryptoQuote) {}, "BTCUSD", "ETCUSD")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeAfterTerminated))
}

func TestSubscriptionTimeout(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	mockTimeAfterCh := make(chan time.Time)
	timeAfter = func(d time.Duration) <-chan time.Time {
		return mockTimeAfterCh
	}
	defer func() {
		timeAfter = time.After
	}()

	c := NewStocksClient("iex",
		WithCredentials("a", "b"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "a", "b", subscriptions{})

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}

	go subFunc()
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])

	mockTimeAfterCh <- time.Now()
	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeTimeout), "actual: %s", err)

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

	c := NewStocksClient("iex",
		WithCredentials("a", "b"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "a", "b", subscriptions{})

	subErrCh := make(chan error, 2)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}

	go subFunc()
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 410,
			Msg:  "invalid subscribe action for this feed",
		},
	})
	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionChangeInvalidForFeed), "actual: %s", err)
}

func TestSubscripitionAcrossConnectionIssues(t *testing.T) {
	conn1 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn1, subscriptions{})

	key := "testkey"
	secret := "testsecret"
	c := NewStocksClient("iex",
		WithCredentials(key, secret),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
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
		subRes <- c.SubscribeToTrades(func(trade Trade) {}, "AL", "PACA")
	}()
	sub := expectWrite(t, conn1)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades1, sub["trades"])

	// shutting down the first connection
	conn2 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn2, subscriptions{})
	c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
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
	require.ElementsMatch(t, trades1, c.(*stocksClient).sub.trades)

	// the connection is shut down and the new one isn't established for a while
	conn3 := newMockConn()
	defer conn3.close()
	c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
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
	require.ElementsMatch(t, []string{"PACA"}, c.(*stocksClient).sub.trades)
}

func TestSubscribeFailsDueToError(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{})

	c := NewCryptoClient(
		WithCredentials("my_key", "my_secret"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
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
		subRes <- c.SubscribeToTrades(func(trade CryptoTrade) {}, "PACOIN")
	}
	go subFunc()
	// wait for message to be written
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request fails
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 405,
			Msg:  "symbol limit exceeded",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSymbolLimitExceeded))

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request interrupted by slow client
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 407,
			Msg:  "slow client",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSlowClient))

	// attempting another sub change
	go subFunc()
	// wait for message to be written
	subMsg = expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"PACOIN"}, subMsg["trades"])

	// sub change request fails due to incorrect due to incorrect subscription for feed
	connection.readCh <- serializeToMsgpack(t, []errorWithT{
		{
			Type: "error",
			Code: 410,
			Msg:  "invalid subscribe action for this feed",
		},
	})

	// making sure the subscription request has failed
	err = <-subRes
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSubscriptionChangeInvalidForFeed))
}

func TestPingFails(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connection := newMockConn()
			defer connection.close()
			connCreator := func(ctx context.Context, u url.URL) (conn, error) {
				return connection, nil
			}

			writeInitialFlowMessagesToConn(t, connection, subscriptions{})

			testTicker := newTestTicker()
			newPingTicker = func() ticker {
				return testTicker
			}

			var c StreamClient
			switch tt.name {
			case stocksTests:
				c = NewStocksClient("iex", WithReconnectSettings(1, 0), withConnCreator(connCreator))
			case cryptoTests:
				c = NewCryptoClient(WithReconnectSettings(1, 0), withConnCreator(connCreator))
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := c.Connect(ctx)
			require.NoError(t, err)

			// replacing connCreator with a new one that returns an error
			// so connection will not be reestablished
			connErr := errors.New("no connection")
			switch tt.name {
			case stocksTests:
				c.(*stocksClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
					return nil, connErr
				}
			case cryptoTests:
				c.(*cryptoClient).connCreator = func(ctx context.Context, u url.URL) (conn, error) {
					return nil, connErr
				}
			}
			// disabling ping (but not closing the connection alltogether!)
			connection.pingDisabled = true
			// triggering a ping
			testTicker.Tick()

			err = <-c.Terminated()
			assert.Error(t, err)
			assert.True(t, errors.Is(err, connErr))
		})
	}
}

func TestCoreFunctionalityStocks(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:    []string{"ALPACA"},
		quotes:    []string{"ALPACA"},
		bars:      []string{"ALPACA"},
		dailyBars: []string{"LPACA"},
		statuses:  []string{"ALPACA"},
		lulds:     []string{"ALPACA"},
	})

	trades := make(chan Trade, 10)
	quotes := make(chan Quote, 10)
	bars := make(chan Bar, 10)
	dailyBars := make(chan Bar, 10)
	tradingStatuses := make(chan TradingStatus, 10)
	lulds := make(chan LULD, 10)
	c := NewStocksClient("iex",
		WithTrades(func(t Trade) { trades <- t }, "ALPACA"),
		WithQuotes(func(q Quote) { quotes <- q }, "ALPCA"),
		WithBars(func(b Bar) { bars <- b }, "ALPACA"),
		WithDailyBars(func(b Bar) { dailyBars <- b }, "LPACA"),
		WithStatuses(func(ts TradingStatus) { tradingStatuses <- ts }, "ALPACA"),
		WithLULDs(func(l LULD) { lulds <- l }, "ALPACA"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
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
	case l := <-lulds:
		assert.EqualValues(t, 42.1789, l.LimitUpPrice)
		assert.EqualValues(t, 32.2123, l.LimitDownPrice)
		assert.Equal(t, "B", l.Indicator)
		assert.Equal(t, "C", l.Tape)
	case <-time.After(time.Second):
		require.Fail(t, "no LULD received in time")
	}
}

func TestCoreFunctionalityCrypto(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, subscriptions{
		trades:    []string{"BTCUSD"},
		quotes:    []string{"ETHUSD"},
		bars:      []string{"LTCUSD"},
		dailyBars: []string{"BCHUSD"},
	})

	trades := make(chan CryptoTrade, 10)
	quotes := make(chan CryptoQuote, 10)
	bars := make(chan CryptoBar, 10)
	dailyBars := make(chan CryptoBar, 10)
	c := NewCryptoClient(
		WithCryptoTrades(func(t CryptoTrade) { trades <- t }, "BTCUSD"),
		WithCryptoQuotes(func(q CryptoQuote) { quotes <- q }, "ETHUSD"),
		WithCryptoBars(func(b CryptoBar) { bars <- b }, "LTCUSD"),
		WithCryptoDailyBars(func(b CryptoBar) { dailyBars <- b }, "BCHUSD"),
		withConnCreator(func(ctx context.Context, u url.URL) (conn, error) {
			return connection, nil
		}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending two bars and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		cryptoBarWithT{
			Type:       "b",
			Symbol:     "LTCUSD",
			Exchange:   "TEST",
			Volume:     10,
			TradeCount: 3,
			VWAP:       123.45,
		},
		cryptoBarWithT{
			Type:       "d",
			Symbol:     "LTCUSD",
			Exchange:   "TES7",
			Open:       196.05,
			High:       196.3,
			TradeCount: 32,
			VWAP:       196.21,
		},
		cryptoQuoteWithT{
			Type:     "q",
			Symbol:   "ETHUSD",
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
			Symbol:    "BTCUSD",
			Timestamp: ts,
			Exchange:  "TST",
			Price:     4123.123,
			Size:      34.876,
			Id:        25,
			TakerSide: "S",
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
		assert.Equal(t, "ETHUSD", quote.Symbol)
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
			Type:      "subscription",
			Trades:    sub.trades,
			Quotes:    sub.quotes,
			Bars:      sub.bars,
			DailyBars: sub.dailyBars,
			Statuses:  sub.statuses,
			LULDs:     sub.lulds,
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
	require.ElementsMatch(t, sub.dailyBars, s["dailyBars"])
	require.ElementsMatch(t, sub.statuses, s["statuses"])
	require.ElementsMatch(t, sub.lulds, s["lulds"])
}

func TestCryptoClientConstructURL(t *testing.T) {
	for _, test := range []struct {
		name      string
		exchanges []string
		baseUrl   string
		expected  string
	}{
		{
			name:     "wss_noexchange",
			baseUrl:  "wss://test.example.com/test/crypto",
			expected: "wss://test.example.com/test/crypto",
		},
		{
			name:      "wss_exchange",
			baseUrl:   "wss://test.example.com/test/crypto",
			exchanges: []string{"TEST", "TEST2"},
			expected:  "wss://test.example.com/test/crypto?exchanges=TEST,TEST2",
		},
		{
			name:     "ws_noexchange",
			baseUrl:  "ws://test.example.com/test/crypto",
			expected: "ws://test.example.com/test/crypto",
		},
		{
			name:     "http_noexchange",
			baseUrl:  "http://test.example.com/test/crypto",
			expected: "ws://test.example.com/test/crypto",
		},
		{
			name:     "https_noexchange",
			baseUrl:  "https://test.example.com/test/crypto",
			expected: "wss://test.example.com/test/crypto",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := NewCryptoClient(
				WithBaseURL(test.baseUrl),
				WithExchanges(test.exchanges...),
			)
			cryptoClient, ok := c.(*cryptoClient)
			require.True(t, ok)
			got, err := cryptoClient.constructURL()
			require.NoError(t, err)
			assert.EqualValues(t, test.expected, got.String())
		})
	}
}
