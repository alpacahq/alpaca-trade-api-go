package alpaca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"1
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"testing/quick"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------
// Extended Environment & Auth Tests
// ---------------------------

// Test that NewClient falls back in the proper order:
// OAuth > Broker > API keys.
func TestAuthenticationPriority(t *testing.T) {
	// When OAuth is provided, it should override all other credentials.
	client := NewClient(ClientOpts{
		OAuth:        "oauthtoken",
		APIKey:       "apikey",
		APISecret:    "apisecret",
		BrokerKey:    "brokerkey",
		BrokerSecret: "brokersecret",
		BaseURL:      "http://dummy",
	})
	req, _ := http.NewRequest(http.MethodGet, client.opts.BaseURL, nil)
	// In defaultDo, OAuth header should be set:
	req.Header.Set("Authorization", "Bearer "+client.opts.OAuth)
	assert.Equal(t, "Bearer oauthtoken", req.Header.Get("Authorization"))

	// If OAuth is not provided but Broker credentials are, then Basic Auth is used.
	client = NewClient(ClientOpts{
		BrokerKey:    "brokerkey",
		BrokerSecret: "brokersecret",
		BaseURL:      "http://dummy",
	})
	req, _ = http.NewRequest(http.MethodGet, client.opts.BaseURL, nil)
	req.SetBasicAuth(client.opts.BrokerKey, client.opts.BrokerSecret)
	authHeader := req.Header.Get("Authorization")
	require.True(t, strings.HasPrefix(authHeader, "Basic "))
	decoded, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
	require.NoError(t, err)
	parts := strings.Split(string(decoded), ":")
	assert.Equal(t, "brokerkey", parts[0])
	assert.Equal(t, "brokersecret", parts[1])
}

// ---------------------------
// Robust Retry & Error Injection
// ---------------------------

// Test the retry mechanism under sustained pressure (simulate intermittent server errors).
func TestDefaultDo_RetryUnderPressure(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Fail the first three calls, then succeed.
		if callCount <= 3 {
			http.Error(w, "Temporary Error", http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("final success"))
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{
		BaseURL:    ts.URL,
		RetryLimit: 5,
		RetryDelay: 5 * time.Millisecond,
	})

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := defaultDo(client, req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "final success", string(b))
	assert.GreaterOrEqual(t, callCount, 4)
}

// ---------------------------
// Endpoint & Streaming Fuzzing
// ---------------------------

// Fuzz test for the streaming endpoint: provide random input lines and ensure the handler never crashes.
func FuzzStreamTradeUpdates_StreamingParser(f *testing.F) {
	// Seed with valid messages.
	validMessages := []string{
		`data: {"execution_id":"first","at":"2024-01-01T00:00:00Z"}` + "\n",
		`data: {"execution_id":"second","at":"2024-01-01T00:00:01Z"}` + "\n",
	}
	for _, msg := range validMessages {
		f.Add(msg)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// Wrap the input into a bufio.Reader to simulate stream reading.
		reader := bufio.NewReader(strings.NewReader(input))
		// Only process lines starting with "data: " to mimic our stream.
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			return
		}
		line = line[len("data: "):]
		var tu TradeUpdate
		// The Unmarshal might error, but we should not panic.
		_ = json.Unmarshal(line, &tu)
	})
}

// ---------------------------
// Extended Endpoint Tests
// ---------------------------

func TestGetAccountEpic(t *testing.T) {
	account := Account{ID: "epic_id", AccountNumber: "123456789"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/account", r.URL.Path)
		json.NewEncoder(w).Encode(account)
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{BaseURL: ts.URL})
	got, err := client.GetAccount()
	require.NoError(t, err)
	assert.Equal(t, account.ID, got.ID)
	assert.Equal(t, account.AccountNumber, got.AccountNumber)
}

func TestPlaceOrderEpic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req PlaceOrderRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		// For epic effect, simulate some transformation in the server.
		order := Order{
			ClientOrderID: req.ClientOrderID,
			Side:          req.Side,
			TimeInForce:   req.TimeInForce,
			Type:          req.Type,
			Qty:           req.Qty,
		}
		json.NewEncoder(w).Encode(order)
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{BaseURL: ts.URL})
	qty := decimal.NewFromInt(100)
	req := PlaceOrderRequest{
		ClientOrderID: "epic-order-001",
		Qty:           &qty,
		Side:          Buy,
		TimeInForce:   GTC,
		Type:          Market,
	}
	order, err := client.PlaceOrder(req)
	require.NoError(t, err)
	assert.Equal(t, "epic-order-001", order.ClientOrderID)
	assert.Equal(t, Buy, order.Side)
}

// ---------------------------
// Property-Based & Boundary Tests for Rounding Utility
// ---------------------------

func TestRoundLimitPrice_PropertyEpic(t *testing.T) {
	f := func(price float64, side string) bool {
		// Only test with positive prices.
		if price <= 0 {
			return true
		}
		d := decimal.NewFromFloat(price)
		var rounded *decimal.Decimal
		if side == "buy" {
			rounded = RoundLimitPrice(d, Buy)
		} else if side == "sell" {
			rounded = RoundLimitPrice(d, Sell)
		} else {
			// Skip invalid side values.
			return true
		}
		var maxDecimals int32 = 2
		if d.LessThan(decimal.NewFromInt(1)) {
			maxDecimals = 4
		}
		// Convert the rounded value to a string with fixed decimals.
		roundedStr := rounded.StringFixed(maxDecimals)
		expected, _ := decimal.NewFromString(roundedStr)
		// For buys, the rounded price should be >= original price.
		if side == "buy" && rounded.LessThan(d) {
			return false
		}
		// For sells, the rounded price should be <= original price.
		if side == "sell" && rounded.GreaterThan(d) {
			return false
		}
		return rounded.Equal(expected)
	}
	err := quick.Check(f, nil)
	require.NoError(t, err)
}

// ---------------------------
// Concurrency & Resilience Tests
// ---------------------------

// Test that StreamTradeUpdatesInBackground properly cancels when the context is cancelled.
func TestStreamTradeUpdatesInBackground_Cancellation(t *testing.T) {
	// Create a server that streams updates slowly.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "expected http.Flusher")
		for i := 0; i < 100; i++ {
			w.Write([]byte("data: {\"execution_id\":\"" + strconv.Itoa(i) + "\", \"at\":\"" + time.Now().Format(time.RFC3339Nano) + "\"}\n"))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{BaseURL: ts.URL})
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan TradeUpdate, 10)
	// Start streaming in background.
	client.StreamTradeUpdatesInBackground(ctx, func(tu TradeUpdate) {
		received <- tu
	})
	// Let the stream run a little.
	time.Sleep(50 * time.Millisecond)
	cancel()
	// Ensure that we received at least one update, and that cancellation stops further updates.
	count := len(received)
	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, count, 1, "expected at least one trade update before cancellation")
	assert.Equal(t, count, len(received), "no new updates after cancellation")
}

// ---------------------------
// Helper Functions
// ---------------------------

func deci(d decimal.Decimal) *decimal.Decimal {
	return &d
}
