package stream

import (
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

//go:generate go install github.com/mailru/easyjson/...@v0.7.7
//go:generate easyjson -all -snake_case $GOFILE

// Trade is a stock trade that happened on the market
type Trade struct {
	ID         int64
	Symbol     string
	Exchange   string
	Price      float64
	Size       uint32
	Timestamp  time.Time
	Conditions []string
	Tape       string

	internal tradeInternal
}

type tradeInternal struct {
	ReceivedAt time.Time
}

// Internal contains internal fields. There aren't any behavioural or backward compatibility
// promises for them: they can be empty or removed in the future. You should not use them at all.
func (t Trade) Internal() tradeInternal { //nolint:revive // We intentionally return an unexported struct here
	return t.internal
}

// Quote is a stock quote from the market
type Quote struct {
	Symbol      string
	BidExchange string
	BidPrice    float64
	BidSize     uint32
	AskExchange string
	AskPrice    float64
	AskSize     uint32
	Timestamp   time.Time
	Conditions  []string
	Tape        string

	internal quoteInternal
}

type quoteInternal struct {
	ReceivedAt time.Time
}

// Internal contains internal fields. There aren't any behavioural or backward compatibility
// promises for them: they can be empty or removed in the future. You should not use them at all.
func (q Quote) Internal() quoteInternal { //nolint:revive // We intentionally return an unexported struct here
	return q.internal
}

// Bar is an aggregate of trades
type Bar struct {
	Symbol     string
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     uint64
	Timestamp  time.Time
	TradeCount uint64
	VWAP       float64
}

// TradingStatus is a halt or resume status for a security
type TradingStatus struct {
	Symbol     string
	StatusCode string
	StatusMsg  string
	ReasonCode string
	ReasonMsg  string
	Timestamp  time.Time
	Tape       string
}

// Imbalance is an order imbalance message during LULD halts for a security
type Imbalance struct {
	Symbol    string
	Price     float64
	Timestamp time.Time
	Tape      string
}

// LULD is a Limit Up Limit Down message
type LULD struct {
	Symbol         string
	LimitUpPrice   float64
	LimitDownPrice float64
	Indicator      string
	Timestamp      time.Time
	Tape           string
}

type TradeCancelError struct {
	Symbol            string
	ID                int64
	Exchange          string
	Price             float64
	Size              uint32
	CancelErrorAction string
	Tape              string
	Timestamp         time.Time
}

type TradeCorrection struct {
	Symbol              string
	Exchange            string
	OriginalID          int64
	OriginalPrice       float64
	OriginalSize        uint32
	OriginalConditions  []string
	CorrectedID         int64
	CorrectedPrice      float64
	CorrectedSize       uint32
	CorrectedConditions []string
	Tape                string
	Timestamp           time.Time
}

type CryptoTrade struct {
	ID        int64
	Symbol    string
	Exchange  string
	Price     float64
	Size      float64
	Timestamp time.Time
	TakerSide marketdata.TakerSide
}

type CryptoQuote struct {
	Symbol    string
	Exchange  string
	BidPrice  float64
	BidSize   float64
	AskPrice  float64
	AskSize   float64
	Timestamp time.Time
}

type CryptoBar struct {
	Symbol     string
	Exchange   string
	Open       float64
	High       float64
	Low        float64
	Close      float64
	Volume     float64
	Timestamp  time.Time
	TradeCount uint64
	VWAP       float64
}

type CryptoOrderbook struct {
	Symbol    string
	Exchange  string
	Timestamp time.Time
	Bids      []CryptoOrderbookEntry
	Asks      []CryptoOrderbookEntry
	Reset     bool
}

type CryptoOrderbookEntry struct {
	Price float64
	Size  float64
}

// OptionTrade is an option trade that happened on the market
type OptionTrade struct {
	Symbol    string
	Exchange  string
	Price     float64
	Size      uint32
	Timestamp time.Time
	Condition string
}

// OptionQuote is an option quote from the market
type OptionQuote struct {
	Symbol      string
	BidExchange string
	BidPrice    float64
	BidSize     uint32
	AskExchange string
	AskPrice    float64
	AskSize     uint32
	Timestamp   time.Time
	Condition   string
}

type News struct {
	ID        int
	Author    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Headline  string
	Summary   string
	Content   string
	URL       string
	Symbols   []string
}

// errorMessage is an error received from the server
type errorMessage struct { //nolint:errname // Not an actual error.
	msg  string
	code int
}

func (e errorMessage) Error() string {
	// NOTE: these special cases exist because the error message
	// used to be different from the one sent by the server
	switch e.code {
	case 402:
		return "invalid credentials"
	case 410:
		return "subscription change invalid for feed"
	}

	return e.msg
}

type CryptoPerpPricing struct {
	Symbol          string
	Timestamp       time.Time
	Exchange        string
	IndexPrice      float64
	MarkPrice       float64
	FundingRate     float64
	OpenInterest    float64
	NextFundingTime time.Time
}
