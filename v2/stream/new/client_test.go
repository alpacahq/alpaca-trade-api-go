package new

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectFails(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}
	c := NewClient(
		"iex",
		WithReconnectSettings(1, 0),
	)
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
}

func TestContextCancelledBeforeConnect(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}

	c := NewClient("iex")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.Connect(ctx)
	assert.Error(t, err)
	assert.Error(t, <-c.Error())
}

func TestConnectSucceeds(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}

	c := NewClient("iex")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)

	// Connect can't be called multiple times
	err = c.Connect(ctx)
	assert.Equal(t, ErrConnectCalledMultipleTimes, err)
}

func TestSubscribeBeforeConnect(t *testing.T) {
	c := NewClient("iex")

	err := c.SubscribeToTrades(func(trade Trade) {})
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
	err = c.SubscribeToQuotes(func(quote Quote) {})
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
	err = c.SubscribeToBars(func(bar Bar) {})
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
	err = c.UnsubscribeFromTrades()
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
	err = c.UnsubscribeFromQuotes()
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
	err = c.UnsubscribeFromBars()
	assert.Equal(t, ErrSubChangeBeforeConnect, err)
}

func TestSubscribeMultipleCalls(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}

	c := NewClient("iex")
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
	assert.True(t, errors.Is(err, ErrSubChangeAlreadyInProgress))
}

func TestSubscribeCalledButClientTerminates(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}

	c := NewClient("iex")
	ctx, cancel := context.WithCancel(context.Background())

	err := c.Connect(ctx)
	require.NoError(t, err)

	checkInitialMessagesSentByClient(t, connection, "", "", []string{}, []string{}, []string{})
	subErrCh := make(chan error, 1)
	subFunc := func() {
		subErrCh <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}

	// calling Subscribe
	go subFunc()
	// making sure Subscribe got called
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])
	// terminating the client
	cancel()

	err = <-subErrCh
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubChangeInterrupted))

	// Subscribing after the client has terminated results in an error
	err = c.SubscribeToQuotes(func(quote Quote) {}, "AL", "PACA")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubChangeAfterTerminated))
}

func TestSubscripitionAcrossConnectionIssues(t *testing.T) {
	conn1 := newMockConn()
	writeInitialFlowMessagesToConn(t, conn1, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) { return conn1, nil }

	key := "testkey"
	secret := "testsecret"
	c := NewClient("iex", WithCredentials(key, secret))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, conn1, key, secret, []string{}, []string{}, []string{})

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
	writeInitialFlowMessagesToConn(t, conn2, []string{}, []string{}, []string{})
	connCreator = func(ctx context.Context, u url.URL) (conn, error) { return conn2, nil }
	conn1.close()

	// checking whether the client sent what we wanted it to (auth,sub1,sub2)
	checkInitialMessagesSentByClient(t, conn2, key, secret, []string{}, []string{}, []string{})
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
	require.ElementsMatch(t, trades1, c.(*client).trades)

	// the connection is shut down and the new one isn't established for a while
	conn3 := newMockConn()
	defer conn3.close()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		time.Sleep(100 * time.Millisecond)
		writeInitialFlowMessagesToConn(t, conn3, trades1, []string{}, []string{})
		return conn3, nil
	}
	conn2.close()

	// call an unsubscribe with the connection being down
	unsubRes := make(chan error)
	go func() { unsubRes <- c.UnsubscribeFromTrades("AL") }()

	// connection starts up, proper messages (auth,sub,unsub)
	checkInitialMessagesSentByClient(t, conn3, key, secret, trades1, []string{}, []string{})
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
	require.ElementsMatch(t, []string{"PACA"}, c.(*client).trades)
}

func TestSubscribeFailsDueToError(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) { return connection, nil }

	c := NewClient("iex")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connect
	err := c.Connect(ctx)
	require.NoError(t, err)
	checkInitialMessagesSentByClient(t, connection, "", "", []string{}, []string{}, []string{})

	// attempting sub change
	subRes := make(chan error)
	subFunc := func() {
		subRes <- c.SubscribeToTrades(func(trade Trade) {}, "ALPACA")
	}
	go subFunc()
	// wait for message to be written
	subMsg := expectWrite(t, connection)
	require.Equal(t, "subscribe", subMsg["action"])
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])

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
	require.ElementsMatch(t, []string{"ALPACA"}, subMsg["trades"])

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
}

func TestPingFails(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	onpt := newPingTicker
	defer func() {
		connCreator = occ
		newPingTicker = onpt
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}
	tt := newTestTicker()
	newPingTicker = func() ticker {
		return tt
	}

	c := NewClient(
		"iex",
		WithReconnectSettings(1, 0),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := c.Connect(ctx)
	require.NoError(t, err)

	// replacing connCreator with a new one that returns an error
	// so connection will not be reestablished
	connErr := errors.New("no connection")
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return nil, connErr
	}
	// disabling ping (but not closing the connection alltogether!)
	connection.pingDisabled = true
	// triggering a ping
	tt.Tick()

	err = <-c.Error()
	assert.Error(t, err)
	assert.True(t, errors.Is(err, connErr))
}

func TestCoreFunctionality(t *testing.T) {
	connection := newMockConn()
	defer connection.close()
	writeInitialFlowMessagesToConn(t, connection, []string{}, []string{}, []string{})
	occ := connCreator
	defer func() {
		connCreator = occ
	}()
	connCreator = func(ctx context.Context, u url.URL) (conn, error) {
		return connection, nil
	}

	var trade Trade
	var quote Quote
	var bar Bar
	c := NewClient(
		"iex",
		WithTrades(func(t Trade) { trade = t }, "ALPACA"),
		WithQuotes(func(q Quote) { quote = q }, "ALPCA"),
		WithBars(func(b Bar) { bar = b }, "ALPACA"),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// connecting with the client
	err := c.Connect(ctx)
	require.NoError(t, err)

	// sending a bar and a quote
	connection.readCh <- serializeToMsgpack(t, []interface{}{
		barWithT{
			Type:   "b",
			Symbol: "ALPACA",
			Volume: 322,
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

	// waiting until the client processes them
	time.Sleep(10 * time.Millisecond)

	// checking contents
	assert.EqualValues(t, 322, bar.Volume)
	assert.EqualValues(t, 42, quote.BidSize)
	assert.EqualValues(t, 123, trade.ID)
}

func writeInitialFlowMessagesToConn(t *testing.T, conn *mockConn, trades, quotes, bars []string) {
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
	// server accepts subscription
	conn.readCh <- serializeToMsgpack(t, []subWithT{
		{
			Type:   "subscription",
			Trades: trades,
			Quotes: quotes,
			Bars:   bars,
		},
	})
}

func checkInitialMessagesSentByClient(t *testing.T, m *mockConn, key, secret string, trades, quotes, bars []string) {
	// auth
	auth := expectWrite(t, m)
	require.Equal(t, "auth", auth["action"])
	require.Equal(t, key, auth["key"])
	require.Equal(t, secret, auth["secret"])

	// subscribe
	sub := expectWrite(t, m)
	require.Equal(t, "subscribe", sub["action"])
	require.ElementsMatch(t, trades, sub["trades"])
	require.ElementsMatch(t, quotes, sub["quotes"])
	require.ElementsMatch(t, bars, sub["bars"])
}
