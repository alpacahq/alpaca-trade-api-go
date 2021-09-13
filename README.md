
# alpaca-trade-api-go

[![CircleCI Status](https://circleci.com/gh/alpacahq/alpaca-trade-api-go.svg?style=svg)](https://circleci.com/gh/alpacahq/alpaca-trade-api-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/alpacahq/alpaca-trade-api-go)](https://goreportcard.com/report/github.com/alpacahq/alpaca-trade-api-go)

`alpaca-trade-api-go` is a Go library for the Alpaca trade and marketdata API. It allows rapid
trading algo development easily, with support for the both REST and streaming interfaces.
For details of each API behavior, please see the online API document.

## Installation

```bash
go get -u github.com/alpacahq/alpaca-trade-api-go/v2
```

## Examples

In order to call Alpaca's trade API, you need to obtain an API key pair from the web console.

### Trading REST example

```go
package main

import (
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
)

func main() {
	client := alpaca.NewClient(alpaca.ClientOpts{
		// Alternatively you can set your key and secret using the
		// APCA_API_KEY_ID and APCA_API_SECRET_KEY environment variables
		ApiKey:    "YOUR_API_KEY",
		ApiSecret: "YOUR_API_SECRET",
		BaseURL:   "https://paper-api.alpaca.markets",
	})
	acct, err := client.GetAccount()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", *acct)
}
```

### Trade updates stream example

The following example shows how you can stream your own trade updates.
First we register a handler function that simply prints the received trade updates,
then we submit a single AAPL buy order. You should see two updates, a "new" event
as soon as you submit the order, and a "fill" event soon after that, provided that
the market is open.

```go
// Listen to trade updates in the background (with unlimited reconnect)
go func() {
	for {
		if err := alpaca.StreamTradeUpdates(context.TODO(), func(tu alpaca.TradeUpdate) {
			log.Printf("TRADE UPDATE: %+v\n", tu)
		}); err != nil {
			log.Print("trade updates error", err)
		}
		time.Sleep(time.Second)
	}
}()

// Send a single AAPL order
symbol := "AAPL"
qty := decimal.NewFromInt(1)
if _, err := alpaca.PlaceOrder(alpaca.PlaceOrderRequest{
	AssetKey:    &symbol,
	Qty:         &qty,
	Side:        "buy",
	Type:        "market",
	TimeInForce: "day",
}); err != nil {
	log.Fatalf("failed place order: %v", err)
}
log.Println("order sent")

select {}
```

### Further examples

See the [examples](https://github.com/alpacahq/alpaca-trade-api-go/tree/master/examples)
directory for further examples:

- algo-trading examples
  - long-short
  - martingale
  - mean-reversion
- marketdata examples
  - crypto-stream
  - data-stream
  - marketdata

## API Document

The HTTP API document is located [here](https://alpaca.markets/docs/api-documentation/).

## Authentication

The Alpaca API requires API key ID and secret key, which you can obtain from
the web console after you sign in. This key pair can then be applied to the SDK
either by setting environment variables (`APCA_API_KEY_ID=<key_id>` and `APCA_API_SECRET_KEY=<secret_key>`),
or hardcoding them into the Go code directly as shown in the examples above.

```sh
export APCA_API_KEY_ID=xxxxx
export APCA_API_SECRET_KEY=yyyyy
```

## Endpoint

For paper trading, set the environment variable `APCA_API_BASE_URL` or set the
`BaseURL` option when constructing the client.

```sh
export APCA_API_BASE_URL=https://paper-api.alpaca.markets
```

## GoDoc

For a more in-depth look at the SDK, see the
[GoDoc](https://pkg.go.dev/github.com/alpacahq/alpaca-trade-api-go)
