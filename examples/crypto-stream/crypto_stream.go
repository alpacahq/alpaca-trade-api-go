package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata/stream"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creating a client that connexts to iex
	c := stream.NewCryptoClient(
		stream.WithLogger(&logger{}),
		// configuring initial subscriptions and handlers
		stream.WithCryptoTrades(func(ct stream.CryptoTrade) {
			fmt.Printf("%s TRADE %s %g of $%g at %s\n",
				ct.Exchange, ct.Symbol, ct.Size, ct.Price, ct.Timestamp)
		}, "*"),
		stream.WithCryptoQuotes(func(cq stream.CryptoQuote) {
			fmt.Printf("%s QUOTE %s bid: $%g ask: $%g at %s\n",
				cq.Exchange, cq.Symbol, cq.BidPrice, cq.AskPrice, cq.Timestamp)
		}, "BTCUSD"),
	)
	if err := c.Connect(ctx); err != nil {
		panic(err)
	}
	if err := <-c.Terminated(); err != nil {
		panic(err)
	}
}

type logger struct{}

func (l *logger) Infof(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("INFO "+format, v...))
}

func (l *logger) Warnf(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("WARN "+format, v...))
}

func (l *logger) Errorf(format string, v ...interface{}) {
	log.Println(fmt.Sprintf("ERROR "+format, v...))
}
