package main

import (
	"fmt"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
)

func main() {
	// Get AAPL and MSFT trades from the first second of the 2021-08-09 market open
	multiTrades, err := alpaca.GetMultiTrades([]string{"AAPL", "MSFT"}, alpaca.GetTradesParams{
		Start: time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		End:   time.Date(2021, 8, 9, 13, 30, 1, 0, time.UTC),
	})
	if err != nil {
		panic(err)
	}
	for symbol, trades := range multiTrades {
		fmt.Println(symbol + " trades:")
		for _, trade := range trades {
			fmt.Printf("%+v\n", trade)
		}
	}
	fmt.Println()

	// Get first 30 TSLA quotes from 2021-08-09 market open
	quotes, err := alpaca.GetQuotes("TSLA", alpaca.GetQuotesParams{
		Start:      time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		TotalLimit: 30,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("TSLA quotes:")
	for _, quote := range quotes {
		fmt.Printf("%+v\n", quote)
	}
	fmt.Println()

	// Get all the IBM and GE minute bars from the first half hour of the 2021-08-09 market open
	for item := range alpaca.GetMultiBarsAsync([]string{"IBM", "GE"}, alpaca.GetBarsParams{
		TimeFrame:  marketdata.Min,
		Adjustment: marketdata.Split,
		Start:      time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		End:        time.Date(2021, 8, 9, 14, 0, 0, 0, time.UTC),
	}) {
		if err := item.Error; err != nil {
			panic(err)
		}
		fmt.Printf("%s: %+v\n", item.Symbol, item.Bar)
	}
}
