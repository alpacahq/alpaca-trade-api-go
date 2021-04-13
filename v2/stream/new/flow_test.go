package new

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

type initializeResult struct {
	err           error
	irrecoverable bool
}

func TestInitializeConnectFails(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}

	res := make(chan initializeResult, 1)

	go func() {
		// client connects to the server
		err, irrecoverable := c.initialize(context.Background())
		res <- initializeResult{err: err, irrecoverable: irrecoverable}
	}()
	// server doesn't send proper response
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"not": "correct",
		},
	})

	r := <-res
	assert.Error(t, r.err)
	assert.False(t, r.irrecoverable)
}

func TestInitializeAuthError(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}

	res := make(chan initializeResult, 1)

	go func() {
		// client connects to the server
		err, irrecoverable := c.initialize(context.Background())
		res <- initializeResult{err: err, irrecoverable: irrecoverable}
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
			"T":    "error",
			"code": 402,
			"msg":  "auth failed",
		},
	})

	r := <-res
	assert.Error(t, r.err)
	assert.True(t, r.irrecoverable)
}

func TestInitializeAuthRetryFails(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}
	ordm := authRetryDelayMultiplier
	arc := authRetryCount
	defer func() {
		authRetryDelayMultiplier = ordm
		authRetryCount = arc
	}()
	authRetryDelayMultiplier = 0
	// reducing retry count to simulate what happens after a lot of failures
	authRetryCount = 1

	res := make(chan initializeResult, 1)

	go func() {
		// client connects to the server
		err, irrecoverable := c.initialize(context.Background())
		res <- initializeResult{err: err, irrecoverable: irrecoverable}
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
			"T":    "error",
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})
	// client attempts to authenticate - 406 again
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    "error",
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})

	r := <-res
	assert.Error(t, r.err)
	assert.False(t, r.irrecoverable)
}

func TestInitializeAuthRetrySucceeds(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	// Using NewClient because initialize logs during the test
	trades := []string{"AL", "PACA"}
	quotes := []string{"ALPACA"}
	bars := []string{"ALP", "ACA"}
	c := NewClient(
		"sip",
		WithCredentials("testkey", "testsecret"),
		WithTrades(func(t Trade) {}, trades...),
		WithQuotes(func(q Quote) {}, quotes...),
		WithBars(func(b Bar) {}, bars...),
	).(*client)
	c.conn = conn
	ordm := authRetryDelayMultiplier
	defer func() {
		authRetryDelayMultiplier = ordm
	}()
	authRetryDelayMultiplier = 0

	res := make(chan initializeResult, 1)

	go func() {
		// client connects to the server
		err, irrecoverable := c.initialize(context.Background())
		res <- initializeResult{err: err, irrecoverable: irrecoverable}
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
			"T":    "error",
			"code": 406,
			"msg":  "connection limit exceeded",
		},
	})
	// client attempts to authenticate - 406 again
	conn.readCh <- serializeToMsgpack(t, []map[string]interface{}{
		{
			"T":    "error",
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
			"T":      "subscription",
			"trades": trades,
			"quotes": quotes,
			"bars":   bars,
		},
	})

	r := <-res
	assert.NoError(t, r.err)
	assert.False(t, r.irrecoverable)
	assert.ElementsMatch(t, trades, c.trades)
	assert.ElementsMatch(t, quotes, c.quotes)
	assert.ElementsMatch(t, bars, c.bars)

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
}

func TestInitializeSubError(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn}
	ordm := authRetryDelayMultiplier
	defer func() {
		authRetryDelayMultiplier = ordm
	}()
	authRetryDelayMultiplier = 0

	res := make(chan initializeResult, 1)

	go func() {
		// client connects to the server
		err, irrecoverable := c.initialize(context.Background())
		res <- initializeResult{err: err, irrecoverable: irrecoverable}
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
			"T":    "error",
			"code": 405,
			"msg":  "symbol limit exceeded",
		},
	})

	r := <-res
	assert.Error(t, r.err)
	assert.False(t, r.irrecoverable)
}

func TestReadConnectedCancelled(t *testing.T) {
	conn := newMockConn()
	defer conn.close()
	c := client{conn: conn, key: "key", secret: "secret"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.readConnected(ctx)

	assert.Error(t, err)
}

func TestReadConnectedContents(t *testing.T) {
	c := client{}
	var tests = []struct {
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
				assert.Error(t, err)
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

	assert.Error(t, err)
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
	_, err := c.readAuthResponse(ctx)

	assert.Error(t, err)
}

func TestReadAuthResponseContents(t *testing.T) {
	c := client{}
	var tests = []struct {
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
					"T":    "error",
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
					"T":    "error",
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
					"T": "error",
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

			shouldRetry, err := c.readAuthResponse(context.Background())
			if test.expectError {
				assert.Error(t, err)
				assert.Equal(t, test.shouldRetry, shouldRetry)
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

	assert.Error(t, err)
}

func TestWriteSubContents(t *testing.T) {
	var tests = []struct {
		name   string
		trades []string
		quotes []string
		bars   []string
	}{
		{"empty", []string{}, []string{}, []string{}},
		{"trades_only", []string{"ALPACA"}, []string{}, []string{}},
		{"quotes_only", []string{}, []string{"AL", "PACA"}, []string{}},
		{"bars_only", []string{}, []string{}, []string{"A", "L", "PACA"}},
		{"mix", []string{"ALPACA"}, []string{"A", "L", "PACA"}, []string{}},
		{"complete", []string{"ALPACA"}, []string{"ALPACA"}, []string{"ALPACA"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conn := newMockConn()
			defer conn.close()
			c := client{
				conn:   conn,
				trades: test.trades,
				quotes: test.quotes,
				bars:   test.bars,
			}

			err := c.writeSub(context.Background())

			require.NoError(t, err)
			msg := <-conn.writeCh
			var got struct {
				Action string   `msgpack:"action"`
				Trades []string `msgpack:"trades"`
				Quotes []string `msgpack:"quotes"`
				Bars   []string `msgpack:"bars"`
			}
			err = msgpack.Unmarshal(msg, &got)
			require.NoError(t, err)
			assert.Equal(t, "subscribe", got.Action)
			assert.ElementsMatch(t, test.trades, got.Trades)
			assert.ElementsMatch(t, test.quotes, got.Quotes)
			assert.ElementsMatch(t, test.bars, got.Bars)
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

	assert.Error(t, err)
}

func TestReadSubResponseContents(t *testing.T) {
	var tests = []struct {
		name        string
		message     []byte
		expectError bool
		trades      []string
		quotes      []string
		bars        []string
	}{
		{
			name: "not_array",
			message: serializeToMsgpack(t, map[string]interface{}{
				"T": "subscription",
			}),
			expectError: true,
			trades:      []string{},
			quotes:      []string{},
			bars:        []string{},
		},
		{
			name: "wrong_contents",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"not": "correct",
				},
			}),
			expectError: true,
			trades:      []string{},
			quotes:      []string{},
			bars:        []string{},
		},
		{
			name: "error",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":    "error",
					"code": 402,
					"msg":  "auth failed",
				},
			}),
			expectError: true,
			trades:      []string{},
			quotes:      []string{},
			bars:        []string{},
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
			trades:      []string{},
			quotes:      []string{},
			bars:        []string{},
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
			trades:      []string{},
			quotes:      []string{},
			bars:        []string{},
		},
		{
			name: "success",
			message: serializeToMsgpack(t, []map[string]interface{}{
				{
					"T":      "subscription",
					"trades": []string{"ALPACA"},
					"quotes": []string{"AL", "PACA"},
					"bars":   []string{"AL", "PA", "CA"},
				},
			}),
			expectError: false,
			trades:      []string{"ALPACA"},
			quotes:      []string{"AL", "PACA"},
			bars:        []string{"AL", "PA", "CA"},
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, test.trades, c.trades)
				assert.ElementsMatch(t, test.quotes, c.quotes)
				assert.ElementsMatch(t, test.bars, c.bars)
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
	if err != nil {
		require.Failf(t, "msgpack marshal error", "v", err)
	}
	return m
}
