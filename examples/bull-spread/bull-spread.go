package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

var decimal10k = decimal.NewFromInt(10000)

type Service struct {
	tdClient *alpaca.Client
	mdClient *marketdata.Client
	acct     *alpaca.Account
}

func NewService() (*Service, error) {
	// You can set your API key/secret here or you can use environment variables
	apiKey := os.Getenv("APCA_API_KEY_ID")
	apiSecret := os.Getenv("APCA_API_SECRET_KEY")
	// NOTE: mleg complex option strategies are still in beta only availabe in Paper
	baseURL := "https://paper-api.alpaca.markets"

	tdClient := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
		BaseURL:   baseURL,
	})

	mdClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})

	// Cancel any open orders so they don't interfere with this algo
	if err := tdClient.CancelAllOrders(); err != nil {
		return nil, err
	}

	// Make sure we have enough green for some mleg fun
	acct, err := tdClient.GetAccount()
	if err != nil {
		return nil, err
	}
	if acct.BuyingPower.LessThan(decimal10k) {
		return nil, fmt.Errorf("insufficient buying power: needed %s, have %s",
			decimal10k.String(), acct.BuyingPower.String())
	}

	service := Service{tdClient: tdClient, mdClient: mdClient, acct: acct}
	return &service, nil
}

func main() {
	svc, err := NewService()
	if err != nil {
		log.Fatalf("failed to intilize service: %v", err)
	}

	underlying := "INTC"
	td, err := svc.mdClient.GetLatestTrade(underlying, marketdata.GetLatestTradeRequest{})
	if err != nil {
		log.Fatalf("getting latest trade for symbol %s: %v", underlying, err)
	}
	px := td.Price

	// list contracts that are expiring at least a week from now and strike is GTE latest trade
	dt := civil.DateOf(time.Now()).AddDays(7)

	// Select two contracts for the two legs of the spread:
	// 1. long leg, strike A at latest trade
	var (
		long  alpaca.OptionContract
		short alpaca.OptionContract
	)
	req := alpaca.GetOptionContractsRequest{
		UnderlyingSymbols: underlying,
		Status:            alpaca.OptionStatusActive,
		ExpirationDateGTE: dt,
		StrikePriceGTE:    decimal.NewFromFloat(px), // strike A
		TotalLimit:        1,
	}
	contracts, err := svc.tdClient.GetOptionContracts(req)
	if err != nil {
		log.Fatalf("listing contracts: %v", err)
	}
	if len(contracts) < 1 {
		log.Fatal("no contracts to choose from for the long leg")
	}
	long = contracts[0]
	log.Printf("-> contract to be bought long: %s", long.Symbol)

	// 2. short leg, strike B at $10 above latest trade
	req = alpaca.GetOptionContractsRequest{
		UnderlyingSymbols: underlying,
		Status:            alpaca.OptionStatusActive,
		ExpirationDateGTE: dt,
		TotalLimit:        1,
	}
	req.StrikePriceGTE = decimal.NewFromFloat(px + 10) // strike B
	contracts, err = svc.tdClient.GetOptionContracts(req)
	if err != nil {
		log.Fatalf("listing contracts: %v", err)
	}
	if len(contracts) < 1 {
		log.Fatal("no contracts to choose from for the short leg")
	}
	short = contracts[0]
	log.Printf("-> contract to be shorted: %s", short.Symbol)

	// Bullish Call Spread: You profit if the stock price is between the strike (A) and the
	// higher strike (B). The maximum profit occurs when the stock is at or above strike B, but
	// the profit is capped at the difference between the two strikes minus the premium paid (cost of the spread).
	qty := decimal.NewFromInt(2) // instances of the strategy to be placed
	order, err := svc.tdClient.PlaceOrder(alpaca.PlaceOrderRequest{
		Qty:         &qty,
		TimeInForce: alpaca.Day,
		Type:        alpaca.Market,
		OrderClass:  alpaca.MLeg,
		Legs: []alpaca.Leg{
			{
				Symbol:         long.Symbol,
				Side:           alpaca.Buy,
				PositionIntent: alpaca.BuyToOpen,
				RatioQty:       decimal.NewFromInt(1),
			},
			{
				Symbol:         short.Symbol,
				Side:           alpaca.Sell,
				PositionIntent: alpaca.SellToOpen,
				RatioQty:       decimal.NewFromInt(1),
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to submit mleg order: %v", err)
	}

	log.Println("Strategy in place")
	v, err := json.MarshalIndent(order, "", "\t")
	if err != nil {
		log.Fatalf("failed to marshal order: %v", err)
	}
	log.Println(string(v))
}
