package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// Get AAPL and MSFT trades from the tenth of a second of the 2021-08-09 market open
func trades() {
	multiTrades, err := marketdata.GetMultiTrades([]string{"AAPL", "MSFT"}, marketdata.GetTradesParams{
		Start: time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		End:   time.Date(2021, 8, 9, 13, 30, 0, 10000000, time.UTC),
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
}

// Get first 30 TSLA quotes from 2021-08-09 market open
func quotes() {
	quotes, err := marketdata.GetQuotes("TSLA", marketdata.GetQuotesParams{
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
}

// Get all the IBM and GE 5-minute bars from the first half hour of the 2021-08-09 market open
func barsAsync() {
	for item := range marketdata.GetMultiBarsAsync([]string{"IBM", "GE"}, marketdata.GetBarsParams{
		TimeFrame:  marketdata.NewTimeFrame(5, marketdata.Min),
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

// Get Facebook bars
func bars() {
	bars, err := marketdata.GetBars("META", marketdata.GetBarsParams{
		TimeFrame: marketdata.OneDay,
		Start:     time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2022, 6, 22, 0, 0, 0, 0, time.UTC),
		AsOf:      "2022-06-10", // Leaving it empty yields the same results
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("META bars:")
	for _, bar := range bars {
		fmt.Printf("%+v\n", bar)
	}
}

// Get Average Daily Trading Volume
func adtv() {
	start := time.Date(2021, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 9, 1, 0, 0, 0, 0, time.UTC)
	averageVolume, count, err := getADTV("AAPL", start, end)
	if err != nil {
		panic(err)
	}
	fmt.Printf("AAPL ADTV: %.2f (%d marketdays)\n", averageVolume, count)
}

func news() {
	news, err := marketdata.GetNews(marketdata.GetNewsParams{
		Symbols:    []string{"AAPL", "TSLA"},
		Start:      time.Date(2021, 5, 6, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 5, 7, 0, 0, 0, 0, time.UTC),
		TotalLimit: 4,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("news:")
	for _, n := range news {
		fmt.Printf("%+v\n", n)
	}
}

func auctions() {
	auctions, err := marketdata.GetAuctions("IBM", marketdata.GetAuctionsParams{
		Start: time.Date(2022, 10, 17, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2022, 10, 20, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("IBM auctions:")
	for _, da := range auctions {
		fmt.Printf(" Date: %s\n", da.Date)
		fmt.Println(" Opening:")
		for _, a := range da.Opening {
			fmt.Printf("  %+v\n", a)
		}
		fmt.Println(" Closing:")
		for _, a := range da.Closing {
			fmt.Printf("  %+v\n", a)
		}
		fmt.Println()
	}
}

type example struct {
	Name string
	Func func()
}

func main() {
	examples := []example{
		{Name: "trades", Func: trades},
		{Name: "quotes", Func: quotes},
		{Name: "barsAsync", Func: barsAsync},
		{Name: "bars", Func: bars},
		{Name: "adtv", Func: adtv},
		{Name: "news", Func: news},
		{Name: "auctions", Func: auctions},
	}
	for {
		fmt.Println("Examples: ")
		for i, e := range examples {
			fmt.Printf("[ %d ] %s\n", i, e.Name)
		}
		fmt.Print("Please type the number of the example you'd like to run or q to exit: ")
		r := bufio.NewReader(os.Stdin)
		s, err := r.ReadString('\n')
		if err != nil {
			panic(err)
		}
		s = strings.TrimSpace(s)
		if s == "q" {
			fmt.Println("Bye!")
			break
		}
		idx, err := strconv.Atoi(s)
		if err != nil {
			fmt.Println("Please input a number!")
			fmt.Println()
			continue
		}
		if idx < 0 || idx >= len(examples) {
			fmt.Printf("Your selection should be between 0 and %d\n\n", len(examples)-1)
			continue
		}
		fmt.Printf("Running example: %s\n", examples[idx].Name)
		examples[idx].Func()
		fmt.Println()
	}
}

func getADTV(symbol string, start, end time.Time) (av float64, n int, err error) {
	var totalVolume uint64
	for item := range marketdata.GetBarsAsync(symbol, marketdata.GetBarsParams{
		Start: start,
		End:   end,
	}) {
		if err = item.Error; err != nil {
			return
		}
		totalVolume += item.Bar.Volume
		n++
	}
	if n == 0 {
		return
	}
	av = float64(totalVolume) / float64(n)
	return
}
