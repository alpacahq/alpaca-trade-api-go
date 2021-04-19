package stream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// tradeWithT is the incoming trade message that also contains the T type key
type tradeWithT struct {
	Type       string    `msgpack:"T"`
	ID         int64     `msgpack:"i"`
	Symbol     string    `msgpack:"S"`
	Exchange   string    `msgpack:"x"`
	Price      float64   `msgpack:"p"`
	Size       uint32    `msgpack:"s"`
	Timestamp  time.Time `msgpack:"t"`
	Conditions []string  `msgpack:"c"`
	Tape       string    `msgpack:"z"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// quoteWithT is the incoming quote message that also contains the T type key
type quoteWithT struct {
	Type        string    `msgpack:"T"`
	Symbol      string    `msgpack:"S"`
	BidExchange string    `msgpack:"bx"`
	BidPrice    float64   `msgpack:"bp"`
	BidSize     uint32    `msgpack:"bs"`
	AskExchange string    `msgpack:"ax"`
	AskPrice    float64   `msgpack:"ap"`
	AskSize     uint32    `msgpack:"as"`
	Timestamp   time.Time `msgpack:"t"`
	Conditions  []string  `msgpack:"c"`
	Tape        string    `msgpack:"z"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// barWithT is the incoming bar message that also contains the T type key
type barWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Open      float64   `msgpack:"o"`
	High      float64   `msgpack:"h"`
	Low       float64   `msgpack:"l"`
	Close     float64   `msgpack:"c"`
	Volume    uint64    `msgpack:"v"`
	Timestamp time.Time `msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

type other struct {
	Type     string `msgpack:"T"`
	Whatever string `msgpack:"w"`
}

type controlWithT struct {
	Type string `msgpack:"T"`
	Msg  string `msgpack:"msg"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"N"`
}

// errorWithT is the incoming error message that also contains the T type key
type errorWithT struct {
	Type string `msgpack:"T"`
	Msg  string `msgpack:"msg"`
	Code int    `msgpack:"code"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"N"`
}

// subWithT is the incoming error message that also contains the T type key
type subWithT struct {
	Type   string   `msgpack:"T"`
	Trades []string `msgpack:"trades"`
	Quotes []string `msgpack:"quotes"`
	Bars   []string `msgpack:"bars"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"N"`
}

var testTime = time.Date(2021, 03, 04, 15, 16, 17, 18, time.UTC)

var testTrade = tradeWithT{
	Type:       "t",
	ID:         42,
	Symbol:     "TEST",
	Exchange:   "X",
	Price:      100,
	Size:       10,
	Timestamp:  testTime,
	Conditions: []string{" "},
	Tape:       "A",
}

var testQuote = quoteWithT{
	Type:        "q",
	Symbol:      "TEST",
	BidExchange: "B",
	BidPrice:    99.9,
	BidSize:     100,
	AskExchange: "A",
	AskPrice:    100.1,
	AskSize:     200,
	Timestamp:   testTime,
	Conditions:  []string{"R"},
	Tape:        "B",
}

var testBar = barWithT{
	Type:      "b",
	Symbol:    "TEST",
	Open:      100,
	High:      101.2,
	Low:       98.67,
	Close:     101.1,
	Volume:    2560,
	Timestamp: time.Date(2021, 03, 05, 16, 0, 0, 0, time.UTC),
}

var testOther = other{
	Type:     "o",
	Whatever: "whatever",
}

var testError = errorWithT{
	Type: "error",
	Msg:  "test",
	Code: 322,
}

var testSubMessage1 = subWithT{
	Type:   "subscription",
	Trades: []string{"ALPACA"},
	Quotes: []string{},
	Bars:   []string{},
}

var testSubMessage2 = subWithT{
	Type:   "subscription",
	Trades: []string{"ALPACA"},
	Quotes: []string{"AL", "PACA"},
	Bars:   []string{"ALP", "ACA"},
}

func TestHandleMessages(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{testOther, testTrade, testQuote, testBar, testError, testSubMessage1, testSubMessage2})
	require.NoError(t, err)

	emh := errMessageHandler
	smh := subMessageHandler
	defer func() {
		errMessageHandler = emh
		subMessageHandler = smh
	}()

	subscriptionMessages := make([]subscriptionMessage, 0)

	var em errorMessage
	errMessageHandler = func(c *client, e errorMessage) error {
		em = e
		return nil
	}
	subMessageHandler = func(c *client, s subscriptionMessage) error {
		subscriptionMessages = append(subscriptionMessages, s)
		return nil
	}

	c := &client{}
	var trade Trade
	c.tradeHandler = func(t Trade) {
		trade = t
	}

	var quote Quote
	c.quoteHandler = func(q Quote) {
		quote = q
	}
	var bar Bar
	c.barHandler = func(b Bar) {
		bar = b
	}

	err = c.handleMessage(b)
	require.NoError(t, err)

	assert.EqualValues(t, testTrade.ID, trade.ID)
	assert.EqualValues(t, testTrade.Symbol, trade.Symbol)
	assert.EqualValues(t, testTrade.Exchange, trade.Exchange)
	assert.EqualValues(t, testTrade.Price, trade.Price)
	assert.EqualValues(t, testTrade.Size, trade.Size)
	assert.True(t, trade.Timestamp.Equal(testTime))
	assert.EqualValues(t, testTrade.Conditions, trade.Conditions)
	assert.EqualValues(t, testTrade.Tape, trade.Tape)

	assert.EqualValues(t, testQuote.Symbol, quote.Symbol)
	assert.EqualValues(t, testQuote.BidExchange, quote.BidExchange)
	assert.EqualValues(t, testQuote.BidPrice, quote.BidPrice)
	assert.EqualValues(t, testQuote.BidSize, quote.BidSize)
	assert.EqualValues(t, testQuote.AskExchange, quote.AskExchange)
	assert.EqualValues(t, testQuote.AskPrice, quote.AskPrice)
	assert.EqualValues(t, testQuote.AskSize, quote.AskSize)
	assert.True(t, quote.Timestamp.Equal(testTime))
	assert.EqualValues(t, testQuote.Conditions, quote.Conditions)
	assert.EqualValues(t, testQuote.Tape, quote.Tape)

	assert.EqualValues(t, testBar.Symbol, bar.Symbol)
	assert.EqualValues(t, testBar.Open, bar.Open)
	assert.EqualValues(t, testBar.High, bar.High)
	assert.EqualValues(t, testBar.Low, bar.Low)
	assert.EqualValues(t, testBar.Close, bar.Close)
	assert.EqualValues(t, testBar.Volume, bar.Volume)

	assert.EqualValues(t, testError.Code, em.code)
	assert.EqualValues(t, testError.Msg, em.msg)

	require.Len(t, subscriptionMessages, 2)
	assert.EqualValues(t, testSubMessage1.Trades, subscriptionMessages[0].trades)
	assert.EqualValues(t, testSubMessage1.Quotes, subscriptionMessages[0].quotes)
	assert.EqualValues(t, testSubMessage1.Bars, subscriptionMessages[0].bars)
	assert.EqualValues(t, testSubMessage2.Trades, subscriptionMessages[1].trades)
	assert.EqualValues(t, testSubMessage2.Quotes, subscriptionMessages[1].quotes)
	assert.EqualValues(t, testSubMessage2.Bars, subscriptionMessages[1].bars)
}

func BenchmarkHandleMessages(b *testing.B) {
	msgs, _ := msgpack.Marshal([]interface{}{testTrade, testQuote, testBar})
	c := &client{
		tradeHandler: func(trade Trade) {},
		quoteHandler: func(quote Quote) {},
		barHandler:   func(bar Bar) {},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.handleMessage(msgs)
	}
}
