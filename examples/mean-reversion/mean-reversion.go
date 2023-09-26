package main

import (
	"context"
	"fmt"
	"log"
	"os"
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

type alpacaClientContainer struct {
	tradeClient   *alpaca.Client
	dataClient    *marketdata.Client
	streamClient  *stream.StocksClient
	feed          marketdata.Feed
	movingAverage *movingaverage.MovingAverage
	lastOrder     string
	stock         string
}

var algo alpacaClientContainer

func init() {
	// You can set your API key/secret here or you can use environment variables!
	apiKey := ""
	apiSecret := ""
	// Change baseURL to https://paper-api.alpaca.markets if you want use paper!
	baseURL := ""
	// Change feed to sip if you have proper subscription
	feed := "iex"

	// Check if user input a stock, default is AAPL
	stock := "AAPL"
	if len(os.Args[1:]) == 1 {
		stock = os.Args[1]
	}
	algo = alpacaClientContainer{
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
		stock:         stock,
	}
}

func main() {
	fmt.Println("Cancelling all open orders so they don't impact our buying power...")
	orders, err := algo.tradeClient.GetOrders(alpaca.GetOrdersRequest{
		Status: "open",
		Until:  time.Now(),
		Limit:  100,
	})
	if err != nil {
		log.Fatalf("Failed to list orders: %v", err)
	}
	for _, order := range orders {
		if err := algo.tradeClient.CancelOrder(order.ID); err != nil {
			log.Fatalf("Failed to cancel orders: %v", err)
		}
	}
	fmt.Printf("%d order(s) cancelled\n", len(orders))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := algo.streamClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to the marketdata stream: %v", err)
	}
	if err := algo.streamClient.SubscribeToBars(algo.onBar, algo.stock); err != nil {
		log.Fatalf("Failed to subscribe to the bars stream: %v", err)
	}

	go func() {
		if err := <-algo.streamClient.Terminated(); err != nil {
			log.Fatalf("The marketdata stream was terminated: %v", err)
		}
	}()

	for {
		isOpen, err := algo.awaitMarketOpen()
		if err != nil {
			log.Fatalf("Failed to wait for market open: %v", err)
		}
		if !isOpen {
			time.Sleep(1 * time.Minute)
			continue
		}
		fmt.Printf("The market is open! Waiting for %s minute bars...\n", algo.stock)

		// Reset the moving average for the day
		algo.movingAverage = movingaverage.New(windowSize)

		bars, err := algo.dataClient.GetBars(algo.stock, marketdata.GetBarsRequest{
			TimeFrame: marketdata.OneMin,
			Start:     time.Now().Add(-1 * (windowSize + 1) * time.Minute),
			End:       time.Now(),
			Feed:      algo.feed,
		})
		if err != nil {
			log.Fatalf("Failed to get historical bar: %v", err)
		}
		for _, bar := range bars {
			algo.movingAverage.Add(bar.Close)
		}

		// During market open we react on the minute bars (onBar)

		clock, err := algo.tradeClient.GetClock()
		if err != nil {
			log.Fatalf("Failed to get clock: %v", err)
		}
		untilClose := clock.NextClose.Sub(clock.Timestamp.Add(-15 * time.Minute))
		time.Sleep(untilClose)

		fmt.Println("Market closing soon. Closing position.")
		if _, err := algo.tradeClient.ClosePosition(algo.stock, alpaca.ClosePositionRequest{}); err != nil {
			log.Fatalf("Failed to close position: %v", algo.stock)
		}
		fmt.Println("Position closed.")
	}
}

func (alp alpacaClientContainer) onBar(bar stream.Bar) {
	clock, err := algo.tradeClient.GetClock()
	if err != nil {
		fmt.Println("Failed to get clock:", err)
		return
	}
	if !clock.IsOpen {
		return
	}

	if algo.lastOrder != "" {
		_ = alp.tradeClient.CancelOrder(algo.lastOrder)
	}

	algo.movingAverage.Add(bar.Close)
	count := algo.movingAverage.Count()
	if count < windowSize {
		fmt.Printf("Waiting for %d bars, now we have %d", windowSize, count)
		return
	}
	avg := algo.movingAverage.Avg()
	fmt.Printf("Latest minute bar close price: %g, latest %d average: %g\n",
		bar.Close, windowSize, avg)
	if err := algo.rebalance(bar.Close, avg); err != nil {
		fmt.Println("Failed to rebalance:", err)
	}
}

// Spin until the market is open.
func (alp alpacaClientContainer) awaitMarketOpen() (bool, error) {
	clock, err := algo.tradeClient.GetClock()
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
func (alp alpacaClientContainer) rebalance(currPrice, avg float64) error {
	// Get our position, if any.
	positionQty := 0
	positionVal := 0.0
	position, err := alp.tradeClient.GetPosition(algo.stock)
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
			if err := alp.submitLimitOrder(positionQty, algo.stock, currPrice, "sell"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		} else {
			fmt.Println("Price higher than average, but we have no potision.")
		}
	} else if currPrice < avg {
		// Determine optimal amount of shares based on portfolio and market data.
		account, err := alp.tradeClient.GetAccount()
		if err != nil {
			return fmt.Errorf("get account: %w", err)
		}
		buyingPower, _ := account.BuyingPower.Float64()
		positions, err := alp.tradeClient.GetPositions()
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
			if err := alp.submitLimitOrder(qtyToBuy, algo.stock, currPrice, "buy"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		} else {
			amountToAdd *= -1
			qtyToSell := int(amountToAdd / currPrice)
			if qtyToSell > positionQty {
				qtyToSell = positionQty
			}
			if err := alp.submitLimitOrder(qtyToSell, algo.stock, currPrice, "sell"); err != nil {
				return fmt.Errorf("submit limit order: %v", err)
			}
		}
	}
	return nil
}

// Submit a limit order if quantity is above 0.
func (alp alpacaClientContainer) submitLimitOrder(qty int, symbol string, price float64, side string) error {
	if qty <= 0 {
		fmt.Printf("Quantity is <= 0, order of | %d %s %s | not sent.\n", qty, symbol, side)
	}
	adjSide := alpaca.Side(side)
	decimalQty := decimal.NewFromInt(int64(qty))
	order, err := alp.tradeClient.PlaceOrder(alpaca.PlaceOrderRequest{
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
	algo.lastOrder = order.ID
	return nil
}
