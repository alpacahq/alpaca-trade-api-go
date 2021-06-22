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
	Type       string    `msgpack:"T"`
	Symbol     string    `msgpack:"S"`
	Open       float64   `msgpack:"o"`
	High       float64   `msgpack:"h"`
	Low        float64   `msgpack:"l"`
	Close      float64   `msgpack:"c"`
	Volume     uint64    `msgpack:"v"`
	Timestamp  time.Time `msgpack:"t"`
	TradeCount uint64    `msgpack:"n"`
	VWAP       float64   `msgpack:"vw"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"new"`
}

// tradingStatusWithT is the incoming trading status message that also contains the T type key
type tradingStatusWithT struct {
	Type       string    `msgpack:"T"`
	Symbol     string    `msgpack:"S"`
	StatusCode string    `msgpack:"sc"`
	StatusMsg  string    `msgpack:"sm"`
	ReasonCode string    `msgpack:"rc"`
	ReasonMsg  string    `msgpack:"rm"`
	Timestamp  time.Time `msgpack:"t"`
	Tape       string    `msgpack:"z"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoTradeWithT is the incoming crypto trade message that also contains the T type key
type cryptoTradeWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Price     float64   `msgpack:"p"`
	Size      float64   `msgpack:"s"`
	Timestamp time.Time `msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoQuoteWithT is the incoming crypto quote message that also contains the T type key
type cryptoQuoteWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	BidPrice  float64   `msgpack:"bp"`
	AskPrice  float64   `msgpack:"ap"`
	Timestamp time.Time `msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoBarWithT is the incoming crypto bar message that also contains the T type key
type cryptoBarWithT struct {
	Type       string    `msgpack:"T"`
	Symbol     string    `msgpack:"S"`
	Open       float64   `msgpack:"o"`
	High       float64   `msgpack:"h"`
	Low        float64   `msgpack:"l"`
	Close      float64   `msgpack:"c"`
	Volume     float64   `msgpack:"v"`
	Timestamp  time.Time `msgpack:"t"`
	TradeCount uint64    `msgpack:"n"`
	VWAP       float64   `msgpack:"vw"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"new"`
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
	Type      string   `msgpack:"T"`
	Trades    []string `msgpack:"trades"`
	Quotes    []string `msgpack:"quotes"`
	Bars      []string `msgpack:"bars"`
	DailyBars []string `msgpack:"dailyBars"`
	Statuses  []string `msgpack:"statuses"`
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
	Type:       "b",
	Symbol:     "TEST",
	Open:       100,
	High:       101.2,
	Low:        98.67,
	Close:      101.1,
	Volume:     2560,
	Timestamp:  time.Date(2021, 03, 05, 16, 0, 0, 0, time.UTC),
	TradeCount: 1234,
	VWAP:       100.123456,
}

var testTradingStatus = tradingStatusWithT{
	Type:       "s",
	Symbol:     "BIIB",
	StatusCode: "T",
	StatusMsg:  "Trading Resumption",
	ReasonCode: "LUDP",
	ReasonMsg:  "Volatility Trading Pause",
	Timestamp:  time.Date(2021, 03, 05, 16, 0, 0, 0, time.UTC),
	Tape:       "C",
}

var testCryptoTrade = cryptoTradeWithT{
	Type:      "t",
	Symbol:    "A",
	Price:     100,
	Size:      10.1,
	Timestamp: testTime,
}

var testCryptoQuote = cryptoQuoteWithT{
	Type:      "q",
	Symbol:    "TEST",
	BidPrice:  99.9,
	AskPrice:  100.1,
	Timestamp: testTime,
}

var testCryptoBar = cryptoBarWithT{
	Type:       "b",
	Symbol:     "TEST",
	Open:       100,
	High:       101.2,
	Low:        98.67,
	Close:      101.1,
	Volume:     2560,
	Timestamp:  time.Date(2021, 03, 05, 16, 0, 0, 0, time.UTC),
	TradeCount: 1234,
	VWAP:       100.123456,
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
	Type:      "subscription",
	Trades:    []string{"ALPACA"},
	Quotes:    []string{"AL", "PACA"},
	Bars:      []string{"ALP", "ACA"},
	DailyBars: []string{"LPACA"},
}

func TestHandleMessagesStocks(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{
		testOther,
		testTrade,
		testTradingStatus,
		testQuote,
		testBar,
		testError,
		testSubMessage1,
		testSubMessage2,
	})
	require.NoError(t, err)

	emh := errMessageHandler
	smh := subMessageHandler
	defer func() {
		errMessageHandler = emh
		subMessageHandler = smh
	}()

	subscriptionMessages := make([]subscriptions, 0)

	var em errorMessage
	errMessageHandler = func(c *client, e errorMessage) error {
		em = e
		return nil
	}
	subMessageHandler = func(c *client, s subscriptions) error {
		subscriptionMessages = append(subscriptionMessages, s)
		return nil
	}

	h := &stocksMsgHandler{}
	c := &client{
		handler: h,
	}
	var trade Trade
	h.tradeHandler = func(t Trade) {
		trade = t
	}
	var quote Quote
	h.quoteHandler = func(q Quote) {
		quote = q
	}
	var bar Bar
	h.barHandler = func(b Bar) {
		bar = b
	}
	var tradingStatus TradingStatus
	h.tradingStatusHandler = func(ts TradingStatus) {
		tradingStatus = ts
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

	assert.Equal(t, testTradingStatus.Symbol, tradingStatus.Symbol)
	assert.Equal(t, testTradingStatus.StatusCode, tradingStatus.StatusCode)
	assert.Equal(t, testTradingStatus.StatusMsg, tradingStatus.StatusMsg)
	assert.Equal(t, testTradingStatus.ReasonCode, tradingStatus.ReasonCode)
	assert.Equal(t, testTradingStatus.ReasonMsg, tradingStatus.ReasonMsg)
	assert.True(t, testTradingStatus.Timestamp.Equal(tradingStatus.Timestamp))
	assert.Equal(t, testTradingStatus.Tape, tradingStatus.Tape)

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
	assert.EqualValues(t, testBar.TradeCount, bar.TradeCount)
	assert.EqualValues(t, testBar.VWAP, bar.VWAP)

	assert.EqualValues(t, testError.Code, em.code)
	assert.EqualValues(t, testError.Msg, em.msg)

	require.Len(t, subscriptionMessages, 2)
	assert.EqualValues(t, testSubMessage1.Trades, subscriptionMessages[0].trades)
	assert.EqualValues(t, testSubMessage1.Quotes, subscriptionMessages[0].quotes)
	assert.EqualValues(t, testSubMessage1.Bars, subscriptionMessages[0].bars)
	assert.EqualValues(t, testSubMessage2.Trades, subscriptionMessages[1].trades)
	assert.EqualValues(t, testSubMessage2.Quotes, subscriptionMessages[1].quotes)
	assert.EqualValues(t, testSubMessage2.Bars, subscriptionMessages[1].bars)
	assert.EqualValues(t, testSubMessage2.DailyBars, subscriptionMessages[1].dailyBars)
}

func TestHandleMessagesCrypto(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{
		testOther,
		testCryptoTrade,
		testCryptoQuote,
		testCryptoBar,
		testError,
		testSubMessage1,
		testSubMessage2,
	})
	require.NoError(t, err)

	emh := errMessageHandler
	smh := subMessageHandler
	defer func() {
		errMessageHandler = emh
		subMessageHandler = smh
	}()

	subscriptionMessages := make([]subscriptions, 0)

	var em errorMessage
	errMessageHandler = func(c *client, e errorMessage) error {
		em = e
		return nil
	}
	subMessageHandler = func(c *client, s subscriptions) error {
		subscriptionMessages = append(subscriptionMessages, s)
		return nil
	}

	h := &cryptoMsgHandler{}
	c := &client{
		handler: h,
	}
	var trade CryptoTrade
	h.tradeHandler = func(t CryptoTrade) {
		trade = t
	}
	var quote CryptoQuote
	h.quoteHandler = func(q CryptoQuote) {
		quote = q
	}
	var bar CryptoBar
	h.barHandler = func(b CryptoBar) {
		bar = b
	}

	err = c.handleMessage(b)
	require.NoError(t, err)

	assert.EqualValues(t, testCryptoTrade.Symbol, trade.Symbol)
	assert.EqualValues(t, testCryptoTrade.Price, trade.Price)
	assert.EqualValues(t, testCryptoTrade.Size, trade.Size)
	assert.True(t, trade.Timestamp.Equal(testTime))

	assert.EqualValues(t, testCryptoQuote.Symbol, quote.Symbol)
	assert.EqualValues(t, testCryptoQuote.BidPrice, quote.BidPrice)
	assert.EqualValues(t, testCryptoQuote.AskPrice, quote.AskPrice)
	assert.True(t, quote.Timestamp.Equal(testTime))

	assert.EqualValues(t, testCryptoBar.Symbol, bar.Symbol)
	assert.EqualValues(t, testCryptoBar.Open, bar.Open)
	assert.EqualValues(t, testCryptoBar.High, bar.High)
	assert.EqualValues(t, testCryptoBar.Low, bar.Low)
	assert.EqualValues(t, testCryptoBar.Close, bar.Close)
	assert.EqualValues(t, testCryptoBar.Volume, bar.Volume)
	assert.EqualValues(t, testCryptoBar.TradeCount, bar.TradeCount)
	assert.EqualValues(t, testCryptoBar.VWAP, bar.VWAP)

	assert.EqualValues(t, testError.Code, em.code)
	assert.EqualValues(t, testError.Msg, em.msg)

	require.Len(t, subscriptionMessages, 2)
	assert.EqualValues(t, testSubMessage1.Trades, subscriptionMessages[0].trades)
	assert.EqualValues(t, testSubMessage1.Quotes, subscriptionMessages[0].quotes)
	assert.EqualValues(t, testSubMessage1.Bars, subscriptionMessages[0].bars)
	assert.EqualValues(t, testSubMessage2.Trades, subscriptionMessages[1].trades)
	assert.EqualValues(t, testSubMessage2.Quotes, subscriptionMessages[1].quotes)
	assert.EqualValues(t, testSubMessage2.Bars, subscriptionMessages[1].bars)
	assert.EqualValues(t, testSubMessage2.DailyBars, subscriptionMessages[1].dailyBars)
}

func BenchmarkHandleMessages(b *testing.B) {
	msgs, _ := msgpack.Marshal([]interface{}{testTrade, testQuote, testBar})
	c := &client{
		handler: &stocksMsgHandler{
			tradeHandler: func(trade Trade) {},
			quoteHandler: func(quote Quote) {},
			barHandler:   func(bar Bar) {},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.handleMessage(msgs)
	}
}
