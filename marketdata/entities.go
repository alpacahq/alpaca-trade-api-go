package marketdata

import "time"

// Trade is a stock trade that happened on the market
type Trade struct {
	Timestamp  time.Time `json:"t"`
	Price      float64   `json:"p"`
	Size       uint32    `json:"s"`
	Exchange   string    `json:"x"`
	ID         int64     `json:"i"`
	Conditions []string  `json:"c"`
	Tape       string    `json:"z"`
}

// TradeItem contains a single trade or an error
type TradeItem struct {
	Trade Trade
	Error error
}

// MultiTradeItem contains a single trade for a symbol or an error
type MultiTradeItem struct {
	Symbol string
	Trade  Trade
	Error  error
}

// Quote is a stock quote from the market
type Quote struct {
	Timestamp   time.Time `json:"t"`
	BidPrice    float64   `json:"bp"`
	BidSize     uint32    `json:"bs"`
	BidExchange string    `json:"bx"`
	AskPrice    float64   `json:"ap"`
	AskSize     uint32    `json:"as"`
	AskExchange string    `json:"ax"`
	Conditions  []string  `json:"c"`
	Tape        string    `json:"z"`
}

// QuoteItem contains a single quote or an error
type QuoteItem struct {
	Quote Quote
	Error error
}

// MultiQuoteItem contains a single quote for a symbol or an error
type MultiQuoteItem struct {
	Symbol string
	Quote  Quote
	Error  error
}

// TimeFrame is the resolution of the bars
type TimeFrame string

// List of time frames
const (
	Min  TimeFrame = "1Min"
	Hour TimeFrame = "1Hour"
	Day  TimeFrame = "1Day"
)

// Adjustment specifies the corporate action adjustment(s) for the bars
type Adjustment string

// List of adjustments
const (
	Raw      Adjustment = "raw"
	Split    Adjustment = "split"
	Dividend Adjustment = "dividend"
	All      Adjustment = "all"
)

// Bar is an aggregate of trades
type Bar struct {
	Timestamp  time.Time `json:"t"`
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     uint64    `json:"v"`
	TradeCount uint64    `json:"n"`
	VWAP       float64   `json:"vw"`
}

// BarItem contains a single bar or an error
type BarItem struct {
	Bar   Bar
	Error error
}

// MultiBarItem contains a single bar for a symbol or an error
type MultiBarItem struct {
	Symbol string
	Bar    Bar
	Error  error
}

// Snapshot is a snapshot of a symbol
type Snapshot struct {
	LatestTrade  *Trade `json:"latestTrade"`
	LatestQuote  *Quote `json:"latestQuote"`
	MinuteBar    *Bar   `json:"minuteBar"`
	DailyBar     *Bar   `json:"dailyBar"`
	PrevDailyBar *Bar   `json:"prevDailyBar"`
}
