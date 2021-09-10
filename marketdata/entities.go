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

// CryptoTrade is a crypto trade
type CryptoTrade struct {
	Timestamp time.Time `json:"t"`
	Price     float64   `json:"p"`
	Size      float64   `json:"s"`
	Exchange  string    `json:"x"`
	ID        int64     `json:"i"`
	// TakerSide is the side of the taker of the trade. It is either "B" (buy), "S" (sell) or "-" (unspecified)
	TakerSide string `json:"tks"`
}

type CryptoTradeItem struct {
	Trade CryptoTrade
	Error error
}

// CryptoQuote is crypto quote
type CryptoQuote struct {
	Timestamp time.Time `json:"t"`
	Exchange  string    `json:"x"`
	BidPrice  float64   `json:"bp"`
	BidSize   float64   `json:"bs"`
	AskPrice  float64   `json:"ap"`
	AskSize   float64   `json:"as"`
}

// CryptoQuoteItem contains a single crypto quote or an error
type CryptoQuoteItem struct {
	Quote CryptoQuote
	Error error
}

// CryptoBar is an aggregate of crypto trades
type CryptoBar struct {
	Timestamp  time.Time `json:"t"`
	Exchange   string    `json:"x"`
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     float64   `json:"v"`
	TradeCount uint64    `json:"n"`
	VWAP       float64   `json:"vw"`
}

// CryptoBarItem contains a single crypto bar or an error
type CryptoBarItem struct {
	Bar   CryptoBar
	Error error
}

// CryptoMultiBarItem contains a single crypto bar for a symbol or an error
type CryptoMultiBarItem struct {
	Symbol string
	Bar    CryptoBar
	Error  error
}

// CryptoXBBO is a cross exchange crypto BBO
type CryptoXBBO struct {
	Timestamp   time.Time `json:"t"`
	BidExchange string    `json:"bx"`
	BidPrice    float64   `json:"bp"`
	BidSize     float64   `json:"bs"`
	AskExchange string    `json:"ax"`
	AskPrice    float64   `json:"ap"`
	AskSize     float64   `json:"as"`
}

type tradeResponse struct {
	Symbol        string  `json:"symbol"`
	NextPageToken *string `json:"next_page_token"`
	Trades        []Trade `json:"trades"`
}

type multiTradeResponse struct {
	NextPageToken *string            `json:"next_page_token"`
	Trades        map[string][]Trade `json:"trades"`
}

type quoteResponse struct {
	Symbol        string  `json:"symbol"`
	NextPageToken *string `json:"next_page_token"`
	Quotes        []Quote `json:"quotes"`
}

type multiQuoteResponse struct {
	NextPageToken *string            `json:"next_page_token"`
	Quotes        map[string][]Quote `json:"quotes"`
}

type barResponse struct {
	Symbol        string  `json:"symbol"`
	NextPageToken *string `json:"next_page_token"`
	Bars          []Bar   `json:"bars"`
}

type multiBarResponse struct {
	NextPageToken *string          `json:"next_page_token"`
	Bars          map[string][]Bar `json:"bars"`
}

type latestTradeResponse struct {
	Symbol string `json:"symbol"`
	Trade  Trade  `json:"trade"`
}

type latestQuoteResponse struct {
	Symbol string `json:"symbol"`
	Quote  Quote  `json:"quote"`
}

type cryptoTradeResponse struct {
	Symbol        string        `json:"symbol"`
	NextPageToken *string       `json:"next_page_token"`
	Trades        []CryptoTrade `json:"trades"`
}

type cryptoQuoteResponse struct {
	Symbol        string        `json:"symbol"`
	NextPageToken *string       `json:"next_page_token"`
	Quotes        []CryptoQuote `json:"quotes"`
}

type cryptoBarResponse struct {
	Symbol        string      `json:"symbol"`
	NextPageToken *string     `json:"next_page_token"`
	Bars          []CryptoBar `json:"bars"`
}

type cryptoMultiBarResponse struct {
	NextPageToken *string                `json:"next_page_token"`
	Bars          map[string][]CryptoBar `json:"bars"`
}

type latestCryptoTradeResponse struct {
	Symbol string      `json:"symbol"`
	Trade  CryptoTrade `json:"trade"`
}

type latestCryptoQuoteResponse struct {
	Symbol string      `json:"symbol"`
	Quote  CryptoQuote `json:"quote"`
}

type latestCryptoXBBOResponse struct {
	Symbol string     `json:"symbol"`
	XBBO   CryptoXBBO `json:"xbbo"`
}
