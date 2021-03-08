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

func TestHandleMessages(t *testing.T) {
	b, err := msgpack.Marshal([]interface{}{testOther, testTrade, testQuote, testBar})
	require.NoError(t, err)

	s := &datav2stream{}
	var trade Trade
	s.tradeHandlers = map[string]func(trade Trade){
		"TEST": func(got Trade) {
			trade = got
		},
	}
	var quote Quote
	s.quoteHandlers = map[string]func(quote Quote){
		"TEST": func(got Quote) {
			quote = got
		},
	}
	var bar Bar
	s.barHandlers = map[string]func(bar Bar){
		"TEST": func(got Bar) {
			bar = got
		},
	}

	err = s.handleMessages(b)
	require.NoError(t, err)

	assert.EqualValues(t, 42, trade.ID)
	assert.EqualValues(t, "TEST", trade.Symbol)
	assert.EqualValues(t, "X", trade.Exchange)
	assert.EqualValues(t, 100, trade.Price)
	assert.EqualValues(t, 10, trade.Size)
	assert.True(t, trade.Timestamp.Equal(testTime))
	assert.EqualValues(t, []string{" "}, trade.Conditions)
	assert.EqualValues(t, "A", trade.Tape)

	assert.EqualValues(t, "TEST", quote.Symbol)
	assert.EqualValues(t, "B", quote.BidExchange)
	assert.EqualValues(t, 99.9, quote.BidPrice)
	assert.EqualValues(t, 100, quote.BidSize)
	assert.EqualValues(t, "A", quote.AskExchange)
	assert.EqualValues(t, 100.1, quote.AskPrice)
	assert.EqualValues(t, 200, quote.AskSize)
	assert.True(t, quote.Timestamp.Equal(testTime))
	assert.EqualValues(t, []string{"R"}, quote.Conditions)
	assert.EqualValues(t, "B", quote.Tape)

	assert.EqualValues(t, "TEST", bar.Symbol)
	assert.EqualValues(t, 100, bar.Open)
	assert.EqualValues(t, 101.2, bar.High)
	assert.EqualValues(t, 98.67, bar.Low)
	assert.EqualValues(t, 101.1, bar.Close)
	assert.EqualValues(t, 2560, bar.Volume)
}

func BenchmarkHandleMessages(b *testing.B) {
	msgs, _ := msgpack.Marshal([]interface{}{testTrade, testQuote, testBar})
	s := &datav2stream{
		tradeHandlers: map[string]func(trade Trade){
			"*": func(trade Trade) {},
		},
		quoteHandlers: map[string]func(quote Quote){
			"*": func(quote Quote) {},
		},
		barHandlers: map[string]func(bar Bar){
			"*": func(bar Bar) {},
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s.handleMessages(msgs)
	}
}
