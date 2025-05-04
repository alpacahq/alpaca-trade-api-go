package stream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

func TestInitializeConnectFails(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}

	res := make(chan error, 1)

	go func() {
		// client connects to the server
		res <- c.initialize(context.Background())
	}()
	// server doesn't send proper response
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"not": "correct",
		},
	})

	assert.Error(t, <-res)
}

func TestInitializeAuthError(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}

	res := make(chan error, 1)

	go func() {
		// client connects to the server
		res <- c.initialize(context.Background())
	}()
	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "connected",
		},
	})
	// server rejects the authentication attempt - 402
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 402,
			"msg":  "auth failed",
		},
	})

	err := <-res
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestInitializeAuthRetryFails(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret", logger: DefaultLogger()}
	ordm := authRetryDelayMultiplier
	arc := authRetryCount
	defer func() {
		authRetryDelayMultiplier = ordm
		authRetryCount = arc
	}()
	authRetryDelayMultiplier = 0
	// reducing retry count to simulate what happens after a lot of failures
	authRetryCount = 1

	res := make(chan error, 1)

	go func() {
		// client connects to the server
		res <- c.initialize(context.Background())
	}()

	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "connected",
		},
	})
	// client attempts to authenticate - 406
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})
	// client attempts to authenticate - 406 again
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})

	err := <-res
	require.Error(t, err)
	require.ErrorIs(t, err, ErrConnectionLimitExceeded)
}

func TestInitializeAuthRetrySucceeds(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	// Using NewClient because initialize logs during the test
	trades := []string{"AL", "PACA"}
	quotes := []string{"ALPACA"}
	bars := []string{"ALP", "ACA"}
	updatedBars := []string{"AAPL"}
	dailyBars := []string{"CLDR"}
	statuses := []string{"*"}
	imbalances := []string{"PACA", "AL"}
	lulds := []string{"AL", "PACA", "ALP"}
	c := NewStocksClient(
		marketdata.SIP,
		WithCredentials("testkey", "testsecret"),
		WithTrades(func(_ Trade) {}, trades...),
		WithQuotes(func(_ Quote) {}, quotes...),
		WithBars(func(_ Bar) {}, bars...),
		WithUpdatedBars(func(_ Bar) {}, updatedBars...),
		WithDailyBars(func(_ Bar) {}, dailyBars...),
		WithStatuses(func(_ TradingStatus) {}, statuses...),
		WithImbalances(func(_ Imbalance) {}, imbalances...),
		WithLULDs(func(_ LULD) {}, lulds...),
	)
	c.conn = conn
	ordm := authRetryDelayMultiplier
	defer func() {
		authRetryDelayMultiplier = ordm
	}()
	authRetryDelayMultiplier = 0

	res := make(chan error, 1)

	go func() {
		// client connects to the server
		res <- c.initialize(context.Background())
	}()

	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "connected",
		},
	})
	// client attempts to authenticate - 406
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})
	// client attempts to authenticate - 406 again
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})
	// client succeeds
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "authenticated",
		},
	})
	// client subscription succeeds

	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":            "subscription",
			"trades":       trades,
			"quotes":       quotes,
			"bars":         bars,
			"updatedBars":  updatedBars,
			"dailyBars":    dailyBars,
			"statuses":     statuses,
			"imbalances":   imbalances,
			"lulds":        lulds,
			"cancelErrors": trades, // Subscribed automatically with trades.
			"corrections":  trades, // Subscribed automatically with trades.
		},
	})

	require.NoError(t, <-res)
	assert.ElementsMatch(t, trades, c.sub.trades)
	assert.ElementsMatch(t, quotes, c.sub.quotes)
	assert.ElementsMatch(t, bars, c.sub.bars)
	assert.ElementsMatch(t, updatedBars, c.sub.updatedBars)
	assert.ElementsMatch(t, dailyBars, c.sub.dailyBars)
	assert.ElementsMatch(t, statuses, c.sub.statuses)
	assert.ElementsMatch(t, imbalances, c.sub.imbalances)
	assert.ElementsMatch(t, lulds, c.sub.lulds)
	assert.ElementsMatch(t, trades, c.sub.cancelErrors)
	assert.ElementsMatch(t, trades, c.sub.corrections)

	// checking whether the client sent the proper messages
	// First auth
	auth1 := expectWrite(t, conn)
	assert.Equal(t, "auth", auth1["action"])
	assert.Equal(t, "testkey", auth1["key"])
	assert.Equal(t, "testsecret", auth1["secret"])
	// Second auth
	auth2 := expectWrite(t, conn)
	assert.Equal(t, auth1, auth2)
	// Third auth
	auth3 := expectWrite(t, conn)
	assert.Equal(t, auth1, auth3)
	// Subscriptions
	sub := expectWrite(t, conn)
	assert.Equal(t, "subscribe", sub["action"])
	assert.ElementsMatch(t, trades, sub["trades"])
	assert.ElementsMatch(t, quotes, sub["quotes"])
	assert.ElementsMatch(t, bars, sub["bars"])
	assert.ElementsMatch(t, updatedBars, sub["updatedBars"])
	assert.ElementsMatch(t, dailyBars, sub["dailyBars"])
	assert.ElementsMatch(t, statuses, sub["statuses"])
	assert.ElementsMatch(t, imbalances, sub["imbalances"])
	assert.ElementsMatch(t, lulds, sub["lulds"])
	assert.NotContains(t, sub, "cancelErrors")
	assert.NotContains(t, sub, "corrections")
}

func TestInitializeSubError(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{
		conn: conn,
		sub: subscriptions{
			trades: []string{"TEST"},
		},
	}
	ordm := authRetryDelayMultiplier
	defer func() {
		authRetryDelayMultiplier = ordm
	}()
	authRetryDelayMultiplier = 0

	res := make(chan error, 1)

	go func() {
		// client connects to the server
		res <- c.initialize(context.Background())
	}()

	// server welcomes the client
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "connected",
		},
	})
	// client authenticates
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":   "success",
			"msg": "authenticated",
		},
	})
	// client subscription fails
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    msgTypeError,
			"code": 405,
			"msg":  "symbol limit exceeded",
		},
	})

	assert.Error(t, <-res)
}

func TestReadConnectedCancelled(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.readConnected(ctx)

	require.Error(t, err)
}

func TestReadConnectedContents(t *testing.T) {
	c := client{}
	tests := []struct {
		name        string
		message     []byte
		expectError bool
	}{
		{
			name: "not_array",
			message: serializeToMsgpack(t, map[string]interface{}{
				"T":   "success",
				"msg": "connected",
			}),
			expectError: true,
		},
		{
			name: "wrong_contents",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "correct",
				},
			}),
			expectError: true,
		},
		{
			name: "wrong_T",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "succez",
					"msg": "connected",
				},
			}),
			expectError: true,
		},
		{
			name: "wrong_msg",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "not_it",
				},
			}),
			expectError: true,
		},
		{
			name: "array_with_multiple_items",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "connected",
				},
				{
					"T":   "success",
					"msg": "connected",
				},
			}),
			expectError: true,
		},
		{
			name: "correct",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "connected",
				},
			}),
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newMockConn()
			defer conn.close()
			c.conn = conn

			conn.readCh <- test.message

			err := c.readConnected(context.Background())
			if test.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteAuthCancelled(t *testing.T) {
	conn := newMockConn()
	// We want to close it so that the write fails
	conn.close()
	c := client{conn: conn}

	err := c.writeAuth(context.Background())

	require.Error(t, err)
}

func TestWriteAuthContents(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "mykey", secret: "mysecret"}

	err := c.writeAuth(context.Background())

	require.NoError(t, err)
	msg := <-conn.writeCh
	var got map[string]string
	err = msgpack.Unmarshal(msg, &got)
	require.NoError(t, err)
	assert.Equal(t, "auth", got["action"])
	assert.Equal(t, "mykey", got["key"])
	assert.Equal(t, "mysecret", got["secret"])
}

func TestReadAuthResponseCancelled(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.readAuthResponse(ctx)

	require.Error(t, err)
}

func TestReadAuthResponseContents(t *testing.T) {
	c := client{}
	tests := []struct {
		name        string
		message     []byte
		expectError bool
		shouldRetry bool
	}{
		{
			name: "not_array",
			message: serializeToMsgpack(t, map[string]interface{}{
				"T":   "success",
				"msg": "connected",
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "wrong_contents_1",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "correct",
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "wrong_T",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "succez",
					"msg": "authenticated",
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "wrong_msg",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "not_it",
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "should_retry",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":    msgTypeError,
					"msg":  "connection limit exceeded",
					"code": 406,
				},
			}),
			expectError: true,
			shouldRetry: true,
		},
		{
			name: "should_not_retry_1",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":    msgTypeError,
					"code": 401,
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "should_not_retry_2",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T": msgTypeError,
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "array_with_multiple_items",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "authenticated",
				},
				{
					"T":   "success",
					"msg": "authenticated",
				},
			}),
			expectError: true,
			shouldRetry: false,
		},
		{
			name: "correct",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":   "success",
					"msg": "authenticated",
				},
			}),
			expectError: false,
			shouldRetry: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newMockConn()
			defer conn.close()
			c.conn = conn

			conn.readCh <- test.message

			err := c.readAuthResponse(context.Background())
			if test.expectError {
				require.Error(t, err)
				assert.Equal(t, test.shouldRetry, isErrorRetriable(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteSubCancelled(t *testing.T) {
	conn := newMockConn()
	// We want to close it so that the write fails
	conn.close()
	c := client{conn: conn}

	err := c.writeSub(context.Background())

	require.Error(t, err)
}

func TestWriteSubContents(t *testing.T) {
	tests := []struct {
		name        string
		trades      []string
		quotes      []string
		bars        []string
		updatedBars []string
		dailyBars   []string
		statuses    []string
		imbalances  []string
		lulds       []string
	}{
		{name: "empty"},
		{name: "trades_only", trades: []string{"ALPACA"}},
		{name: "quotes_only", quotes: []string{"AL", "PACA"}},
		{name: "bars_only", bars: []string{"A", "L", "PACA"}},
		{name: "updated_bars_only", updatedBars: []string{"AAPL"}},
		{name: "daily_bars_only", dailyBars: []string{"LPACA"}},
		{name: "statuses_only", statuses: []string{"ALP", "ACA"}},
		{name: "imbalances_only", imbalances: []string{"ALP", "ACA"}},
		{name: "lulds_only", lulds: []string{"ALPA", "CA"}},
		{
			name:      "mix",
			trades:    []string{"ALPACA"},
			quotes:    []string{"A", "L", "PACA"},
			dailyBars: []string{"*"},
		},
		{
			name:        "complete",
			trades:      []string{"ALPACA"},
			quotes:      []string{"ALPACA"},
			bars:        []string{"ALPACA"},
			updatedBars: []string{"ALPACA"},
			dailyBars:   []string{"ALPACA"},
			statuses:    []string{"ALPACA"},
			imbalances:  []string{"ALPACA"},
			lulds:       []string{"ALPCA"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newMockConn()
			defer conn.close()
			c := client{
				conn: conn,
				sub: subscriptions{
					trades:      test.trades,
					quotes:      test.quotes,
					bars:        test.bars,
					updatedBars: test.updatedBars,
					dailyBars:   test.dailyBars,
					statuses:    test.statuses,
					imbalances:  test.imbalances,
					lulds:       test.lulds,
				},
			}

			err := c.writeSub(context.Background())

			require.NoError(t, err)
			msg := <-conn.writeCh
			var got struct {
				Action      string   `msgpack:"action"`
				Trades      []string `msgpack:"trades"`
				Quotes      []string `msgpack:"quotes"`
				Bars        []string `msgpack:"bars"`
				UpdatedBars []string `msgpack:"updatedBars"`
				DailyBars   []string `msgpack:"dailyBars"`
				Statuses    []string `msgpack:"statuses"`
				Imbalances  []string `msgpack:"imbalances"`
				LULDs       []string `msgpack:"lulds"`
			}
			err = msgpack.Unmarshal(msg, &got)
			require.NoError(t, err)
			assert.Equal(t, "subscribe", got.Action)
			assert.ElementsMatch(t, test.trades, got.Trades)
			assert.ElementsMatch(t, test.quotes, got.Quotes)
			assert.ElementsMatch(t, test.bars, got.Bars)
			assert.ElementsMatch(t, test.updatedBars, got.UpdatedBars)
			assert.ElementsMatch(t, test.dailyBars, got.DailyBars)
			assert.ElementsMatch(t, test.statuses, got.Statuses)
			assert.ElementsMatch(t, test.imbalances, got.Imbalances)
			assert.ElementsMatch(t, test.lulds, got.LULDs)
		})
	}
}

func TestReadSubResponseCancelled(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.readSubResponse(ctx)

	require.Error(t, err)
}

func TestReadSubResponseContents(t *testing.T) {
	tests := []struct {
		name        string
		message     []byte
		expectError bool
		trades      []string
		quotes      []string
		bars        []string
		updatedBars []string
		dailyBars   []string
	}{
		{
			name: "not_array",
			message: serializeToMsgpack(t, map[string]interface{}{
				"T": "subscription",
			}),
			expectError: true,
		},
		{
			name: "wrong_contents",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "correct",
				},
			}),
			expectError: true,
		},
		{
			name: msgTypeError,
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":    msgTypeError,
					"code": 402,
					"msg":  "auth failed",
				},
			}),
			expectError: true,
		},
		{
			name: "array_with_multiple_items",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T": "subscription",
				},
				{
					"T": "subscription",
				},
			}),
			expectError: true,
		},
		{
			name: "empty",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":      "subscription",
					"trades": []string{},
					"quotes": []string{},
					"bars":   []string{},
				},
			}),
			expectError: false,
		},
		{
			name: "success",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":           "subscription",
					"trades":      []string{"ALPACA"},
					"quotes":      []string{"AL", "PACA"},
					"bars":        []string{"AL", "PA", "CA"},
					"updatedBars": []string{"MSFT", "NIO"},
					"dailyBars":   []string{"LPACA"},
				},
			}),
			expectError: false,
			trades:      []string{"ALPACA"},
			quotes:      []string{"AL", "PACA"},
			bars:        []string{"AL", "PA", "CA"},
			updatedBars: []string{"MSFT", "NIO"},
			dailyBars:   []string{"LPACA"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newMockConn()
			defer conn.close()
			c := client{conn: conn}

			conn.readCh <- test.message

			err := c.readSubResponse(context.Background())
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, test.trades, c.sub.trades)
				assert.ElementsMatch(t, test.quotes, c.sub.quotes)
				assert.ElementsMatch(t, test.bars, c.sub.bars)
				assert.ElementsMatch(t, test.updatedBars, c.sub.updatedBars)
				assert.ElementsMatch(t, test.dailyBars, c.sub.dailyBars)
			}
		})
	}
}

func expectWrite(t *testing.T, mockConn *mockConn) map[string]interface{} {
	var a map[string]interface{}
	data := <-mockConn.writeCh
	err := msgpack.Unmarshal(data, &a)
	require.NoError(t, err)
	return a
}

func serializeToMsgpack(t *testing.T, v interface{}) []byte {
	m, err := msgpack.Marshal(v)
	require.NoError(t, err)
	return m
}
