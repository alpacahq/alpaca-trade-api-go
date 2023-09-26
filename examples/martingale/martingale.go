package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
)

type alpacaClientContainer struct {
	client        *alpaca.Client
	tickSize      int
	tickIndex     int
	baseBet       float64
	currStreak    streak
	currOrder     string
	lastPrice     float64
	lastTradeTime time.Time
	stock         string
	position      int64
	equity        float64
	marginMult    float64
	seconds       int
}

type streak struct {
	start      float64
	count      int
	increasing bool
}

var alpacaClient alpacaClientContainer

// The MartingaleTrader bets that streaks of increases or decreases in a stock's
// price are likely to break, and increases its bet each time it is wrong.
func init() {
	// You can set your API key/secret here or you can use environment variables!
	apiKey := ""
	apiSecret := ""
	// Change this to https://api.alpaca.markets if you want to go live!
	baseURL := "https://paper-api.alpaca.markets"

	// Check if user input a stock
	stock := "AAPL"
	if len(os.Args[1:]) == 1 {
		stock = os.Args[1]
	}

	client := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
		BaseURL:   baseURL,
	})

	// Cancel any open orders so they don't interfere with this script
	client.CancelAllOrders()

	pos, err := client.GetPosition(stock)
	position := int64(0)
	if err != nil {
		// No position exists
	} else {
		position = pos.Qty.IntPart()
	}

	// Figure out how much money we have to work with, accounting for margin
	acct, err := client.GetAccount()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Initial total buying power = %s\n", acct.Equity.Mul(acct.Multiplier).StringFixed(2))

	alpacaClient = alpacaClientContainer{
		client,
		5,
		4,
		.1,
		streak{
			0,
			0,
			true,
		},
		"",
		0,
		time.Now().UTC(),
		stock,
		position,
		acct.Equity.InexactFloat64(),
		acct.Multiplier.InexactFloat64(),
		0,
	}
}

func main() {
	// First, cancel any existing orders so they don't impact our buying power
	orders, _ := alpacaClient.client.GetOrders(alpaca.GetOrdersRequest{
		Status: "open",
		Until:  time.Now(),
		Limit:  100,
	})
	for _, order := range orders {
		_ = alpacaClient.client.CancelOrder(order.ID)
	}

	feed := "iex" // Use sip if you have proper subscription
	c := stream.NewStocksClient(feed)
	if err := c.Connect(context.TODO()); err != nil {
		panic(err)
	}
	c.SubscribeToTrades(handleTrades, alpacaClient.stock)

	alpacaClient.client.StreamTradeUpdatesInBackground(context.TODO(), handleTradeUpdates)

	if err := <-c.Terminated(); err != nil {
		panic(err)
	}
}

// Listen for trade data and perform trading logic
func handleTrades(trade stream.Trade) {
	if trade.Symbol != alpacaClient.stock {
		return
	}

	now := time.Now().UTC()
	if now.Sub(alpacaClient.lastTradeTime) < time.Second {
		// don't react every tick unless at least 1 second past
		return
	}
	alpacaClient.lastTradeTime = now

	alpacaClient.tickIndex = (alpacaClient.tickIndex + 1) % alpacaClient.tickSize
	if alpacaClient.tickIndex == 0 {
		// It's time to update

		// Update price info
		tickOpen := alpacaClient.lastPrice
		tickClose := trade.Price
		alpacaClient.lastPrice = tickClose

		alpacaClient.processTick(tickOpen, tickClose)
	}
}

// Listen for updates to our orders
func handleTradeUpdates(tu alpaca.TradeUpdate) {
	fmt.Printf("%s event received for order %s.\n", tu.Event, tu.Order.ID)

	if tu.Order.Symbol != alpacaClient.stock {
		// The order was for a position unrelated to this script
		return
	}

	eventType := tu.Event
	oid := tu.Order.ID

	if eventType == "fill" || eventType == "partial_fill" {
		// Our position size has changed
		pos, err := alpacaClient.client.GetPosition(alpacaClient.stock)
		if err != nil {
			alpacaClient.position = 0
		} else {
			alpacaClient.position = pos.Qty.IntPart()
		}

		fmt.Printf("New position size due to order fill: %d\n", alpacaClient.position)
		if eventType == "fill" && alpacaClient.currOrder == oid {
			alpacaClient.currOrder = ""
		}
	} else if eventType == "rejected" || eventType == "canceled" {
		if alpacaClient.currOrder == oid {
			// Our last order should be removed
			alpacaClient.currOrder = ""
		}
	} else if eventType == "new" {
		alpacaClient.currOrder = oid
	} else {
		fmt.Printf("Unexpected order event type %s received\n", eventType)
	}
}

func (alp alpacaClientContainer) processTick(tickOpen float64, tickClose float64) {
	// Update streak info
	diff := tickClose - tickOpen
	if math.Abs(diff) >= .01 {
		// There was a meaningful change in the price
		alp.currStreak.count++
		increasing := tickOpen > tickClose
		if alp.currStreak.increasing != increasing {
			// It moved in the opposite direction of the streak.
			// Therefore, the streak is over, and we should reset.

			// Empty out the position
			if alp.position != 0 {
				_, err := alp.sendOrder(0)
				if err != nil {
					panic(err)
				}
			}

			// Reset variables
			alp.currStreak.increasing = increasing
			alp.currStreak.start = tickOpen
			alp.currStreak.count = 0
		} else {
			// Calculate the number of shares we want to be holding
			totalBuyingPower := alp.equity * alp.marginMult
			targetValue := math.Pow(2, float64(alp.currStreak.count)) * alp.baseBet * totalBuyingPower
			if targetValue > totalBuyingPower {
				// Limit the amount we can buy to a bit (1 share)
				// less than our total buying power
				targetValue = totalBuyingPower - alp.lastPrice
			}
			targetQty := int(targetValue / alp.lastPrice)
			if alp.currStreak.increasing {
				targetQty = -targetQty
			}

			// We don't want to have two orders open at once
			if int64(targetQty)-alp.position != 0 {
				if alpacaClient.currOrder != "" {
					err := alp.client.CancelOrder(alpacaClient.currOrder)
					if err != nil {
						panic(err)
					}

					alpacaClient.currOrder = ""
				}

				_, err := alp.sendOrder(targetQty)
				if err != nil {
					panic(err)
				}
			}
		}
	}

	// Update our account balance
	acct, err := alp.client.GetAccount()
	if err != nil {
		panic(err)
	}

	alp.equity, _ = acct.Equity.Float64()
}

func (alp alpacaClientContainer) sendOrder(targetQty int) (string, error) {
	delta := float64(int64(targetQty) - alp.position)

	fmt.Printf("Ordering towards %d...\n", targetQty)

	qty := float64(0)
	side := alpaca.Side("")

	if delta > 0 {
		side = alpaca.Buy
		qty = delta
		if alp.position < 0 {
			qty = math.Min(math.Abs(float64(alp.position)), qty)
		}
		fmt.Printf("Buying %d shares.\n", int64(qty))

	} else if delta < 0 {
		side = alpaca.Sell
		qty = math.Abs(delta)
		if alp.position > 0 {
			qty = math.Min(math.Abs(float64(alp.position)), qty)
		}
		fmt.Printf("Selling %d shares.\n", int64(qty))
	}

	if qty > 0 {
		alp.currOrder = randomString()
		decimalQty := decimal.NewFromFloat(qty)
		alp.client.PlaceOrder(alpaca.PlaceOrderRequest{
			Symbol:        alp.stock,
			Qty:           &decimalQty,
			Side:          side,
			Type:          alpaca.Limit,
			LimitPrice:    alpaca.RoundLimitPrice(decimal.NewFromFloat(alp.lastPrice), side),
			TimeInForce:   alpaca.Day,
			ClientOrderID: alp.currOrder,
		})

		return alp.currOrder, nil
	}

	return "", errors.New("Non-positive quantity given")
}

func randomString() string {
	rand.Seed(time.Now().Unix())
	characters := "abcdefghijklmnopqrstuvwxyz"
	resSize := 10

	var output strings.Builder

	for i := 0; i < resSize; i++ {
		index := rand.Intn(len(characters))
		output.WriteString(string(characters[index]))
	}
	return output.String()
}
