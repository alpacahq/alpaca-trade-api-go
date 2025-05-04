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
	// DelayedSIP is 15-minute delayed SIP. It can only be used in the latest
	// endpoints and on the stream. For historical endpoints you can simply
	// use sip and set the end parameter to 15 minutes ago, or leave it empty.
	DelayedSIP Feed = "delayed_sip"
)

// CryptoFeed defines the source feed of crypto data.
type CryptoFeed = string

const (
	// US is the crypto feed for the United States.
	US     CryptoFeed = "us"
	GLOBAL CryptoFeed = "global"
)

// OptionFeed defines the source feed of option data.
type OptionFeed = string

const (
	OPRA       Feed = "opra"
	Indicative Feed = "indicative"
)

type OptionType = string

const (
	Call OptionType = "call"
	Put  OptionType = "put"
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

type (
	CryptoPerpTrade CryptoTrade
	CryptoPerpQuote CryptoQuote
	CryptoPerpBar   CryptoBar
)

type CryptoPerpPricing struct {
	IndexPrice      float64   `json:"ip"`
	MarkPrice       float64   `json:"mp"`
	FundingRate     float64   `json:"fr"`
	OpenInterest    float64   `json:"oi"`
	Timestamp       time.Time `json:"t" `
	NextFundingTime time.Time `json:"ft"`
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

type ReverseSplit struct {
	Symbol      string      `json:"symbol"`
	NewRate     float64     `json:"new_rate"`
	OldRate     float64     `json:"old_rate"`
	ProcessDate civil.Date  `json:"process_date"`
	ExDate      civil.Date  `json:"ex_date"`
	RecordDate  *civil.Date `json:"record_date,omitempty"`
	PayableDate *civil.Date `json:"payable_date,omitempty"`
}

type ForwardSplit struct {
	Symbol                string      `json:"symbol"`
	NewRate               float64     `json:"new_rate"`
	OldRate               float64     `json:"old_rate"`
	ProcessDate           civil.Date  `json:"process_date"`
	ExDate                civil.Date  `json:"ex_date"`
	RecordDate            *civil.Date `json:"record_date,omitempty"`
	PayableDate           *civil.Date `json:"payable_date,omitempty"`
	DueBillRedemptionDate *civil.Date `json:"due_bill_redemption_date,omitempty"`
}

type UnitSplit struct {
	NewSymbol       string      `json:"new_symbol"`
	NewRate         float64     `json:"new_rate"`
	OldSymbol       string      `json:"old_symbol"`
	OldRate         float64     `json:"old_rate"`
	AlternateSymbol string      `json:"alternate_symbol"`
	AlternateRate   float64     `json:"alternate_rate"`
	ProcessDate     civil.Date  `json:"process_date"`
	EffectiveDate   civil.Date  `json:"effective_date"`
	PayableDate     *civil.Date `json:"payable_date,omitempty"`
}

type CashDividend struct {
	Symbol         string      `json:"symbol"`
	Rate           float64     `json:"rate"`
	Foreign        bool        `json:"foreign"`
	Special        bool        `json:"special"`
	ProcessDate    civil.Date  `json:"process_date"`
	ExDate         civil.Date  `json:"ex_date"`
	RecordDate     *civil.Date `json:"record_date,omitempty"`
	PayableDate    *civil.Date `json:"payable_date,omitempty"`
	DueBillOffDate *civil.Date `json:"due_bill_off_date,omitempty"`
	DueBillOnDate  *civil.Date `json:"due_bill_on_date,omitempty"`
}

type CashMerger struct {
	AcquirerSymbol *string     `json:"acquirer_symbol,omitempty"`
	AcquireeSymbol string      `json:"acquiree_symbol"`
	Rate           float64     `json:"rate"`
	ProcessDate    civil.Date  `json:"process_date"`
	EffectiveDate  civil.Date  `json:"effective_date"`
	PayableDate    *civil.Date `json:"payable_date,omitempty"`
}

type StockMerger struct {
	AcquirerSymbol string      `json:"acquirer_symbol"`
	AcquirerRate   float64     `json:"acquirer_rate"`
	AcquireeSymbol string      `json:"acquiree_symbol"`
	AcquireeRate   float64     `json:"acquiree_rate"`
	ProcessDate    civil.Date  `json:"process_date"`
	EffectiveDate  civil.Date  `json:"effective_date"`
	PayableDate    *civil.Date `json:"payable_date,omitempty"`
}

type StockAndCashMerger struct {
	AcquirerSymbol string      `json:"acquirer_symbol"`
	AcquirerRate   float64     `json:"acquirer_rate"`
	AcquireeSymbol string      `json:"acquiree_symbol"`
	AcquireeRate   float64     `json:"acquiree_rate"`
	CashRate       float64     `json:"cash_rate"`
	ProcessDate    civil.Date  `json:"process_date"`
	EffectiveDate  civil.Date  `json:"effective_date"`
	PayableDate    *civil.Date `json:"payable_date,omitempty"`
}

type StockDividend struct {
	Symbol      string      `json:"symbol"`
	Rate        float64     `json:"rate"`
	ProcessDate civil.Date  `json:"process_date"`
	ExDate      civil.Date  `json:"ex_date"`
	RecordDate  *civil.Date `json:"record_date,omitempty"`
	PayableDate *civil.Date `json:"payable_date,omitempty"`
}

type Redemption struct {
	Symbol      string      `json:"symbol"`
	Rate        float64     `json:"rate"`
	PayableDate *civil.Date `json:"payable_date,omitempty"`
	ProcessDate civil.Date  `json:"process_date"`
}

type SpinOff struct {
	SourceSymbol          string      `json:"source_symbol"`
	SourceRate            float64     `json:"source_rate"`
	NewSymbol             string      `json:"new_symbol"`
	NewRate               float64     `json:"new_rate"`
	ProcessDate           civil.Date  `json:"process_date"`
	ExDate                civil.Date  `json:"ex_date"`
	PayableDate           *civil.Date `json:"payable_date,omitempty"`
	RecordDate            *civil.Date `json:"record_date,omitempty"`
	DueBillRedemptionDate *civil.Date `json:"due_bill_redemption_date,omitempty"`
}

type NameChange struct {
	NewSymbol   string     `json:"new_symbol"`
	OldSymbol   string     `json:"old_symbol"`
	ProcessDate civil.Date `json:"process_date"`
}

type WorthlessRemoval struct {
	Symbol      string     `json:"symbol"`
	ProcessDate civil.Date `json:"process_date"`
}

type RightsDistribution struct {
	SourceSymbol   string      `json:"source_symbol"`
	NewSymbol      string      `json:"new_symbol"`
	Rate           float64     `json:"rate"`
	ProcessDate    civil.Date  `json:"process_date"`
	ExDate         civil.Date  `json:"ex_date"`
	PayableDate    civil.Date  `json:"payable_date,omitempty"`
	RecordDate     *civil.Date `json:"record_date,omitempty"`
	ExpirationDate *civil.Date `json:"expiration_date,omitempty"`
}

// CorporateActions contains corporate actions grouped by type
type CorporateActions struct {
	ReverseSplits       []ReverseSplit       `json:"reverse_splits,omitempty"`
	ForwardSplits       []ForwardSplit       `json:"forward_splits,omitempty"`
	UnitSplits          []UnitSplit          `json:"unit_splits,omitempty"`
	CashDividends       []CashDividend       `json:"cash_dividends,omitempty"`
	CashMergers         []CashMerger         `json:"cash_mergers,omitempty"`
	StockMergers        []StockMerger        `json:"stock_mergers,omitempty"`
	StockAndCashMergers []StockAndCashMerger `json:"stock_and_cash_mergers,omitempty"`
	StockDividends      []StockDividend      `json:"stock_dividends,omitempty"`
	Redemptions         []Redemption         `json:"redemptions,omitempty"`
	SpinOffs            []SpinOff            `json:"spin_offs,omitempty"`
	NameChanges         []NameChange         `json:"name_changes,omitempty"`
	WorthlessRemovals   []WorthlessRemoval   `json:"worthless_removals,omitempty"`
	RightsDistributions []RightsDistribution `json:"rights_distributions,omitempty"`
}

// OptionTrade is an option trade that happened on the market
type OptionTrade struct {
	Timestamp time.Time `json:"t"`
	Price     float64   `json:"p"`
	Size      uint32    `json:"s"`
	Exchange  string    `json:"x"`
	Condition string    `json:"c"`
}

// OptionBar is an aggregate of option trades
type OptionBar struct {
	Timestamp  time.Time `json:"t"`
	Open       float64   `json:"o"`
	High       float64   `json:"h"`
	Low        float64   `json:"l"`
	Close      float64   `json:"c"`
	Volume     uint64    `json:"v"`
	TradeCount uint64    `json:"n"`
	VWAP       float64   `json:"vw"`
}

// OptionQuote is an option NBBO (National Best Bid and Offer)
type OptionQuote struct {
	Timestamp   time.Time `json:"t"`
	BidPrice    float64   `json:"bp"`
	BidSize     uint32    `json:"bs"`
	BidExchange string    `json:"bx"`
	AskPrice    float64   `json:"ap"`
	AskSize     uint32    `json:"as"`
	AskExchange string    `json:"ax"`
	Condition   string    `json:"c"`
}

type OptionGreeks struct {
	Delta float64 `json:"delta"`
	Gamma float64 `json:"gamma"`
	Rho   float64 `json:"rho"`
	Theta float64 `json:"theta"`
	Vega  float64 `json:"vega"`
}

type OptionSnapshot struct {
	LatestTrade       *OptionTrade  `json:"latestTrade"`
	LatestQuote       *OptionQuote  `json:"latestQuote"`
	ImpliedVolatility float64       `json:"impliedVolatility,omitempty"`
	Greeks            *OptionGreeks `json:"greeks,omitempty"`
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

type cryptoMultiQuoteResponse struct {
	NextPageToken *string                  `json:"next_page_token"`
	Quotes        map[string][]CryptoQuote `json:"quotes"`
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

type latestCryptoPerpPricingResponse struct {
	Pricing map[string]CryptoPerpPricing `json:"pricing"`
}

type latestCryptoQuotesResponse struct {
	Quotes map[string]CryptoQuote `json:"quotes"`
}

type newsResponse struct {
	NextPageToken *string `json:"next_page_token"`
	News          []News  `json:"news"`
}

type corporateActionsResponse struct {
	NextPageToken    *string          `json:"next_page_token"`
	CorporateActions CorporateActions `json:"corporate_actions"`
}

type multiOptionTradeResponse struct {
	NextPageToken *string                  `json:"next_page_token"`
	Trades        map[string][]OptionTrade `json:"trades"`
}

type multiOptionBarResponse struct {
	NextPageToken *string                `json:"next_page_token"`
	Bars          map[string][]OptionBar `json:"bars"`
}

type latestOptionTradesResponse struct {
	Trades map[string]OptionTrade `json:"trades"`
}

type latestOptionQuotesResponse struct {
	Quotes map[string]OptionQuote `json:"quotes"`
}

type optionSnapshotsResponse struct {
	NextPageToken *string                   `json:"next_page_token"`
	Snapshots     map[string]OptionSnapshot `json:"snapshots"`
}
