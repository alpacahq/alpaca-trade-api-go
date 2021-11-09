package stream

import "time"

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

// LULD is a Limit Up Limit Down message
type LULD struct {
	Symbol         string
	LimitUpPrice   float64
	LimitDownPrice float64
	Indicator      string
	Timestamp      time.Time
	Tape           string
}

type CryptoTrade struct {
	Symbol    string
	Exchange  string
	Price     float64
	Size      float64
	Timestamp time.Time
	Id        int64
	// TakerSide is the taker's side: one of B, S or -.
	// B is buy, S is sell and - is unknown.
	TakerSide string
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

// errorMessage is an error received from the server
type errorMessage struct {
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
