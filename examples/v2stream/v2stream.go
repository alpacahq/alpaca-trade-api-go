package main

import (
	"fmt"
	"os"

	"github.com/alpacahq/alpaca-trade-api-go/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/alpacahq/alpaca-trade-api-go/v2/stream"
)

func main() {
	// You can set your credentials here in the code, or (preferably) via the
	// APCA_API_KEY_ID and APCA_API_SECRET_KEY environment variables
	apiKey := "YOUR_API_KEY_HERE"
	apiSecret := "YOUR_API_SECRET_HERE"
	if common.Credentials().ID == "" {
		os.Setenv(common.EnvApiKeyID, apiKey)
	}
	if common.Credentials().Secret == "" {
		os.Setenv(common.EnvApiSecretKey, apiSecret)
	}

	// uncomment if you have PRO subscription
	// stream.UseFeed("sip")

	if err := stream.SubscribeTradeUpdates(tradeUpdateHandler); err != nil {
		panic(err)
	}

	if err := stream.SubscribeTrades(tradeHandler, "AAPL"); err != nil {
		panic(err)
	}
	if err := stream.SubscribeQuotes(quoteHandler, "MSFT"); err != nil {
		panic(err)
	}
	if err := stream.SubscribeBars(barHandler, "IBM"); err != nil {
		panic(err)
	}

	select {}
}

func tradeUpdateHandler(update alpaca.TradeUpdate) {
	fmt.Println("trade update", update)
}

func tradeHandler(trade stream.Trade) {
	fmt.Println("trade", trade)
}

func quoteHandler(quote stream.Quote) {
	fmt.Println("quote", quote)
}

func barHandler(bar stream.Bar) {
	fmt.Println("bar", bar)
}
