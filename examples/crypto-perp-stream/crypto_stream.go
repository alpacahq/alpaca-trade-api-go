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

	c := stream.NewCryptoPerpClient(
		marketdata.GLOBAL,
		stream.WithLogger(stream.DefaultLogger()),
		stream.WithBaseURL(baseURL), // Set the base URL
		// configuring initial subscriptions and handlers
		stream.WithCryptoPerpTrades(func(ct stream.CryptoPerpTrade) {
			fmt.Printf("TRADE: %+v\n", ct)
		}, "BTC-PERP"),
		stream.WithCryptoPerpQuotes(func(cq stream.CryptoPerpQuote) {
			fmt.Printf("QUOTE: %+v\n", cq)
		}, "BTC-PERP"),
		stream.WithCryptoPerpOrderbooks(func(cob stream.CryptoPerpOrderbook) {
			fmt.Printf("ORDERBOOK: %+v\n", cob)
		}, "BTC-PERP"),
		stream.WithCryptoPerpBars(func(cb stream.CryptoPerpBar) {
			fmt.Printf("BAR: %+v\n", cb)
		}, "BTC-PERP"),
		stream.WithCryptoPerpUpdatedBars(func(cb stream.CryptoPerpBar) {
			fmt.Printf("UPDATED BAR: %+v\n", cb)
		}, "BTC-PERP"),
		stream.WithCryptoPerpDailyBars(func(cb stream.CryptoPerpBar) {
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
