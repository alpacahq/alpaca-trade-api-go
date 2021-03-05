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
	Symbol    string    `msgpack:"S"`
	Open      float64   `msgpack:"o"`
	High      float64   `msgpack:"h"`
	Low       float64   `msgpack:"l"`
	Close     float64   `msgpack:"c"`
	Volume    uint64    `msgpack:"v"`
	Timestamp time.Time `msgpack:"t"`
}
