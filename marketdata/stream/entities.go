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
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    uint64
	Timestamp time.Time
}

// errorMessage is an error received from the server
type errorMessage struct {
	msg  string
	code int
}

// subscriptionMessage is a subscription confirmation received from the server
type subscriptionMessage struct {
	trades []string
	quotes []string
	bars   []string
}
