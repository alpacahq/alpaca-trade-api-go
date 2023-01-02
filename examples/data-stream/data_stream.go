package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

func main() {
	var tradeCount, quoteCount, barCount int32
	// modify these according to your needs
	tradeHandler := func(t stream.Trade) {
		atomic.AddInt32(&tradeCount, 1)
	}
	quoteHandler := func(q stream.Quote) {
		atomic.AddInt32(&quoteCount, 1)
	}
	barHandler := func(b stream.Bar) {
		atomic.AddInt32(&barCount, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// setting up cancelling upon interrupt
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	go func() {
		<-s
		cancel()
	}()

	// Creating a client that connexts to iex
	c := stream.NewStocksClient(
		"iex",
		// configuring initial subscriptions and handlers
		stream.WithTrades(tradeHandler, "SPY"),
		stream.WithQuotes(quoteHandler, "AAPL", "SPY"),
		stream.WithBars(barHandler, "AAPL", "SPY"),
		// use stream.WithDailyBars to subscribe to daily bars too
		// use stream.WithCredentials to manually override envvars
		// use stream.WithHost to manually override envvar
		// use stream.WithLogger to use your own logger (i.e. zap, logrus) instead of log
		// use stream.WithProcessors to use multiple processing gourotines
		// use stream.WithBufferSize to change buffer size
		// use stream.WithReconnectSettings to change reconnect settings
	)

	// periodically displaying number of trades/quotes/bars received so far
	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Println("trades:", tradeCount, "quotes:", quoteCount, "bars:", barCount)
		}
	}()

	if err := c.Connect(ctx); err != nil {
		log.Fatalf("could not establish connection, error: %s", err)
	}
	fmt.Println("established connection")

	// starting a goroutine that checks whether the client has terminated
	go func() {
		err := <-c.Terminated()
		if err != nil {
			log.Fatalf("terminated with error: %s", err)
		}
		fmt.Println("exiting")
		os.Exit(0)
	}()

	time.Sleep(3 * time.Second)
	// Adding AAPL trade subscription
	if err := c.SubscribeToTrades(tradeHandler, "AAPL"); err != nil {
		log.Fatalf("error during subscribing: %s", err)
	}
	fmt.Println("subscribed to AAPL trades")

	time.Sleep(3 * time.Second)
	// Unsubscribing from AAPL quotes
	if err := c.UnsubscribeFromQuotes("AAPL"); err != nil {
		log.Fatalf("error during unsubscribing: %s", err)
	}
	fmt.Println("unsubscribed from AAPL quotes")

	// and so on...
	time.Sleep(100 * time.Second)
	fmt.Println("we're done")
}
