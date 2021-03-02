package stream

import "time"

// Trade is a stock trade that happened on the market
type Trade struct {
	ID         int64     `mapstructure:"i"`
	Symbol     string    `mapstructure:"S"`
	Exchange   string    `mapstructure:"x"`
	Price      float64   `mapstructure:"p"`
	Size       uint32    `mapstructure:"s"`
	Timestamp  time.Time `mapstructure:"t"`
	Conditions []string  `mapstructure:"c"`
	Tape       string    `mapstructure:"z"`
}

// Quote is a stock quote from the market
type Quote struct {
	Symbol      string    `mapstructure:"S"`
	BidExchange string    `mapstructure:"bx"`
	BidPrice    float64   `mapstructure:"bp"`
	BidSize     uint32    `mapstructure:"bs"`
	AskExchange string    `mapstructure:"ax"`
	AskPrice    float64   `mapstructure:"ap"`
	AskSize     uint32    `mapstructure:"as"`
	Timestamp   time.Time `mapstructure:"t"`
	Conditions  []string  `mapstructure:"c"`
	Tape        string    `mapstructure:"z"`
}

// Bar is an aggregate of trades
type Bar struct {
	Symbol    string    `mapstructure:"S"`
	Open      float64   `mapstructure:"o"`
	High      float64   `mapstructure:"h"`
	Low       float64   `mapstructure:"l"`
	Close     float64   `mapstructure:"c"`
	Volume    uint64    `mapstructure:"v"`
	Timestamp time.Time `mapstructure:"t"`
}
