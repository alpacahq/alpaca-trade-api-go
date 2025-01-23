package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"cloud.google.com/go/civil"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// Get AAPL and MSFT trades from the tenth of a second of the 2021-08-09 market open
func trades() {
	marketdata.GetTrades("AAPL", marketdata.GetTradesRequest{})
	multiTrades, err := marketdata.GetMultiTrades([]string{"AAPL", "MSFT"}, marketdata.GetTradesRequest{
		Start: time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		End:   time.Date(2021, 8, 9, 13, 30, 0, 10000000, time.UTC),
	})
	must(err)
	for symbol, trades := range multiTrades {
		fmt.Println(symbol + " trades:")
		for _, trade := range trades {
			fmt.Printf("%+v\n", trade)
		}
	}
}

// Get first 30 TSLA quotes from 2021-08-09 market open
func quotes() {
	quotes, err := marketdata.GetQuotes("TSLA", marketdata.GetQuotesRequest{
		Start:      time.Date(2021, 8, 9, 13, 30, 0, 0, time.UTC),
		TotalLimit: 30,
	})
	must(err)
	fmt.Println("TSLA quotes:")
	for _, quote := range quotes {
		fmt.Printf("%+v\n", quote)
	}
}

// Get Facebook bars
func bars() {
	bars, err := marketdata.GetBars("META", marketdata.GetBarsRequest{
		TimeFrame: marketdata.OneDay,
		Start:     time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2022, 6, 22, 0, 0, 0, 0, time.UTC),
		AsOf:      "2022-06-10", // Leaving it empty yields the same results
	})
	must(err)
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
	must(err)
	fmt.Printf("AAPL ADTV: %.2f (%d marketdays)\n", averageVolume, count)
}

func news() {
	news, err := marketdata.GetNews(marketdata.GetNewsRequest{
		Symbols:    []string{"AAPL", "TSLA"},
		Start:      time.Date(2021, 5, 6, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 5, 7, 0, 0, 0, 0, time.UTC),
		TotalLimit: 4,
	})
	must(err)
	fmt.Println("news:")
	for _, n := range news {
		fmt.Printf("%+v\n", n)
	}
}

func auctions() {
	auctions, err := marketdata.GetAuctions("IBM", marketdata.GetAuctionsRequest{
		Start: time.Date(2022, 10, 17, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2022, 10, 20, 0, 0, 0, 0, time.UTC),
	})
	must(err)
	fmt.Println("IBM auctions:")
	for i, da := range auctions {
		fmt.Printf(" Date: %s\n", da.Date)
		fmt.Println(" Opening:")
		for _, a := range da.Opening {
			fmt.Printf("  %+v\n", a)
		}
		fmt.Println(" Closing:")
		for _, a := range da.Closing {
			fmt.Printf("  %+v\n", a)
		}
		if i < len(auctions)-1 {
			fmt.Println()
		}
	}
}

func cryptoSpot() {
	fmt.Println("Latest BTC/USD marketdata:")
	quote, err := marketdata.GetLatestCryptoQuote("BTC/USD", marketdata.GetLatestCryptoQuoteRequest{})
	must(err)
	fmt.Printf(" Latest quote: %+v\n", quote)
	trade, err := marketdata.GetLatestCryptoTrade("BTC/USD", marketdata.GetLatestCryptoTradeRequest{})
	must(err)
	fmt.Printf(" Latest trade: %+v\n", trade)
	bar, err := marketdata.GetLatestCryptoBar("BTC/USD", marketdata.GetLatestCryptoBarRequest{})
	must(err)
	fmt.Printf(" Latest bar:   %+v\n", bar)
}

func cryptoPerp() {
	fmt.Println("Latest BTC-PERP (crypto perpetual future) marketdata:")
	quote, err := marketdata.GetLatestCryptoPerpQuote("BTC-PERP", marketdata.GetLatestCryptoQuoteRequest{})
	must(err)
	fmt.Printf(" Latest quote:   %+v\n", quote)
	trade, err := marketdata.GetLatestCryptoPerpTrade("BTC-PERP", marketdata.GetLatestCryptoTradeRequest{})
	must(err)
	fmt.Printf(" Latest trade:   %+v\n", trade)
	bar, err := marketdata.GetLatestCryptoPerpBar("BTC-PERP", marketdata.GetLatestCryptoBarRequest{})
	must(err)
	fmt.Printf(" Latest bar:     %+v\n", bar)
	pricing, err := marketdata.GetLatestCryptoPerpPricing("BTC-PERP", marketdata.GetLatestCryptoPerpPricingRequest{})
	must(err)
	fmt.Printf(" Latest pricing: %+v\n", pricing)
}

func optionChain() {
	fmt.Println("AAPL calls within 5 days")
	chain, err := marketdata.GetOptionChain("AAPL", marketdata.GetOptionChainRequest{
		Type:              marketdata.Call,
		ExpirationDateLte: civil.DateOf(time.Now()).AddDays(5),
	})
	must(err)
	type snap struct {
		marketdata.OptionSnapshot
		Symbol string
	}
	snaps := []snap{}
	for symbol, snapshot := range chain {
		snaps = append(snaps, snap{OptionSnapshot: snapshot, Symbol: symbol})
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Symbol < snaps[j].Symbol })

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	columns := []any{
		"Contract name", "Last trade time", "Price", "Bid", "Ask",
		"IV", "Delta", "Gamma", "Rho", "Theta", "Vega",
	}
	fmt.Fprintf(tw, strings.Repeat("%s\t", len(columns))+"\n", columns...)
	for _, s := range snaps {
		ts := ""
		if s.LatestTrade != nil {
			ts = s.LatestTrade.Timestamp.Format(time.RFC3339)
		}
		price := float64(0)
		if s.LatestTrade != nil {
			price = s.LatestTrade.Price
		}
		bid, ask := float64(0), float64(0)
		if s.LatestQuote != nil {
			bid = s.LatestQuote.BidPrice
			ask = s.LatestQuote.AskPrice
		}
		iv := ""
		if s.ImpliedVolatility != 0 {
			iv = strconv.FormatFloat(s.ImpliedVolatility, 'f', 4, 64)
		}
		var delta, gamma, rho, theta, vega string
		if s.Greeks != nil {
			delta = strconv.FormatFloat(s.Greeks.Delta, 'f', 4, 64)
			gamma = strconv.FormatFloat(s.Greeks.Gamma, 'f', 4, 64)
			rho = strconv.FormatFloat(s.Greeks.Rho, 'f', 4, 64)
			theta = strconv.FormatFloat(s.Greeks.Theta, 'f', 4, 64)
			vega = strconv.FormatFloat(s.Greeks.Vega, 'f', 4, 64)
		}
		fmt.Fprintf(tw, "%s\t%s\t%g\t%g\t%g\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.Symbol, ts, price, bid, ask, iv, delta, gamma, rho, theta, vega)
	}
	tw.Flush()
}

func corporateActions() {
	cas, err := marketdata.GetCorporateActions(marketdata.GetCorporateActionsRequest{
		Symbols: []string{"TSLA"},
		Types:   []string{"forward_split"},
		Start:   civil.Date{Year: 2018, Month: 1, Day: 1},
	})
	must(err)
	fmt.Println("TSLA forward splits:")
	for _, split := range cas.ForwardSplits {
		fmt.Printf(" - %+v\n", split)
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
		{Name: "bars", Func: bars},
		{Name: "adtv", Func: adtv},
		{Name: "news", Func: news},
		{Name: "auctions", Func: auctions},
		{Name: "crypto", Func: cryptoSpot},
		{Name: "crypto_perp", Func: cryptoPerp},
		{Name: "option_chain", Func: optionChain},
		{Name: "corporate_actions", Func: corporateActions},
	}
	for {
		fmt.Println("Examples: ")
		for i, e := range examples {
			fmt.Printf("[ %d ] %s\n", i, e.Name)
		}
		fmt.Print("Please type the number of the example you'd like to run or q to exit: ")
		r := bufio.NewReader(os.Stdin)
		s, err := r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "q" || (err != nil && errors.Is(err, io.EOF)) {
			fmt.Println("Bye!")
			return
		}
		must(err)
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
	bars, err := marketdata.GetBars(symbol, marketdata.GetBarsRequest{
		Start: start,
		End:   end,
	})
	if err != nil {
		return 0, 0, err
	}
	for _, bar := range bars {
		totalVolume += bar.Volume
		n++
	}
	if n == 0 {
		return
	}
	av = float64(totalVolume) / float64(n)
	return
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
