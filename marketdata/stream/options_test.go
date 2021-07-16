package stream

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	var tests = []struct {
		name            string
		dataProxyWSVal  string
		expectedBaseURL string
	}{
		{name: "no_data_ws_proxy", dataProxyWSVal: "",
			expectedBaseURL: "https://stream.data.alpaca.markets/v2"},
		{name: "data_ws_proxy", dataProxyWSVal: "test", expectedBaseURL: "test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv("APCA_API_KEY_ID", "testkey")
			os.Setenv("APCA_API_SECRET_KEY", "testsecret")
			os.Setenv("DATA_PROXY_WS", test.dataProxyWSVal)

			o := defaultStockOptions()

			assert.EqualValues(t, DefaultLogger(), o.logger)
			assert.EqualValues(t, test.expectedBaseURL, o.baseURL)
			assert.EqualValues(t, "testkey", o.key)
			assert.EqualValues(t, "testsecret", o.secret)
			assert.EqualValues(t, 20, o.reconnectLimit)
			assert.EqualValues(t, 150*time.Millisecond, o.reconnectDelay)
			assert.EqualValues(t, 1, o.processorCount)
			assert.EqualValues(t, 100000, o.bufferSize)
			assert.EqualValues(t, []string{}, o.sub.trades)
			assert.EqualValues(t, []string{}, o.sub.quotes)
			assert.EqualValues(t, []string{}, o.sub.bars)
			assert.EqualValues(t, []string{}, o.sub.dailyBars)
			assert.EqualValues(t, []string{}, o.sub.statuses)
			assert.EqualValues(t, []string{}, o.sub.lulds)
			// NOTE: function equality can not be tested well
		})
	}
}

func TestConfigureStocks(t *testing.T) {
	// NOTE: we are also testing the various options and their apply
	// even though the test is testing multiple things they're closely related

	logger := ErrorOnlyLogger()
	c := NewStocksClient("iex",
		WithLogger(logger),
		WithBaseURL("testhost"),
		WithCredentials("testkey", "testsecret"),
		WithReconnectSettings(42, 322*time.Nanosecond),
		WithProcessors(322),
		WithBufferSize(1000000),
		WithTrades(func(t Trade) {}, "ALPACA"),
		WithQuotes(func(q Quote) {}, "AL", "PACA"),
		WithBars(func(b Bar) {}, "ALP", "ACA"),
		WithDailyBars(func(b Bar) {}, "LPACA"),
		WithStatuses(func(ts TradingStatus) {}, "ALPACA"),
		WithLULDs(func(l LULD) {}, "ALPA", "CA"),
	).(*stocksClient)

	assert.EqualValues(t, logger, c.logger)
	assert.EqualValues(t, "testhost", c.baseURL)
	assert.EqualValues(t, "testkey", c.key)
	assert.EqualValues(t, "testsecret", c.secret)
	assert.EqualValues(t, 42, c.reconnectLimit)
	assert.EqualValues(t, 322*time.Nanosecond, c.reconnectDelay)
	assert.EqualValues(t, 322, c.processorCount)
	assert.EqualValues(t, 1000000, c.bufferSize)
	assert.EqualValues(t, []string{"ALPACA"}, c.sub.trades)
	assert.EqualValues(t, []string{"AL", "PACA"}, c.sub.quotes)
	assert.EqualValues(t, []string{"ALP", "ACA"}, c.sub.bars)
	assert.EqualValues(t, []string{"LPACA"}, c.sub.dailyBars)
	assert.EqualValues(t, []string{"ALPACA"}, c.sub.statuses)
	assert.EqualValues(t, []string{"ALPA", "CA"}, c.sub.lulds)
	// NOTE: function equality can not be tested well
}
