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
	ReceivedAt time.Time `msgpack:"r"`
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
	ReceivedAt  time.Time `msgpack:"r"`
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

// tradingStatusWithT is the incoming trading status message that also contains the T type key
type imbalanceWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Price     float64   `msgpack:"p"`
	Timestamp time.Time `msgpack:"t"`
	Tape      string    `msgpack:"z"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// luldWithT is the incoming LULD message that also contains the T type key
type luldWithT struct {
	Type           string    `json:"T" msgpack:"T"`
	Symbol         string    `json:"S" msgpack:"S"`
	LimitUpPrice   float64   `json:"u" msgpack:"u"`
	LimitDownPrice float64   `json:"d" msgpack:"d"`
	Indicator      string    `json:"i" msgpack:"i"`
	Timestamp      time.Time `json:"t" msgpack:"t"`
	Tape           string    `json:"z" msgpack:"z"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// tradeCancelErrorWithT is the incoming cancel error message that also contains the T type key
type tradeCancelErrorWithT struct {
	Type              string    `json:"T" msgpack:"T"`
	Symbol            string    `json:"S" msgpack:"S"`
	ID                int64     `json:"i" msgpack:"i"`
	Exchange          string    `json:"x" msgpack:"x"`
	Price             float64   `json:"p" msgpack:"p"`
	Size              uint32    `json:"s" msgpack:"s"`
	CancelErrorAction string    `json:"a" msgpack:"a"`
	Tape              string    `json:"z" msgpack:"z"`
	Timestamp         time.Time `json:"t" msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// tradeCorrectionWithT is the incoming cancel error message that also contains the T type key
type tradeCorrectionWithT struct {
	Type                string    `json:"T"  msgpack:"T"`
	Symbol              string    `json:"S"  msgpack:"S"`
	Exchange            string    `json:"x"  msgpack:"x"`
	OriginalID          int64     `json:"oi" msgpack:"oi"`
	OriginalPrice       float64   `json:"op" msgpack:"op"`
	OriginalSize        uint32    `json:"os" msgpack:"os"`
	OriginalConditions  []string  `json:"oc" msgpack:"oc"`
	CorrectedID         int64     `json:"ci" msgpack:"ci"`
	CorrectedPrice      float64   `json:"cp" msgpack:"cp"`
	CorrectedSize       uint32    `json:"cs" msgpack:"cs"`
	CorrectedConditions []string  `json:"cc" msgpack:"cc"`
	Tape                string    `json:"z"  msgpack:"z"`
	Timestamp           time.Time `json:"t"  msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoTradeWithT is the incoming crypto trade message that also contains the T type key
type cryptoTradeWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Exchange  string    `msgpack:"x"`
	Price     float64   `msgpack:"p"`
	Size      float64   `msgpack:"s"`
	Timestamp time.Time `msgpack:"t"`
	ID        int64     `msgpack:"i"`
	TakerSide string    `msgpack:"tks"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoQuoteWithT is the incoming crypto quote message that also contains the T type key
type cryptoQuoteWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Exchange  string    `msgpack:"x"`
	BidPrice  float64   `msgpack:"bp"`
	BidSize   float64   `msgpack:"bs"`
	AskPrice  float64   `msgpack:"ap"`
	AskSize   float64   `msgpack:"as"`
	Timestamp time.Time `msgpack:"t"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoBarWithT is the incoming crypto bar message that also contains the T type key
type cryptoBarWithT struct {
	Type       string    `msgpack:"T"`
	Symbol     string    `msgpack:"S"`
	Exchange   string    `msgpack:"x"`
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

// cryptoOrderbookWithT is the incoming crypto orderbook message that also contains the T type key
type cryptoOrderbookWithT struct {
	Type      string                 `msgpack:"T"`
	Symbol    string                 `msgpack:"S"`
	Exchange  string                 `msgpack:"x"`
	Timestamp time.Time              `msgpack:"t"`
	Bids      []cryptoOrderbookEntry `msgpack:"b"`
	Asks      []cryptoOrderbookEntry `msgpack:"a"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"n"`
}

// cryptoOrderbookEntry is the incoming crypto orderbook entry message
type cryptoOrderbookEntry struct {
	Price float64 `msgpack:"p"`
	Size  float64 `msgpack:"s"`
}

// optionTradeWithT is the incoming option trade message that also contains the T type key
type optionTradeWithT struct {
	Type      string    `msgpack:"T"`
	Symbol    string    `msgpack:"S"`
	Exchange  string    `msgpack:"x"`
	Price     float64   `msgpack:"p"`
	Size      uint32    `msgpack:"s"`
	Timestamp time.Time `msgpack:"t"`
	Condition string    `msgpack:"c"`
}

// optionQuoteWithT is the incoming option quote message that also contains the T type key
type optionQuoteWithT struct {
	Type        string    `msgpack:"T"`
	Symbol      string    `msgpack:"S"`
	BidExchange string    `msgpack:"bx"`
	BidPrice    float64   `msgpack:"bp"`
	BidSize     uint32    `msgpack:"bs"`
	AskExchange string    `msgpack:"ax"`
	AskPrice    float64   `msgpack:"ap"`
	AskSize     uint32    `msgpack:"as"`
	Timestamp   time.Time `msgpack:"t"`
	Condition   string    `msgpack:"c"`
}

type newsWithT struct {
	Type      string    `msgpack:"T"`
	ID        int       `msgpack:"id"`
	Author    string    `msgpack:"author"`
	CreatedAt time.Time `msgpack:"created_at"`
	UpdatedAt time.Time `msgpack:"updated_at"`
	Headline  string    `msgpack:"headline"`
	Summary   string    `msgpack:"summary"`
	Content   string    `msgpack:"content"`
	URL       string    `msgpack:"url"`
	Symbols   []string  `msgpack:"symbols"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"new_field"`
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
	Type         string   `msgpack:"T"`
	Trades       []string `msgpack:"trades"`
	Quotes       []string `msgpack:"quotes"`
	Bars         []string `msgpack:"bars"`
	UpdatedBars  []string `msgpack:"updatedBars"`
	DailyBars    []string `msgpack:"dailyBars"`
	Statuses     []string `msgpack:"statuses"`
	Imbalances   []string `msgpack:"imbalances"`
	LULDs        []string `msgpack:"lulds"`
	Corrections  []string `msgpack:"corrections"`
	CancelErrors []string `msgpack:"cancelErrors"`
	Orderbooks   []string `msgpack:"orderbooks"`
	News         []string `msgpack:"news"`
	// NewField is for testing correct handling of added fields in the future
	NewField uint64 `msgpack:"N"`
}

var (
	testTime  = time.Date(2021, 3, 4, 15, 16, 17, 18, time.UTC)
	testTime2 = time.Date(2021, 3, 4, 15, 16, 17, 165123789, time.UTC)
)

var testTrade = tradeWithT{
	Type:       "t",
	ID:         42,
	Symbol:     "TEST",
	Exchange:   "X",
	Price:      100,
	Size:       10,
	Timestamp:  testTime,
	ReceivedAt: testTime2,
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
	ReceivedAt:  testTime2,
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
	Timestamp:  time.Date(2021, 3, 5, 16, 0, 0, 0, time.UTC),
	TradeCount: 1234,
	VWAP:       100.123456,
}

var testUpdatedBar = barWithT{
	Type:       "u",
	Symbol:     "TEST",
	Open:       100,
	High:       101.2,
	Low:        98.67,
	Close:      101.3,
	Volume:     2570,
	Timestamp:  time.Date(2021, 3, 5, 16, 0, 30, 0, time.UTC),
	TradeCount: 1235,
	VWAP:       100.123457,
}

var testTradingStatus = tradingStatusWithT{
	Type:       "s",
	Symbol:     "BIIB",
	StatusCode: "T",
	StatusMsg:  "Trading Resumption",
	ReasonCode: "LUDP",
	ReasonMsg:  "Volatility Trading Pause",
	Timestamp:  time.Date(2021, 3, 5, 16, 0, 0, 0, time.UTC),
	Tape:       "C",
}

var testImbalance = imbalanceWithT{
	Type:      "i",
	Symbol:    "BIIB",
	Price:     100.2,
	Timestamp: time.Date(2021, 3, 5, 16, 0, 0, 0, time.UTC),
	Tape:      "C",
}

var testLULD = luldWithT{
	Type:           "l",
	Symbol:         "TEST",
	LimitUpPrice:   4.21,
	LimitDownPrice: 3.22,
	Indicator:      "B",
	Timestamp:      time.Date(2021, 7, 5, 13, 32, 0, 0, time.UTC),
	Tape:           "C",
}

var testCancelError = tradeCancelErrorWithT{
	Type:              "x",
	Symbol:            "TEST",
	ID:                123,
	Exchange:          "X",
	Price:             100,
	Size:              10,
	CancelErrorAction: "X",
	Tape:              "C",
	Timestamp:         time.Date(2021, 12, 7, 13, 32, 0, 0, time.UTC),
}

var testCorrection = tradeCorrectionWithT{
	Type:                "c",
	Symbol:              "TEST",
	Exchange:            "X",
	OriginalID:          123,
	OriginalPrice:       123.123,
	OriginalSize:        123,
	OriginalConditions:  []string{" ", "7", "V"},
	CorrectedID:         124,
	CorrectedPrice:      124.124,
	CorrectedSize:       124,
	CorrectedConditions: []string{" ", "7", "Z", "V"},
	Tape:                "C",
	Timestamp:           time.Date(2021, 12, 7, 13, 32, 0, 0, time.UTC),
}

var testCryptoTrade = cryptoTradeWithT{
	Type:      "t",
	Symbol:    "A",
	Exchange:  "ERSX",
	Price:     100,
	Size:      10.1,
	Timestamp: testTime,
	ID:        32,
	TakerSide: "B",
}

var testCryptoQuote = cryptoQuoteWithT{
	Type:      "q",
	Symbol:    "TEST",
	Exchange:  "ERSX",
	BidPrice:  99.9,
	BidSize:   1.0945,
	AskPrice:  100.1,
	AskSize:   2.41231,
	Timestamp: testTime,
}

var testCryptoBar = cryptoBarWithT{
	Type:       "b",
	Symbol:     "TEST",
	Exchange:   "TEST",
	Open:       100,
	High:       101.2,
	Low:        98.67,
	Close:      101.1,
	Volume:     2560,
	Timestamp:  time.Date(2021, 3, 5, 16, 0, 0, 0, time.UTC),
	TradeCount: 1234,
	VWAP:       100.123456,
}

var testUpdatedCryptoBar = cryptoBarWithT{
	Type:       "u",
	Symbol:     "TEST",
	Exchange:   "TEST",
	Open:       100,
	High:       101.2,
	Low:        98.67,
	Close:      102.78,
	Volume:     2585,
	Timestamp:  time.Date(2021, 3, 5, 16, 0, 30, 0, time.UTC),
	TradeCount: 1236,
	VWAP:       100.123487,
}

var testCryptoOrderbook = cryptoOrderbookWithT{
	Type:      "o",
	Symbol:    "TEST",
	Exchange:  "TEST",
	Timestamp: time.Date(2022, 4, 4, 16, 0, 30, 0, time.UTC),
	Bids: []cryptoOrderbookEntry{
		{Price: 111.1, Size: 222.2},
		{Price: 333.3, Size: 444.4},
	},
	Asks: []cryptoOrderbookEntry{
		{Price: 555.5, Size: 666.6},
		{Price: 777.7, Size: 888.8},
	},
}

type cryptoPerpPricingWithT struct {
	Type            string    `msgpack:"T"`
	Symbol          string    `msgpack:"S"`
	Timestamp       time.Time `msgpack:"t"`
	Exchange        string    `msgpack:"x"`
	IndexPrice      float64   `msgpack:"ip"`
	MarkPrice       float64   `msgpack:"mp"`
	FundingRate     float64   `msgpack:"fr"`
	OpenInterest    float64   `msgpack:"oi"`
	NextFundingTime time.Time `msgpack:"ft"`
}

var testOther = other{
	Type:     "other",
	Whatever: "whatever",
}

var testError = errorWithT{
	Type: msgTypeError,
	Msg:  "test",
	Code: 322,
}

var testSubMessage1 = subWithT{
	Type:         "subscription",
	Trades:       []string{"ALPACA"},
	Quotes:       []string{},
	Bars:         []string{},
	Statuses:     []string{"ALPACA"},
	Imbalances:   []string{"ALPACA"},
	CancelErrors: []string{"ALPACA"},
	Corrections:  []string{"ALPACA"},
}

var testSubMessage2 = subWithT{
	Type:         "subscription",
	Trades:       []string{"ALPACA"},
	Quotes:       []string{"AL", "PACA"},
	Bars:         []string{"ALP", "ACA"},
	DailyBars:    []string{"LPACA"},
	Statuses:     []string{"AL", "PACA"},
	Imbalances:   []string{"AL", "PACA"},
	CancelErrors: []string{"ALPACA"},
	Corrections:  []string{"ALPACA"},
}

func TestHandleMessagesStocks(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{
		testOther,
		testTrade,
		testTradingStatus,
		testImbalance,
		testQuote,
		testBar,
		testUpdatedBar,
		testError,
		testSubMessage1,
		testSubMessage2,
		testLULD,
		testCancelError,
		testCorrection,
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
	errMessageHandler = func(_ *client, e errorMessage) error {
		em = e
		return nil
	}
	subMessageHandler = func(_ *client, s subscriptions) error {
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
	var updatedBar Bar
	h.updatedBarHandler = func(b Bar) {
		updatedBar = b
	}
	var tradingStatus TradingStatus
	h.tradingStatusHandler = func(ts TradingStatus) {
		tradingStatus = ts
	}
	var imbalance Imbalance
	h.imbalanceHandler = func(oi Imbalance) {
		imbalance = oi
	}
	var luld LULD
	h.luldHandler = func(l LULD) {
		luld = l
	}
	var cancelError TradeCancelError
	h.cancelErrorHandler = func(tce TradeCancelError) {
		cancelError = tce
	}
	var correction TradeCorrection
	h.correctionHandler = func(tc TradeCorrection) {
		correction = tc
	}

	err = c.handleMessage(b)
	require.NoError(t, err)

	// Verify stock trade.
	assert.EqualValues(t, testTrade.ID, trade.ID)
	assert.EqualValues(t, testTrade.Symbol, trade.Symbol)
	assert.EqualValues(t, testTrade.Exchange, trade.Exchange)
	assert.EqualValues(t, testTrade.Price, trade.Price)
	assert.EqualValues(t, testTrade.Size, trade.Size)
	assert.True(t, trade.Timestamp.Equal(testTime))
	assert.True(t, trade.Internal().ReceivedAt.Equal(testTime2))
	assert.EqualValues(t, testTrade.Conditions, trade.Conditions)
	assert.EqualValues(t, testTrade.Tape, trade.Tape)

	// Verify stock trading status.
	assert.Equal(t, testTradingStatus.Symbol, tradingStatus.Symbol)
	assert.Equal(t, testTradingStatus.StatusCode, tradingStatus.StatusCode)
	assert.Equal(t, testTradingStatus.StatusMsg, tradingStatus.StatusMsg)
	assert.Equal(t, testTradingStatus.ReasonCode, tradingStatus.ReasonCode)
	assert.Equal(t, testTradingStatus.ReasonMsg, tradingStatus.ReasonMsg)
	assert.True(t, testTradingStatus.Timestamp.Equal(tradingStatus.Timestamp))
	assert.True(t, quote.Internal().ReceivedAt.Equal(testTime2))
	assert.Equal(t, testTradingStatus.Tape, tradingStatus.Tape)

	// Verify stock imbalance.
	assert.Equal(t, testImbalance.Symbol, imbalance.Symbol)
	assert.Equal(t, testImbalance.Price, imbalance.Price)
	assert.True(t, testImbalance.Timestamp.Equal(imbalance.Timestamp))
	assert.Equal(t, testImbalance.Tape, imbalance.Tape)

	// Verify stock luld.
	assert.Equal(t, testLULD.Symbol, luld.Symbol)
	assert.EqualValues(t, testLULD.LimitUpPrice, luld.LimitUpPrice)
	assert.EqualValues(t, testLULD.LimitDownPrice, luld.LimitDownPrice)
	assert.Equal(t, testLULD.Indicator, luld.Indicator)
	assert.True(t, luld.Timestamp.Equal(testLULD.Timestamp))
	assert.Equal(t, testLULD.Tape, luld.Tape)

	// Verify stock trade correction.
	assert.EqualValues(t, testCorrection.Symbol, correction.Symbol)
	assert.EqualValues(t, testCorrection.Exchange, correction.Exchange)
	assert.EqualValues(t, testCorrection.OriginalID, correction.OriginalID)
	assert.EqualValues(t, testCorrection.OriginalPrice, correction.OriginalPrice)
	assert.EqualValues(t, testCorrection.OriginalSize, correction.OriginalSize)
	assert.EqualValues(t, testCorrection.OriginalConditions, correction.OriginalConditions)
	assert.EqualValues(t, testCorrection.CorrectedID, correction.CorrectedID)
	assert.EqualValues(t, testCorrection.CorrectedPrice, correction.CorrectedPrice)
	assert.EqualValues(t, testCorrection.CorrectedSize, correction.CorrectedSize)
	assert.EqualValues(t, testCorrection.CorrectedConditions, correction.CorrectedConditions)
	assert.EqualValues(t, testCorrection.Tape, correction.Tape)
	assert.True(t, testCorrection.Timestamp.Equal(correction.Timestamp))

	// Verify stock trade cancel error.
	assert.EqualValues(t, testCancelError.Symbol, cancelError.Symbol)
	assert.EqualValues(t, testCancelError.ID, cancelError.ID)
	assert.EqualValues(t, testCancelError.Exchange, cancelError.Exchange)
	assert.EqualValues(t, testCancelError.Price, cancelError.Price)
	assert.EqualValues(t, testCancelError.Size, cancelError.Size)
	assert.EqualValues(t, testCancelError.CancelErrorAction, cancelError.CancelErrorAction)
	assert.EqualValues(t, testCancelError.Tape, cancelError.Tape)
	assert.True(t, testCancelError.Timestamp.Equal(cancelError.Timestamp))

	// Verify stock quote.
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

	// Verify stock bar.
	assert.EqualValues(t, testBar.Symbol, bar.Symbol)
	assert.EqualValues(t, testBar.Open, bar.Open)
	assert.EqualValues(t, testBar.High, bar.High)
	assert.EqualValues(t, testBar.Low, bar.Low)
	assert.EqualValues(t, testBar.Close, bar.Close)
	assert.EqualValues(t, testBar.Volume, bar.Volume)
	assert.EqualValues(t, testBar.TradeCount, bar.TradeCount)
	assert.EqualValues(t, testBar.VWAP, bar.VWAP)

	// Verify stock updated bar.
	assert.EqualValues(t, testUpdatedBar.Symbol, updatedBar.Symbol)
	assert.EqualValues(t, testUpdatedBar.Open, updatedBar.Open)
	assert.EqualValues(t, testUpdatedBar.High, updatedBar.High)
	assert.EqualValues(t, testUpdatedBar.Low, updatedBar.Low)
	assert.EqualValues(t, testUpdatedBar.Close, updatedBar.Close)
	assert.EqualValues(t, testUpdatedBar.Volume, updatedBar.Volume)
	assert.EqualValues(t, testUpdatedBar.TradeCount, updatedBar.TradeCount)
	assert.EqualValues(t, testUpdatedBar.VWAP, updatedBar.VWAP)

	// Verify error.
	assert.EqualValues(t, testError.Code, em.code)
	assert.EqualValues(t, testError.Msg, em.msg)

	// Verify subscription.
	require.Len(t, subscriptionMessages, 2)
	assert.EqualValues(t, testSubMessage1.Trades, subscriptionMessages[0].trades)
	assert.EqualValues(t, testSubMessage1.Quotes, subscriptionMessages[0].quotes)
	assert.EqualValues(t, testSubMessage1.Bars, subscriptionMessages[0].bars)
	assert.EqualValues(t, testSubMessage1.CancelErrors, subscriptionMessages[0].cancelErrors)
	assert.EqualValues(t, testSubMessage1.Corrections, subscriptionMessages[0].corrections)
	assert.EqualValues(t, testSubMessage1.Imbalances, subscriptionMessages[0].imbalances)
	assert.EqualValues(t, testSubMessage2.Trades, subscriptionMessages[1].trades)
	assert.EqualValues(t, testSubMessage2.Quotes, subscriptionMessages[1].quotes)
	assert.EqualValues(t, testSubMessage2.Bars, subscriptionMessages[1].bars)
	assert.EqualValues(t, testSubMessage2.DailyBars, subscriptionMessages[1].dailyBars)
	assert.EqualValues(t, testSubMessage2.CancelErrors, subscriptionMessages[1].cancelErrors)
	assert.EqualValues(t, testSubMessage2.Corrections, subscriptionMessages[1].corrections)
	assert.EqualValues(t, testSubMessage2.Imbalances, subscriptionMessages[1].imbalances)
}

func TestHandleMessagesCrypto(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{
		testOther,
		testCryptoTrade,
		testLULD,
		testTradingStatus,
		testCryptoQuote,
		testCryptoBar,
		testUpdatedCryptoBar,
		testCryptoOrderbook,
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
	errMessageHandler = func(_ *client, e errorMessage) error {
		em = e
		return nil
	}
	subMessageHandler = func(_ *client, s subscriptions) error {
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
	var updatedBar CryptoBar
	h.updatedBarHandler = func(b CryptoBar) {
		updatedBar = b
	}
	var orderbook CryptoOrderbook
	h.orderbookHandler = func(ob CryptoOrderbook) {
		orderbook = ob
	}

	err = c.handleMessage(b)
	require.NoError(t, err)

	// Verify crypto trade.
	assert.EqualValues(t, testCryptoTrade.Symbol, trade.Symbol)
	assert.EqualValues(t, testCryptoTrade.Exchange, trade.Exchange)
	assert.EqualValues(t, testCryptoTrade.Price, trade.Price)
	assert.EqualValues(t, testCryptoTrade.Size, trade.Size)
	assert.EqualValues(t, testCryptoTrade.ID, trade.ID)
	assert.EqualValues(t, testCryptoTrade.TakerSide, trade.TakerSide)
	assert.True(t, trade.Timestamp.Equal(testTime))

	// Verify crypto quote.
	assert.EqualValues(t, testCryptoQuote.Symbol, quote.Symbol)
	assert.EqualValues(t, testCryptoQuote.Exchange, quote.Exchange)
	assert.EqualValues(t, testCryptoQuote.BidPrice, quote.BidPrice)
	assert.EqualValues(t, testCryptoQuote.BidSize, quote.BidSize)
	assert.EqualValues(t, testCryptoQuote.AskPrice, quote.AskPrice)
	assert.EqualValues(t, testCryptoQuote.AskSize, quote.AskSize)
	assert.True(t, quote.Timestamp.Equal(testTime))

	// Verify crypto bar.
	assert.EqualValues(t, testCryptoBar.Symbol, bar.Symbol)
	assert.EqualValues(t, testCryptoBar.Exchange, bar.Exchange)
	assert.EqualValues(t, testCryptoBar.Open, bar.Open)
	assert.EqualValues(t, testCryptoBar.High, bar.High)
	assert.EqualValues(t, testCryptoBar.Low, bar.Low)
	assert.EqualValues(t, testCryptoBar.Close, bar.Close)
	assert.EqualValues(t, testCryptoBar.Volume, bar.Volume)
	assert.EqualValues(t, testCryptoBar.TradeCount, bar.TradeCount)
	assert.EqualValues(t, testCryptoBar.VWAP, bar.VWAP)

	// Verify crypto updated bar.
	assert.EqualValues(t, testUpdatedCryptoBar.Symbol, updatedBar.Symbol)
	assert.EqualValues(t, testUpdatedCryptoBar.Exchange, updatedBar.Exchange)
	assert.EqualValues(t, testUpdatedCryptoBar.Open, updatedBar.Open)
	assert.EqualValues(t, testUpdatedCryptoBar.High, updatedBar.High)
	assert.EqualValues(t, testUpdatedCryptoBar.Low, updatedBar.Low)
	assert.EqualValues(t, testUpdatedCryptoBar.Close, updatedBar.Close)
	assert.EqualValues(t, testUpdatedCryptoBar.Volume, updatedBar.Volume)
	assert.EqualValues(t, testUpdatedCryptoBar.TradeCount, updatedBar.TradeCount)
	assert.EqualValues(t, testUpdatedCryptoBar.VWAP, updatedBar.VWAP)

	// Verify crypto orderbook.
	assert.EqualValues(t, testCryptoOrderbook.Symbol, orderbook.Symbol)
	assert.EqualValues(t, testCryptoOrderbook.Exchange, orderbook.Exchange)
	for i := range testCryptoOrderbook.Bids {
		assert.EqualValues(t, testCryptoOrderbook.Bids[i].Price, orderbook.Bids[i].Price)
		assert.EqualValues(t, testCryptoOrderbook.Bids[i].Size, orderbook.Bids[i].Size)
	}
	for i := range testCryptoOrderbook.Asks {
		assert.EqualValues(t, testCryptoOrderbook.Asks[i].Price, orderbook.Asks[i].Price)
		assert.EqualValues(t, testCryptoOrderbook.Asks[i].Size, orderbook.Asks[i].Size)
	}

	// Verify error.
	assert.EqualValues(t, testError.Code, em.code)
	assert.EqualValues(t, testError.Msg, em.msg)

	// Verify subscription.
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
			tradeHandler: func(_ Trade) {},
			quoteHandler: func(_ Quote) {},
			barHandler:   func(_ Bar) {},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = c.handleMessage(msgs)
	}
}
