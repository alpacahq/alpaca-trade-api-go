package stream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

func TestDefaultOptions(t *testing.T) {
	tests := []struct {
		name            string
		dataProxyWSVal  string
		expectedBaseURL string
	}{
		{
			name: "no_data_ws_proxy", dataProxyWSVal: "",
			expectedBaseURL: "https://stream.data.alpaca.markets/v2",
		},
		{name: "data_ws_proxy", dataProxyWSVal: "test", expectedBaseURL: "test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APCA_API_KEY_ID", "testkey")
			t.Setenv("APCA_API_SECRET_KEY", "testsecret")
			t.Setenv("DATA_PROXY_WS", test.dataProxyWSVal)

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
			assert.EqualValues(t, []string{}, o.sub.updatedBars)
			assert.EqualValues(t, []string{}, o.sub.dailyBars)
			assert.EqualValues(t, []string{}, o.sub.statuses)
			assert.EqualValues(t, []string{}, o.sub.imbalances)
			assert.EqualValues(t, []string{}, o.sub.lulds)
			assert.EqualValues(t, []string{}, o.sub.cancelErrors)
			assert.EqualValues(t, []string{}, o.sub.corrections)
			// NOTE: function equality can not be tested well
		})
	}
}

func TestConfigureStocks(t *testing.T) {
	// NOTE: we are also testing the various options and their apply
	// even though the test is testing multiple things they're closely related

	logger := ErrorOnlyLogger()
	c := NewStocksClient(marketdata.IEX,
		WithLogger(logger),
		WithBaseURL("testhost"),
		WithCredentials("testkey", "testsecret"),
		WithReconnectSettings(42, 322*time.Nanosecond),
		WithProcessors(322),
		WithBufferSize(1000000),
		WithTrades(func(_ Trade) {}, "ALPACA"),
		WithQuotes(func(_ Quote) {}, "AL", "PACA"),
		WithBars(func(_ Bar) {}, "ALP", "ACA"),
		WithUpdatedBars(func(_ Bar) {}, "AAPL"),
		WithDailyBars(func(_ Bar) {}, "LPACA"),
		WithStatuses(func(_ TradingStatus) {}, "ALPACA"),
		WithImbalances(func(_ Imbalance) {}, "ALPACA"),
		WithLULDs(func(_ LULD) {}, "ALPA", "CA"),
		WithCancelErrors(func(_ TradeCancelError) {}),
		WithCorrections(func(_ TradeCorrection) {}),
	)

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
	assert.EqualValues(t, []string{"AAPL"}, c.sub.updatedBars)
	assert.EqualValues(t, []string{"LPACA"}, c.sub.dailyBars)
	assert.EqualValues(t, []string{"ALPACA"}, c.sub.statuses)
	assert.EqualValues(t, []string{"ALPACA"}, c.sub.imbalances)
	assert.EqualValues(t, []string{"ALPA", "CA"}, c.sub.lulds)
	assert.EqualValues(t, []string{}, c.sub.cancelErrors)
	assert.EqualValues(t, []string{}, c.sub.corrections)
	// NOTE: function equality can not be tested well
}
