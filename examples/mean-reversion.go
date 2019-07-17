package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/shopspring/decimal"
)

type alpacaClientContainer struct {
	client         *alpaca.Client
	closingPrices  []float64
	runningAverage float64
	lastOrder      string
	timeToClose    float64
	stock          string
}

var alpacaClient alpacaClientContainer

func init() {
	API_KEY := "API_KEY"
	API_SECRET := "API_SECRET"
	BASE_URL := "https://paper-api.alpaca.markets"

	os.Setenv(common.EnvApiKeyID, API_KEY)
	os.Setenv(common.EnvApiSecretKey, API_SECRET)
	alpaca.SetBaseUrl(BASE_URL)

	alpacaClient = alpacaClientContainer{
		alpaca.NewClient(common.Credentials()),
		[]float64{},
		0.0,
		"",
		0.0,
		"AAPL",
	}
}

func main() {
	// First, cancel any existing orders so they don't impact our buying power.
	status, until, limit := "open", time.Now(), 100
	orders, _ := alpacaClient.client.ListOrders(&status, &until, &limit)
	for _, order := range orders {
		_ = alpacaClient.client.CancelOrder(order.ID)
	}

	// Wait for market to open
	fmt.Println("Waiting for market to open...")
	for {
		isOpen := alpacaClient.awaitMarketOpen()
		if isOpen {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	fmt.Println("Market Opened.")

	// Get the running average of prices of the last 20 minutes, waiting until we have 20 bars from market open.
	fmt.Println("Waiting for 20 bars...")
	for {
		layout := "2006-01-02T15:04:05.000Z"
		rawTime, _ := time.Parse(layout, time.Now().String())
		currTime := rawTime.String()
		cal, _ := alpacaClient.client.GetCalendar(&currTime, &currTime)
		marketOpen, _ := time.Parse(layout, cal[0].Open)
		bars, _ := alpacaClient.client.GetSymbolBars(alpacaClient.stock, alpaca.ListBarParams{Timeframe: "minute", StartDt: &marketOpen})
		if len(bars) >= 20 {
			break
		} else {
			time.Sleep(60000 * time.Millisecond)
		}
	}
	fmt.Println("We have 20 bars.")

	amtBars := 20
	bars, err := alpacaClient.client.GetSymbolBars(alpacaClient.stock, alpaca.ListBarParams{Timeframe: "minute", Limit: &amtBars})
	if err != nil {
		fmt.Println(err)
	}
	for _, bar := range bars {
		alpacaClient.closingPrices = append(alpacaClient.closingPrices, float64(bar.Close))
		if len(alpacaClient.closingPrices) > 20 {
			alpacaClient.closingPrices = alpacaClient.closingPrices[1:]
		}
		alpacaClient.runningAverage = ((alpacaClient.runningAverage * float64(len(alpacaClient.closingPrices)-1)) + float64(bar.Close)) / float64(len(alpacaClient.closingPrices))
	}

	for {
		alpacaClient.run()
	}
}

// Rebalance our portfolio every minute based off running average data.
func (alp alpacaClientContainer) run() {
	if alpacaClient.lastOrder != "" {
		_ = alp.client.CancelOrder(alpacaClient.lastOrder)
	}

	// Rebalance the portfolio.
	alp.rebalance()

	// Figure out when the market will close so we can prepare to sell beforehand.
	clock, _ := alp.client.GetClock()
	timeToClose := int((clock.NextClose.UnixNano() - clock.Timestamp.UnixNano()) / 1000000)
	if timeToClose < 60000*15 {
		// Close all positions when 15 minutes til market close.
		fmt.Println("Market closing soon.  Closing positions.")

		positions, _ := alp.client.ListPositions()
		for _, position := range positions {
			var orderSide string
			if position.Side == "long" {
				orderSide = "sell"
			} else {
				orderSide = "buy"
			}
			qty, _ := position.Qty.Float64()
			qty = math.Abs(qty)
			alp.submitMarketOrder(int(qty), position.Symbol, orderSide)
			// Run script again after market close for next trading day.
			time.Sleep((60000 * 15) * time.Millisecond)
		}
	} else {
		time.Sleep(60000 * time.Millisecond)
	}
}

// Spin until the market is open.
func (alp alpacaClientContainer) awaitMarketOpen() bool {
	clock, _ := alp.client.GetClock()
	if clock.IsOpen {
		return true
	}
	fmt.Println("spinning")
	return false
}

// Rebalance our position after an update.
func (alp alpacaClientContainer) rebalance() {
	// Get our position, if any.
	positionQty := 0
	positionVal := 0.0
	position, err := alp.client.GetPosition(alpacaClient.stock)
	if err != nil {
	} else {
		positionQty = int(position.Qty.IntPart())
		positionVal, _ = position.MarketValue.Float64()
	}

	// Get the new updated price and running average.
	bars, _ := alp.client.GetSymbolBars(alpacaClient.stock, alpaca.ListBarParams{Timeframe: "minute"})
	currPrice := float64(bars[len(bars)-1].Close)
	alpacaClient.closingPrices = append(alpacaClient.closingPrices, currPrice)
	if len(alpacaClient.closingPrices) > 20 {
		alpacaClient.closingPrices = alpacaClient.closingPrices[1:]
	}
	alpacaClient.runningAverage = ((alpacaClient.runningAverage * float64(len(alpacaClient.closingPrices)-1)) + currPrice) / float64(len(alpacaClient.closingPrices))

	if currPrice > alpacaClient.runningAverage {
		// Sell our position if the price is above the running average, if any.
		if positionQty > 0 {
			fmt.Println("Setting position to zero")
			alp.submitLimitOrder(positionQty, alpacaClient.stock, currPrice, "sell")
		} else {
			fmt.Println("No position in the stock.  No action required.")
		}
	} else if currPrice < alpacaClient.runningAverage {
		// Determine optimal amount of shares based on portfolio and market data.
		account, _ := alp.client.GetAccount()
		buyingPower, _ := account.BuyingPower.Float64()
		positions, _ := alp.client.ListPositions()
		portfolioVal, _ := account.Cash.Float64()
		for _, position := range positions {
			rawVal, _ := position.MarketValue.Float64()
			portfolioVal += rawVal
		}
		portfolioShare := (alpacaClient.runningAverage - currPrice) / currPrice * 200
		targetPositionValue := portfolioVal * portfolioShare
		amountToAdd := targetPositionValue - positionVal

		// Add to our position, constrained by our buying power; or, sell down to optimal amount of shares.
		if amountToAdd > 0 {
			if amountToAdd > buyingPower {
				amountToAdd = buyingPower
			}
			var qtyToBuy = int(amountToAdd / currPrice)
			alp.submitLimitOrder(qtyToBuy, alpacaClient.stock, currPrice, "buy")
		} else {
			amountToAdd *= -1
			var qtyToSell = int(amountToAdd / currPrice)
			if qtyToSell > positionQty {
				qtyToSell = positionQty
			}
			alp.submitLimitOrder(qtyToSell, alpacaClient.stock, currPrice, "buy")
		}
	}
}

// Submit a limit order if quantity is above 0.
func (alp alpacaClientContainer) submitLimitOrder(qty int, symbol string, price float64, side string) error {
	account, _ := alp.client.GetAccount()
	if qty > 0 {
		adjSide := alpaca.Side(side)
		limPrice := decimal.NewFromFloat(price)
		order, err := alp.client.PlaceOrder(alpaca.PlaceOrderRequest{
			AccountID:   account.ID,
			AssetKey:    &symbol,
			Qty:         decimal.NewFromFloat(float64(qty)),
			Side:        adjSide,
			Type:        "limit",
			LimitPrice:  &limPrice,
			TimeInForce: "day",
		})
		if err == nil {
			fmt.Println("Limit order of " + "|" + strconv.Itoa(qty) + " " + symbol + " " + side + "|" + " completed.")
		} else {
			fmt.Println("Order of " + "|" + strconv.Itoa(qty) + " " + symbol + " " + side + "|" + " did not go through.")
		}
		alpacaClient.lastOrder = order.ID
		return err
	}
	fmt.Println("Quantity is <= 0, order order of " + strconv.Itoa(qty) + " " + symbol + " " + side + " not sent")
	return nil
}

// Submit a market order if quantity is above 0.
func (alp alpacaClientContainer) submitMarketOrder(qty int, symbol string, side string) error {
	account, _ := alp.client.GetAccount()
	if qty > 0 {
		adjSide := alpaca.Side(side)
		lastOrder, err := alp.client.PlaceOrder(alpaca.PlaceOrderRequest{
			AccountID:   account.ID,
			AssetKey:    &symbol,
			Qty:         decimal.NewFromFloat(float64(qty)),
			Side:        adjSide,
			Type:        "market",
			TimeInForce: "day",
		})
		if err == nil {
			fmt.Println("Market order of " + "|" + strconv.Itoa(qty) + " " + symbol + " " + side + "|" + " completed.")
		} else {
			fmt.Println("Order of " + "|" + strconv.Itoa(qty) + " " + symbol + " " + side + "|" + " did not go through.")
		}
		alpacaClient.lastOrder = lastOrder.ID
		return err
	}
	fmt.Println("Quantity is <= 0, order of " + strconv.Itoa(qty) + " " + symbol + " " + side + " not completed")
	return nil
}
