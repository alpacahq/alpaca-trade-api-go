package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	movingaverage "github.com/RobinUS2/golang-moving-average"
	"github.com/shopspring/decimal"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

const (
	windowSize = 20
)

type algo struct {
	tradeClient   *alpaca.Client
	dataClient    *marketdata.Client
	streamClient  *stream.StocksClient
	feed          marketdata.Feed
	movingAverage *movingaverage.MovingAverage
	lastOrder     string
	stock         string
	shouldTrade   atomic.Bool
}

func main() {
	// You can set your API key/secret here or you can use environment variables!
	apiKey := ""
	apiSecret := ""
	// Change baseURL to https://paper-api.alpaca.markets if you want use paper!
	baseURL := ""
	// Change feed to sip if you have proper subscription
	feed := "iex"

	symbol := "AAPL"
	if len(os.Args) > 1 {
		symbol = os.Args[1]
	}
	fmt.Println("Selected symbol: " + symbol)

	a := &algo{
		tradeClient: alpaca.NewClient(alpaca.ClientOpts{
			APIKey:    apiKey,
			APISecret: apiSecret,
			BaseURL:   baseURL,
		}),
		dataClient: marketdata.NewClient(marketdata.ClientOpts{
			APIKey:    apiKey,
			APISecret: apiSecret,
		}),
		streamClient: stream.NewStocksClient(feed,
			stream.WithCredentials(apiKey, apiSecret),
		),
		feed:          feed,
		movingAverage: movingaverage.New(windowSize),
		stock:         symbol,
	}

	fmt.Println("Cancelling all open orders so they don't impact our buying power...")
	orders, err := a.tradeClient.GetOrders(alpaca.GetOrdersRequest{
		Status: "open",
		Until:  time.Now(),
		Limit:  100,
	})
	for _, order := range orders {
		fmt.Printf("%+v\n", order)
	}
	if err != nil {
		log.Fatalf("Failed to list orders: %v", err)
	}
	for _, order := range orders {
		if err := a.tradeClient.CancelOrder(order.ID); err != nil {
			log.Fatalf("Failed to cancel orders: %v", err)
		}
	}
	fmt.Printf("%d order(s) cancelled\n", len(orders))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := a.streamClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to the marketdata stream: %v", err)
	}
	if err := a.streamClient.SubscribeToBars(a.onBar, a.stock); err != nil {
		log.Fatalf("Failed to subscribe to the bars stream: %v", err)
	}

	go func() {
		if err := <-a.streamClient.Terminated(); err != nil {
			log.Fatalf("The marketdata stream was terminated: %v", err)
		}
	}()

	for {
		isOpen, err := a.awaitMarketOpen()
		if err != nil {
			log.Fatalf("Failed to wait for market open: %v", err)
		}
		if !isOpen {
			time.Sleep(1 * time.Minute)
			continue
		}
		fmt.Printf("The market is open! Waiting for %s minute bars...\n", a.stock)

		// Reset the moving average for the day
		a.movingAverage = movingaverage.New(windowSize)

		bars, err := a.dataClient.GetBars(a.stock, marketdata.GetBarsRequest{
			TimeFrame: marketdata.OneMin,
			Start:     time.Now().Add(-1 * (windowSize + 1) * time.Minute),
			End:       time.Now(),
			Feed:      a.feed,
		})
		if err != nil {
			log.Fatalf("Failed to get historical bar: %v", err)
		}
		for _, bar := range bars {
			a.movingAverage.Add(bar.Close)
		}
		a.shouldTrade.Store(true)

		// During market open we react on the minute bars (onBar)

		clock, err := a.tradeClient.GetClock()
		if err != nil {
			log.Fatalf("Failed to get clock: %v", err)
		}
		untilClose := clock.NextClose.Sub(clock.Timestamp.Add(-15 * time.Minute))
		time.Sleep(untilClose)

		fmt.Println("Market closing soon. Closing position.")
		a.shouldTrade.Store(false)
		if _, err := a.tradeClient.ClosePosition(a.stock, alpaca.ClosePositionRequest{}); err != nil {
			log.Fatalf("Failed to close position: %v", a.stock)
		}
		fmt.Println("Position closed.")
	}
}

func (a *algo) onBar(bar stream.Bar) {
	if !a.shouldTrade.Load() {
		return
	}

	if a.lastOrder != "" {
		_ = a.tradeClient.CancelOrder(a.lastOrder)
	}

	a.movingAverage.Add(bar.Close)
	count := a.movingAverage.Count()
	if count < windowSize {
		fmt.Printf("Waiting for %d bars, now we have %d", windowSize, count)
		return
	}
	avg := a.movingAverage.Avg()
	fmt.Printf("Latest minute bar close price: %g, latest %d average: %g\n",
		bar.Close, windowSize, avg)
	if err := a.rebalance(bar.Close, avg); err != nil {
		fmt.Println("Failed to rebalance:", err)
	}
}

// Spin until the market is open.
func (a *algo) awaitMarketOpen() (bool, error) {
	clock, err := a.tradeClient.GetClock()
	if err != nil {
		return false, fmt.Errorf("get clock: %w", err)
	}
	if clock.IsOpen {
		return true, nil
	}
	timeToOpen := int(clock.NextOpen.Sub(clock.Timestamp).Minutes())
	fmt.Printf("%d minutes until next market open\n", timeToOpen)
	return false, nil
}

// Rebalance our position after an update.
func (a *algo) rebalance(currPrice, avg float64) error {
	// Get our position, if any.
	positionQty := 0
	positionVal := 0.0
	position, err := a.tradeClient.GetPosition(a.stock)
	if err != nil {
		if apiErr, ok := err.(*alpaca.APIError); !ok || apiErr.Message != "position does not exist" {
			return fmt.Errorf("get position: %w", err)
		}
	} else {
		positionQty = int(position.Qty.IntPart())
		positionVal, _ = position.MarketValue.Float64()
	}

	if currPrice > avg {
		// Sell our position if the price is above the running average, if any.
		if positionQty > 0 {
			fmt.Println("Setting long position to zero")
			if err := a.submitLimitOrder(positionQty, a.stock, currPrice, "sell"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		} else {
			fmt.Println("Price higher than average, but we have no potision.")
		}
	} else if currPrice < avg {
		// Determine optimal amount of shares based on portfolio and market data.
		account, err := a.tradeClient.GetAccount()
		if err != nil {
			return fmt.Errorf("get account: %w", err)
		}
		buyingPower, _ := account.BuyingPower.Float64()
		positions, err := a.tradeClient.GetPositions()
		if err != nil {
			return fmt.Errorf("list positions: %w", err)
		}
		portfolioVal, _ := account.Cash.Float64()
		for _, position := range positions {
			rawVal, _ := position.MarketValue.Float64()
			portfolioVal += rawVal
		}
		portfolioShare := (avg - currPrice) / currPrice * 200
		targetPositionValue := portfolioVal * portfolioShare
		amountToAdd := targetPositionValue - positionVal

		// Add to our position, constrained by our buying power; or, sell down to optimal amount of shares.
		if amountToAdd > 0 {
			if amountToAdd > buyingPower {
				amountToAdd = buyingPower
			}
			qtyToBuy := int(amountToAdd / currPrice)
			if err := a.submitLimitOrder(qtyToBuy, a.stock, currPrice, "buy"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		} else {
			amountToAdd *= -1
			qtyToSell := int(amountToAdd / currPrice)
			if qtyToSell > positionQty {
				qtyToSell = positionQty
			}
			if err := a.submitLimitOrder(qtyToSell, a.stock, currPrice, "sell"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		}
	}
	return nil
}

// Submit a limit order if quantity is above 0.
func (a *algo) submitLimitOrder(qty int, symbol string, price float64, side string) error {
	if qty <= 0 {
		fmt.Printf("Quantity is <= 0, order of | %d %s %s | not sent.\n", qty, symbol, side)
	}
	adjSide := alpaca.Side(side)
	decimalQty := decimal.NewFromInt(int64(qty))
	order, err := a.tradeClient.PlaceOrder(alpaca.PlaceOrderRequest{
		Symbol:      symbol,
		Qty:         &decimalQty,
		Side:        adjSide,
		Type:        "limit",
		LimitPrice:  alpaca.RoundLimitPrice(decimal.NewFromFloat(price), adjSide),
		TimeInForce: "day",
	})
	if err != nil {
		return fmt.Errorf("qty=%d symbol=%s side=%s: %w", qty, symbol, side, err)
	}
	fmt.Printf("Limit order of | %d %s %s | sent.\n", qty, symbol, side)
	a.lastOrder = order.ID
	return nil
}
