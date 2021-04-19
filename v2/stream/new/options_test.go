package new

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	var tests = []struct {
		name           string
		dataProxyWSVal string
		expectedHost   string
	}{
		{name: "no_data_ws_proxy", dataProxyWSVal: "", expectedHost: "https://stream.data.alpaca.markets"},
		{name: "data_ws_proxy", dataProxyWSVal: "test", expectedHost: "test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Setenv("APCA_API_KEY_ID", "testkey")
			os.Setenv("APCA_API_SECRET_KEY", "testsecret")
			os.Setenv("DATA_PROXY_WS", test.dataProxyWSVal)

			o := defaultOptions()

			assert.EqualValues(t, newStdLog(), o.logger)
			assert.EqualValues(t, test.expectedHost, o.host)
			assert.EqualValues(t, "testkey", o.key)
			assert.EqualValues(t, "testsecret", o.secret)
			assert.EqualValues(t, 20, o.reconnectLimit)
			assert.EqualValues(t, 150*time.Millisecond, o.reconnectDelay)
			assert.EqualValues(t, 1, o.processorCount)
			assert.EqualValues(t, 100000, o.bufferSize)
			assert.EqualValues(t, []string{}, o.trades)
			assert.EqualValues(t, []string{}, o.quotes)
			assert.EqualValues(t, []string{}, o.bars)
			// NOTE: function equality can not be tested well
		})
	}
}

func TestConfigure(t *testing.T) {
	// NOTE: we are also testing the various options and their apply
	// even though the test is testing multiple things they're closely related

	c := client{}
	o := options{}
	logger := &stdLog{logger: log.New(os.Stdout, "TEST", log.LUTC)}
	o.apply(
		WithLogger(logger),
		WithHost("testhost"),
		WithCredentials("testkey", "testsecret"),
		WithReconnectSettings(42, 322*time.Nanosecond),
		WithProcessors(322),
		WithBufferSize(1000000),
		WithTrades(func(t Trade) {}, "ALPACA"),
		WithQuotes(func(q Quote) {}, "AL", "PACA"),
		WithBars(func(b Bar) {}, "ALP", "ACA"),
	)
	c.configure(o)

	assert.EqualValues(t, logger, c.logger)
	assert.EqualValues(t, "testhost", c.host)
	assert.EqualValues(t, "testkey", c.key)
	assert.EqualValues(t, "testsecret", c.secret)
	assert.EqualValues(t, 42, c.reconnectLimit)
	assert.EqualValues(t, 322*time.Nanosecond, c.reconnectDelay)
	assert.EqualValues(t, 322, c.processorCount)
	assert.EqualValues(t, 1000000, c.bufferSize)
	assert.EqualValues(t, []string{"ALPACA"}, c.trades)
	assert.EqualValues(t, []string{"AL", "PACA"}, c.quotes)
	assert.EqualValues(t, []string{"ALP", "ACA"}, c.bars)
	// NOTE: function equality can not be tested well
}
