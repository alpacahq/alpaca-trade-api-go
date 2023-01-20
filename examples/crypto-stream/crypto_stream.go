package main

import (
	"context"
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creating a client that connects to iex
	c := stream.NewCryptoClient(marketdata.US,
		stream.WithLogger(stream.DefaultLogger()),
		// configuring initial subscriptions and handlers
		stream.WithCryptoTrades(func(ct stream.CryptoTrade) {
			fmt.Printf("TRADE: %+v\n", ct)
		}, "*"),
		stream.WithCryptoQuotes(func(cq stream.CryptoQuote) {
			fmt.Printf("QUOTE: %+v\n", cq)
		}, "BTC/USD"),
		stream.WithCryptoOrderbooks(func(cob stream.CryptoOrderbook) {
			fmt.Printf("ORDERBOOK: %+v\n", cob)
		}, "BTC/USD"),
	)
	if err := c.Connect(ctx); err != nil {
		panic(err)
	}
	if err := <-c.Terminated(); err != nil {
		panic(err)
	}
}
