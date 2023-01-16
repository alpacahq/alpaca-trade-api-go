package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

type longShortAlgo struct {
	tradeClient *alpaca.Client
	dataClient  *marketdata.Client
	long        bucket
	short       bucket
	allStocks   []stockField
	blacklist   []string
}

type bucket struct {
	list        []string
	qty         int
	adjustedQty int
	equityAmt   float64
}

type stockField struct {
	name string
	pc   float64
}

var algo longShortAlgo

// Set this to true if you have unlimited subscription!
var hasSipAccess bool = false

func init() {
	// You can set your API key/secret here or you can use environment variables!
	apiKey := ""
	apiSecret := ""
	// Change baseURL to https://paper-api.alpaca.markets if you want use paper!
	baseURL := ""

	// Format the allStocks variable for use in the class.
	allStocks := []stockField{}
	stockList := []string{"DOMO", "SQ", "MRO", "AAPL", "GM", "SNAP", "SHOP", "SPLK", "BA", "AMZN", "SUI", "SUN", "TSLA", "CGC", "SPWR", "NIO", "CAT", "MSFT", "PANW", "OKTA", "TWTR", "TM", "GE", "ATVI", "GS", "BAC", "MS", "TWLO", "QCOM", "IBM"}
	for _, stock := range stockList {
		allStocks = append(allStocks, stockField{stock, 0})
	}

	algo = longShortAlgo{
		tradeClient: alpaca.NewClient(alpaca.ClientOpts{
			APIKey:    apiKey,
			APISecret: apiSecret,
			BaseURL:   baseURL,
		}),
		dataClient: marketdata.NewClient(marketdata.ClientOpts{
			APIKey:    apiKey,
			APISecret: apiSecret,
		}),
		long: bucket{
			qty:         -1,
			adjustedQty: -1,
		},
		short: bucket{
			qty:         -1,
			adjustedQty: -1,
		},
		allStocks: allStocks,
		blacklist: []string{},
	}
}

func main() {
	fmt.Print("Cancelling all open orders so they don't impact our buying power... ")
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
			log.Fatalf("Failed to cancel order %s: %v", order.ID, err)
		}
	}
	fmt.Printf("%d order(s) cancelled\n", len(orders))

	for {
		isOpen, err := algo.awaitMarketOpen()
		if err != nil {
			log.Fatalf("Failed to wait for market open: %v", err)
		}
		if !isOpen {
			time.Sleep(1 * time.Minute)
			continue
		}
		if err := algo.run(); err != nil {
			log.Fatalf("Run error: %v", err)
		}
	}
}

// Rebalance the portfolio every minute, making necessary trades.
func (alp longShortAlgo) run() error {
	// Figure out when the market will close so we can prepare to sell beforehand.
	clock, err := algo.tradeClient.GetClock()
	if err != nil {
		return fmt.Errorf("get clock: %w", err)
	}
	if clock.NextClose.Sub(clock.Timestamp) < 15*time.Minute {
		// Close all positions when 15 minutes til market close.
		fmt.Println("Market closing soon. Closing positions")

		positions, err := algo.tradeClient.GetPositions()
		if err != nil {
			return fmt.Errorf("get positions: %w", err)
		}
		for _, position := range positions {
			var orderSide string
			if position.Side == "long" {
				orderSide = "sell"
			} else {
				orderSide = "buy"
			}
			qty, _ := position.Qty.Float64()
			qty = math.Abs(qty)
			if err := algo.submitOrder(int(qty), position.Symbol, orderSide); err != nil {
				return fmt.Errorf("submit order: %w", err)
			}
		}
		// Run script again after market close for next trading day.
		fmt.Println("Sleeping until market close (15 minutes)")
		time.Sleep(15 * time.Minute)
	} else {
		// Rebalance the portfolio.
		if err := algo.rebalance(); err != nil {
			fmt.Println("Failed to rebalance, will try again in a minute:", err)
		}
		fmt.Println("Sleeping for 1 minute")
		time.Sleep(1 * time.Minute)
	}
	return nil
}

// Spin until the market is open.
func (alp longShortAlgo) awaitMarketOpen() (bool, error) {
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
func (alp longShortAlgo) rebalance() error {
	if err := algo.rerank(); err != nil {
		return fmt.Errorf("rerank: %w", err)
	}

	fmt.Printf("We are taking a long position in: %v\n", algo.long.list)
	fmt.Printf("We are taking a short position in: %v\n", algo.short.list)

	// Clear existing orders again.
	orders, err := algo.tradeClient.GetOrders(alpaca.GetOrdersRequest{
		Status: "open",
		Until:  time.Now(),
		Limit:  100,
	})
	if err != nil {
		return fmt.Errorf("list orders: %w", err)
	}
	for _, order := range orders {
		if err := algo.tradeClient.CancelOrder(order.ID); err != nil {
			return fmt.Errorf("cancel order %s: %w", order.ID, err)
		}
	}

	// Remove positions that are no longer in the short or long list, and make a list of positions that do not need to change.  Adjust position quantities if needed.
	algo.blacklist = nil
	var executed [2][]string
	positions, err := algo.tradeClient.GetPositions()
	if err != nil {
		return fmt.Errorf("list positions: %w", err)
	}
	for _, position := range positions {
		indLong := indexOf(algo.long.list, position.Symbol)
		indShort := indexOf(algo.short.list, position.Symbol)

		rawQty, _ := position.Qty.Float64()
		qty := int(math.Abs(rawQty))
		side := "buy"
		if indLong < 0 {
			// Position is not in long list.
			if indShort < 0 {
				// Position not in short list either.  Clear position.
				if position.Side == "long" {
					side = "sell"
				} else {
					side = "buy"
				}
				if err := algo.submitOrder(int(math.Abs(float64(qty))), position.Symbol, side); err != nil {
					return fmt.Errorf("submit order for %d %s: %w", qty, position.Symbol, err)
				}
			} else {
				if position.Side == "long" {
					// Position changed from long to short.  Clear long position to prep for short sell.
					side = "sell"
					if err := algo.submitOrder(qty, position.Symbol, side); err != nil {
						return fmt.Errorf("submit order for %d %s: %w", qty, position.Symbol, err)
					}
				} else {
					// Position in short list
					if qty == algo.short.qty {
						// Position is where we want it.  Pass for now
					} else {
						// Need to adjust position amount.
						diff := qty - algo.short.qty
						if diff > 0 {
							// Too many short positions.  Buy some back to rebalance.
							side = "buy"
						} else {
							// Too little short positions.  Sell some more.
							diff = int(math.Abs(float64(diff)))
							side = "sell"
						}
						qty = diff
						if err := algo.submitOrder(qty, position.Symbol, side); err != nil {
							return fmt.Errorf("submit order for %d %s: %w", qty, position.Symbol, err)
						}
					}
					executed[1] = append(executed[1], position.Symbol)
					algo.blacklist = append(algo.blacklist, position.Symbol)
				}
			}
		} else {
			// Position in long list.
			if position.Side == "short" {
				// Position changed from short to long.  Clear short position to prep for long purchase.
				side = "buy"
				if err := algo.submitOrder(qty, position.Symbol, side); err != nil {
					return fmt.Errorf("submit order for %d %s: %w", qty, position.Symbol, err)
				}
			} else {
				if qty == algo.long.qty {
					// Position is where we want it.  Pass for now.
				} else {
					// Need to adjust position amount
					diff := qty - algo.long.qty
					if diff > 0 {
						// Too many long positions.  Sell some to rebalance.
						side = "sell"
					} else {
						diff = int(math.Abs(float64(diff)))
						side = "buy"
					}
					qty = diff
					if err := algo.submitOrder(qty, position.Symbol, side); err != nil {
						return fmt.Errorf("submit order for %d %s: %w", qty, position.Symbol, err)
					}
				}
				executed[0] = append(executed[0], position.Symbol)
				algo.blacklist = append(algo.blacklist, position.Symbol)
			}
		}
	}

	// Send orders to all remaining stocks in the long and short list.
	longBOResp := algo.sendBatchOrder(algo.long.qty, algo.long.list, "buy")
	executed[0] = append(executed[0], longBOResp[0][:]...)
	if len(longBOResp[1][:]) > 0 {
		// Handle rejected/incomplete orders and determine new quantities to purchase.

		longTPResp, err := algo.getTotalPrice(executed[0])
		if err != nil {
			return fmt.Errorf("get total long price: %w", err)
		}
		if longTPResp > 0 {
			algo.long.adjustedQty = int(algo.long.equityAmt / longTPResp)
		} else {
			algo.long.adjustedQty = -1
		}
	} else {
		algo.long.adjustedQty = -1
	}

	shortBOResp := algo.sendBatchOrder(algo.short.qty, algo.short.list, "sell")
	executed[1] = append(executed[1], shortBOResp[0][:]...)
	if len(shortBOResp[1][:]) > 0 {
		// Handle rejected/incomplete orders and determine new quantities to purchase.
		shortTPResp, err := algo.getTotalPrice(executed[1])
		if err != nil {
			return fmt.Errorf("get total short price: %w", err)
		}
		if shortTPResp > 0 {
			algo.short.adjustedQty = int(algo.short.equityAmt / shortTPResp)
		} else {
			algo.short.adjustedQty = -1
		}
	} else {
		algo.short.adjustedQty = -1
	}

	// Reorder stocks that didn't throw an error so that the equity quota is reached.
	if algo.long.adjustedQty > -1 {
		algo.long.qty = algo.long.adjustedQty - algo.long.qty
		for _, stock := range executed[0] {
			if err := algo.submitOrder(algo.long.qty, stock, "buy"); err != nil {
				return fmt.Errorf("submit order for %d %s: %w", algo.long.qty, stock, err)
			}
		}
	}

	if algo.short.adjustedQty > -1 {
		algo.short.qty = algo.short.adjustedQty - algo.short.qty
		for _, stock := range executed[1] {
			if err := algo.submitOrder(algo.short.qty, stock, "sell"); err != nil {
				return fmt.Errorf("submit order for %d %s: %w", algo.long.qty, stock, err)
			}
		}
	}

	return nil
}

// Re-rank all stocks to adjust longs and shorts.
func (alp longShortAlgo) rerank() error {
	if err := algo.rank(); err != nil {
		return err
	}

	// Grabs the top and bottom quarter of the sorted stock list to get the long and short lists.
	longShortAmount := int(len(algo.allStocks) / 4)
	algo.long.list = nil
	algo.short.list = nil

	for i, stock := range algo.allStocks {
		if i < longShortAmount {
			algo.short.list = append(algo.short.list, stock.name)
		} else if i > (len(algo.allStocks) - 1 - longShortAmount) {
			algo.long.list = append(algo.long.list, stock.name)
		} else {
			continue
		}
	}

	// Determine amount to long/short based on total stock price of each bucket.
	account, err := algo.tradeClient.GetAccount()
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}
	equity, _ := account.Cash.Float64()
	positions, err := algo.tradeClient.GetPositions()
	if err != nil {
		return fmt.Errorf("list positions: %w", err)
	}
	for _, position := range positions {
		rawVal, _ := position.MarketValue.Float64()
		equity += rawVal
	}

	algo.short.equityAmt = equity * 0.30
	algo.long.equityAmt = equity + algo.short.equityAmt

	longTotal, err := algo.getTotalPrice(algo.long.list)
	if err != nil {
		return fmt.Errorf("get total long price: %w", err)
	}
	shortTotal, err := algo.getTotalPrice(algo.short.list)
	if err != nil {
		return fmt.Errorf("get total short price: %w", err)
	}

	algo.long.qty = int(algo.long.equityAmt / longTotal)
	algo.short.qty = int(algo.short.equityAmt / shortTotal)

	return nil
}

// Get the total price of the array of input stocks.
func (alp longShortAlgo) getTotalPrice(arr []string) (float64, error) {
	totalPrice := 0.0
	snapshots, err := algo.dataClient.GetSnapshots(arr, marketdata.GetSnapshotRequest{})
	if err != nil {
		return 0, fmt.Errorf("get snapshots: %w", err)
	}
	for symbol, snapshot := range snapshots {
		if snapshot == nil {
			return 0, fmt.Errorf("no snapshot for %s", symbol)
		}
		if snapshot.MinuteBar == nil {
			return 0, fmt.Errorf("no price for %s", symbol)
		}
		totalPrice += snapshot.MinuteBar.Close
	}
	return totalPrice, nil
}

// Submit an order if quantity is above 0.
func (alp longShortAlgo) submitOrder(qty int, symbol string, side string) error {
	if qty > 0 {
		adjSide := alpaca.Side(side)
		decimalQty := decimal.NewFromInt(int64(qty))
		_, err := algo.tradeClient.PlaceOrder(alpaca.PlaceOrderRequest{
			Symbol:      symbol,
			Qty:         &decimalQty,
			Side:        adjSide,
			Type:        "market",
			TimeInForce: "day",
		})
		if err == nil {
			fmt.Printf("Market order of | %d %s %s | completed\n", qty, symbol, side)
		} else {
			fmt.Printf("Order of | %d %s %s | did not go through: %s\n", qty, symbol, side, err)
		}
		return err
	}
	fmt.Printf("Quantity is <= 0, order of | %d %s %s | not sent\n", qty, symbol, side)
	return nil
}

// Submit a batch order that returns completed and uncompleted orders.
func (alp longShortAlgo) sendBatchOrder(qty int, stocks []string, side string) [2][]string {
	var executed []string
	var incomplete []string
	for _, stock := range stocks {
		index := indexOf(algo.blacklist, stock)
		if index == -1 {
			err := algo.submitOrder(qty, stock, side)
			if err != nil {
				incomplete = append(incomplete, stock)
			} else {
				executed = append(executed, stock)
			}
		}
	}
	return [2][]string{executed, incomplete}
}

// Get percent changes of the stock prices over the past 10 minutes.
func (alp longShortAlgo) getPercentChanges() error {
	symbols := make([]string, len(alp.allStocks))
	for i, stock := range algo.allStocks {
		symbols[i] = stock.name
	}

	end := time.Now()
	start := end.Add(-10 * time.Minute)
	feed := ""
	if !hasSipAccess {
		feed = "iex"
	}
	multiBars, err := algo.dataClient.GetMultiBars(symbols, marketdata.GetBarsRequest{
		TimeFrame: marketdata.OneMin,
		Start:     start,
		End:       end,
		Feed:      feed,
	})
	if err != nil {
		return fmt.Errorf("get multi bars: %w", err)
	}

	for i, symbol := range symbols {
		bars := multiBars[symbol]
		if len(bars) != 0 {
			percentChange := (bars[len(bars)-1].Close - bars[0].Open) / bars[0].Open
			algo.allStocks[i].pc = float64(percentChange)
		}
	}

	return nil
}

// Mechanism used to rank the stocks, the basis of the Long-Short Equity Strategy.
func (alp longShortAlgo) rank() error {
	// Ranks all stocks by percent change over the past 10 days (higher is better).
	if err := algo.getPercentChanges(); err != nil {
		return err
	}

	// Sort the stocks in place by the percent change field (marked by pc).
	sort.Slice(algo.allStocks, func(i, j int) bool {
		return algo.allStocks[i].pc < algo.allStocks[j].pc
	})

	return nil
}

// Helper method to imitate the indexOf array method.
func indexOf(arr []string, str string) int {
	for i, elem := range arr {
		if elem == str {
			return i
		}
	}
	return -1
}
