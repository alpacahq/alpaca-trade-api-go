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
	os.Setenv(common.EnvApiKeyID, "PKM4OPMZT6GJFCLF9ZCB")
	os.Setenv(common.EnvApiSecretKey, "IxOUZCVXnB/8pfVt/idXPaI6qVw/7pEBMxacHSpK")
	alpaca.SetBaseUrl("https://paper-api.alpaca.markets")

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
	cAMO := make(chan bool)
	fmt.Println("Waiting for market to open...")
	for {
		go alpacaClient.awaitMarketOpen(cAMO)
		if <-cAMO {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
	fmt.Println("Market Opened.")

	for {
		cRun := make(chan bool)
		go alpacaClient.run(cRun)
		<-cRun
		fmt.Println("End")
	}
}

// Rebalance our portfolio every minute based off running average data
func (alp alpacaClientContainer) run(cRun chan bool) {
	if alpacaClient.lastOrder != "" {
		err := alp.client.CancelOrder(alpacaClient.lastOrder)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Rebalance the portfolio
	cRebalance := make(chan bool)
	go alp.rebalance(cRebalance)
	<-cRebalance

	// Figure out when the market will close so we can prepare to sell beforehand.
	clock, _ := alp.client.GetClock()
	timeToClose := int((clock.NextClose.UnixNano() - clock.Timestamp.UnixNano()) / 1000000)
	if timeToClose < 60000*15 {
		// Close all positions when 15 minutes til market close
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
			cSubmitMO := make(chan error)
			go alp.submitMarketOrder(int(qty), position.Symbol, orderSide, cSubmitMO)
			<-cSubmitMO
			// Run script again after market close for next trading day
			time.Sleep((60000 * 15) * time.Millisecond)
		}
	} else {
		time.Sleep(60000 * time.Millisecond)
	}
	cRun <- true
}

// Spin until the market is open
func (alp alpacaClientContainer) awaitMarketOpen(cAMO chan bool) {
	clock, _ := alp.client.GetClock()
	if clock.IsOpen {
		cAMO <- true
	} else {
		fmt.Println("spinning")
		cAMO <- true
	}
	cAMO <- true
	return
}

// Rebalance our position after an update
func (alp alpacaClientContainer) rebalance(cRebalance chan bool) {
	// Get our position, if any
	positionQty := 0
	positionVal := 0.0
	position, err := alp.client.GetPosition(alpacaClient.stock)
	if err != nil {
		fmt.Println(err)
	} else {
		positionQty = int(position.Qty.IntPart())
		positionVal, _ = position.MarketValue.Float64()
	}

	// Get the new updated price and running average
	bars, err := alp.client.GetSymbolBars(alpacaClient.stock, alpaca.ListBarParams{Timeframe: "minute"})
	if err != nil {
		fmt.Println(err)
	}
	currPrice := float64(bars[len(bars)-1].Close)
	alpacaClient.closingPrices = append(alpacaClient.closingPrices, currPrice)
	if len(alpacaClient.closingPrices) > 20 {
		alpacaClient.closingPrices = alpacaClient.closingPrices[1:]
	}
	alpacaClient.runningAverage = ((alpacaClient.runningAverage * float64(len(alpacaClient.closingPrices)-1)) + currPrice) / float64(len(alpacaClient.closingPrices))

	if currPrice > alpacaClient.runningAverage {
		// Sell our position if the price is above the running average, if any
		if positionQty > 0 {
			fmt.Println("Setting position to zero")
			cSubmitLO := make(chan error)
			go alp.submitLimitOrder(positionQty, alpacaClient.stock, currPrice, "sell", cSubmitLO)
			<-cSubmitLO
		} else {
			fmt.Println("No position in the stock.  No action required.")
		}
	} else if currPrice < alpacaClient.runningAverage {
		// Determine optimal amount of shares based on portfolio and market data
		account, err := alp.client.GetAccount()
		if err != nil {
			fmt.Println(err)
		}
		buyingPower, _ := account.BuyingPower.Float64()
		positions, err := alp.client.ListPositions()
		if err != nil {
			fmt.Println(err)
		}
		portfolioVal := 0.0
		for _, position := range positions {
			rawVal, _ := position.MarketValue.Float64()
			portfolioVal += rawVal
		}
		portfolioShare := (alpacaClient.runningAverage - currPrice) / currPrice * 200
		targetPositionValue := portfolioVal * portfolioShare
		amountToAdd := targetPositionValue - positionVal

		// Add to our position, constrained by our buying power; or, sell down to optimal amount of shares
		if amountToAdd > 0 {
			if amountToAdd > buyingPower {
				amountToAdd = buyingPower
			}
			var qtyToBuy = int(amountToAdd / currPrice)
			cSubmitLO := make(chan error)
			go alp.submitLimitOrder(qtyToBuy, alpacaClient.stock, currPrice, "buy", cSubmitLO)
			<-cSubmitLO
		} else {
			amountToAdd *= -1
			var qtyToSell = int(amountToAdd / currPrice)
			if qtyToSell > positionQty {
				qtyToSell = positionQty
			}
			cSubmitLO := make(chan error)
			go alp.submitLimitOrder(qtyToSell, alpacaClient.stock, currPrice, "buy", cSubmitLO)
			<-cSubmitLO
		}
	}
}

// Submit a limit order if quantity is above 0
func (alp alpacaClientContainer) submitLimitOrder(qty int, symbol string, price float64, side string, cSubmitLO chan error) {
	account, _ := alp.client.GetAccount()
	if qty > 0 {
		adjSide := alpaca.Side(side)
		lastOrder, err := alp.client.PlaceOrder(alpaca.PlaceOrderRequest{
			AccountID:   account.ID,
			AssetKey:    &symbol,
			Qty:         decimal.NewFromFloat(float64(qty)),
			Side:        adjSide,
			Type:        "limit",
			TimeInForce: "day",
		})
		fmt.Println("Limit order of " + strconv.Itoa(qty) + " " + symbol + ", " + side)
		alpacaClient.lastOrder = lastOrder.ID
		cSubmitLO <- err
	} else {
		fmt.Println("Quantity is 0, order of " + strconv.Itoa(qty) + " " + symbol + " " + side + " not sent")
		cSubmitLO <- nil
	}
	return
}

// Submit a market order if quantity is above 0
func (alp alpacaClientContainer) submitMarketOrder(qty int, symbol string, side string, cSubmitMO chan error) {
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
		fmt.Println("Market order of " + strconv.Itoa(qty) + " " + symbol + ", " + side)
		alpacaClient.lastOrder = lastOrder.ID
		cSubmitMO <- err
	} else {
		fmt.Println("Quantity is 0, order of " + strconv.Itoa(qty) + " " + symbol + " " + side + " not completed")
		cSubmitMO <- nil
	}
	return
}
