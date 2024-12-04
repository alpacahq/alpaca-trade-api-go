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

	baseURL := "https://stream.data.alpaca.markets/v1beta1/crypto-perps"

	// Creating a client that connects to iex
	c := stream.NewCryptoPerpsClient(
		marketdata.GLOBAL,
		stream.WithLogger(stream.DefaultLogger()),
		stream.WithBaseURL(baseURL), // Set the base URL
		// configuring initial subscriptions and handlers
		stream.WithCryptoTrades(func(ct stream.CryptoTrade) {
			fmt.Printf("TRADE: %+v\n", ct)
		}, "*"),
		stream.WithCryptoQuotes(func(cq stream.CryptoQuote) {
			fmt.Printf("QUOTE: %+v\n", cq)
		}, "BTC-PERP"),
		stream.WithCryptoOrderbooks(func(cob stream.CryptoOrderbook) {
			fmt.Printf("ORDERBOOK: %+v\n", cob)
		}, "BTC-PERP"),
	)

	if err := c.Connect(ctx); err != nil {
		panic(err)
	}
	if err := <-c.Terminated(); err != nil {
		panic(err)
	}
}
