package marketdata

import (
	"fmt"
	"time"

	"cloud.google.com/go/civil"

	// Required for easyjson generation
	_ "github.com/mailru/easyjson/gen"
)

//go:generate go install github.com/mailru/easyjson/...@v0.7.7
//go:generate easyjson -all -lower_camel_case $GOFILE

// Feed defines the source feed of stock data.
type Feed = string

const (
	// SIP includes all US exchanges.
	SIP Feed = "sip"
	// IEX only includes the Investors Exchange.
	IEX Feed = "iex"
	// OTC includes Over-the-Counter exchanges
	OTC Feed = "otc"
)

// CryptoFeed defines the source feed of crypto data.
type CryptoFeed = string

const (
	// US is the crypto feed for the United States.
	US CryptoFeed = "us"
)

// TakerSide is the taker's side: one of B, S or -. B is buy, S is sell and - is unknown.
type TakerSide = string

const (
	TakerSideBuy     TakerSide = "B"
	TakerSideSell    TakerSide = "S"
	TakerSideUnknown TakerSide = "-"
)

// Trade is a stock trade that happened on the market
type Trade struct {
	Timestamp  time.Time `json:"t"`
	Price      float64   `json:"p"`
	Size       uint32    `json:"s"`
	Exchange   string    `json:"x"`
	ID         int64     `json:"i"`
	Conditions []string  `json:"c"`
	Tape       string    `json:"z"`
	Update     string    `json:"u"`
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

// TimeFrameUnite is the base unit of the timeframe.
type TimeFrameUnit string

// List of timeframe units
const (
	Min   TimeFrameUnit = "Min"
	Hour  TimeFrameUnit = "Hour"
	Day   TimeFrameUnit = "Day"
	Week  TimeFrameUnit = "Week"
	Month TimeFrameUnit = "Month"
)

// TimeFrame is the resolution of the bars
type TimeFrame struct {
	N    int
	Unit TimeFrameUnit
}

func NewTimeFrame(n int, unit TimeFrameUnit) TimeFrame {
	return TimeFrame{
		N:    n,
		Unit: unit,
	}
}

func (tf TimeFrame) String() string {
	return fmt.Sprintf("%d%s", tf.N, tf.Unit)
}

var (
	OneMin   = NewTimeFrame(1, Min)
	OneHour  = NewTimeFrame(1, Hour)
	OneDay   = NewTimeFrame(1, Day)
	OneWeek  = NewTimeFrame(1, Week)
	OneMonth = NewTimeFrame(1, Month)
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

// Auction is a special trade that represents a stock auction
type Auction struct {
	Timestamp time.Time `json:"t"`
	Price     float64   `json:"p"`
	Size      uint32    `json:"s"`
	Exchange  string    `json:"x"`
	Condition string    `json:"c"`
}

// DailyAuctions contains all the opening and closing auctions for a symbol in a single day
type DailyAuctions struct {
	Date    civil.Date `json:"d"`
	Opening []Auction  `json:"o"`
	Closing []Auction  `json:"c"`
}

//easyjson:json
type snapshotsResponse map[string]*Snapshot

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
	ID        int64     `json:"i"`
	TakerSide TakerSide `json:"tks"`
}

// CryptoQuote is crypto quote
type CryptoQuote struct {
	Timestamp time.Time `json:"t"`
	BidPrice  float64   `json:"bp"`
	BidSize   float64   `json:"bs"`
	AskPrice  float64   `json:"ap"`
	AskSize   float64   `json:"as"`
}

// CryptoBar is an aggregate of crypto trades
type CryptoBar struct {
	Timestamp  time.Time `json:"t"`
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     float64   `json:"v"`
	TradeCount uint64    `json:"n"`
	VWAP       float64   `json:"vw"`
}

// CryptoSnapshot is a snapshot of a crypto symbol
type CryptoSnapshot struct {
	LatestTrade  *CryptoTrade `json:"latestTrade"`
	LatestQuote  *CryptoQuote `json:"latestQuote"`
	MinuteBar    *CryptoBar   `json:"minuteBar"`
	DailyBar     *CryptoBar   `json:"dailyBar"`
	PrevDailyBar *CryptoBar   `json:"prevDailyBar"`
}

// CryptoSnapshots is the snapshots for multiple crypto symbols
type CryptoSnapshots struct {
	Snapshots map[string]CryptoSnapshot `json:"snapshots"`
}

// NewsImage is a single image for a news article.
type NewsImage struct {
	Size string `json:"size"`
	URL  string `json:"url"`
}

// News is a single news article.
type News struct {
	ID        int         `json:"id"`
	Author    string      `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Headline  string      `json:"headline"`
	Summary   string      `json:"summary"`
	Content   string      `json:"content"`
	Images    []NewsImage `json:"images"`
	URL       string      `json:"url"`
	Symbols   []string    `json:"symbols"`
}

type multiTradeResponse struct {
	NextPageToken *string            `json:"next_page_token"`
	Trades        map[string][]Trade `json:"trades"`
}

type multiQuoteResponse struct {
	NextPageToken *string            `json:"next_page_token"`
	Quotes        map[string][]Quote `json:"quotes"`
}

type multiBarResponse struct {
	NextPageToken *string          `json:"next_page_token"`
	Bars          map[string][]Bar `json:"bars"`
}

type multiAuctionsResponse struct {
	NextPageToken *string                    `json:"next_page_token"`
	Auctions      map[string][]DailyAuctions `json:"auctions"`
}

type latestBarsResponse struct {
	Bars map[string]Bar `json:"bars"`
}

type latestTradesResponse struct {
	Trades map[string]Trade `json:"trades"`
}

type latestQuotesResponse struct {
	Quotes map[string]Quote `json:"quotes"`
}

type cryptoMultiTradeResponse struct {
	NextPageToken *string                  `json:"next_page_token"`
	Trades        map[string][]CryptoTrade `json:"trades"`
}

type cryptoMultiBarResponse struct {
	NextPageToken *string                `json:"next_page_token"`
	Bars          map[string][]CryptoBar `json:"bars"`
}

type latestCryptoBarsResponse struct {
	Bars map[string]CryptoBar `json:"bars"`
}

type latestCryptoTradesResponse struct {
	Trades map[string]CryptoTrade `json:"trades"`
}

type latestCryptoQuotesResponse struct {
	Quotes map[string]CryptoQuote `json:"quotes"`
}

type newsResponse struct {
	NextPageToken *string `json:"next_page_token"`
	News          []News  `json:"news"`
}
