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

	baseURL := "ws://stream.data.alpaca.markets/v1beta1/crypto-perps"

	// Creating a client that connects to iex
	c := stream.NewCryptoPerpsClient(
		marketdata.GLOBAL,
		stream.WithLogger(stream.DefaultLogger()),
		stream.WithBaseURL(baseURL), // Set the base URL
		// configuring initial subscriptions and handlers
		stream.WithCryptoPerpsTrades(func(ct stream.CryptoPerpsTrade) {
			fmt.Printf("TRADE: %+v\n", ct)
		}, "BTC-PERP"),
		stream.WithCryptoPerpsQuotes(func(cq stream.CryptoPerpsQuote) {
			fmt.Printf("QUOTE: %+v\n", cq)
		}, "BTC-PERP"),
		stream.WithCryptoPerpsOrderbooks(func(cob stream.CryptoPerpsOrderbook) {
			fmt.Printf("ORDERBOOK: %+v\n", cob)
		}, "BTC-PERP"),
		stream.WithCryptoPerpsBars(func(cb stream.CryptoPerpsBar) {
			fmt.Printf("BAR: %+v\n", cb)
		}, "BTC-PERP"),
		stream.WithCryptoPerpsUpdatedBars(func(cb stream.CryptoPerpsBar) {
			fmt.Printf("UPDATED BAR: %+v\n", cb)
		}, "BTC-PERP"),
		stream.WithCryptoPerpsDailyBars(func(cb stream.CryptoPerpsBar) {
			fmt.Printf("DAILY BAR: %+v\n", cb)
		}, "BTC-PERP"),
	)

	if err := c.Connect(ctx); err != nil {
		panic(err)
	}
	if err := <-c.Terminated(); err != nil {
		panic(err)
	}
}
