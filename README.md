# alpaca-trade-api-go

[![GitHub Status](https://github.com/alpacahq/alpaca-trade-api-go/actions/workflows/go.yml/badge.svg)](https://github.com/alpacahq/alpaca-trade-api-go/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/alpacahq/alpaca-trade-api-go)](https://goreportcard.com/report/github.com/alpacahq/alpaca-trade-api-go)

`alpaca-trade-api-go` is a Go library for the Alpaca trade and marketdata API. It allows rapid
trading algo development easily, with support for the both REST and streaming interfaces.
For details of each API behavior, please see the online API document.

## Installation

```bash
go get -u github.com/alpacahq/alpaca-trade-api-go/v3/alpaca
```

## Examples

In order to call Alpaca's trade API, you need to obtain an API key pair from the web console.

### Trading REST example

```go
package main

import (
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
)

func main() {
	client := alpaca.NewClient(alpaca.ClientOpts{
		// Alternatively you can set your key and secret using the
		// APCA_API_CLIENT_ID and APCA_API_CLIENT_SECRET environment variables
		ClientID:     "YOUR_CLIENT_ID",
		ClientSecret: "YOUR_CLIENT_SECRET",
		BaseURL:      "https://paper-api.alpaca.markets",
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
alpaca.StreamTradeUpdatesInBackground(context.TODO(), func(tu alpaca.TradeUpdate) {
	log.Printf("TRADE UPDATE: %+v\n", tu)
})

// Send a single AAPL order
qty := decimal.NewFromInt(1)
if _, err := alpaca.PlaceOrder(alpaca.PlaceOrderRequest{
	Symbol:      "AAPL",
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

The Alpaca API requires client id client secret, which you can obtain from
the web console after you sign in. The client credentials can then be applied to the SDK
either by setting environment variables (`APCA_API_CLIENT_ID=<client_id>` and `APCA_API_CLIENT_SECRET=<client_secret>`),
or hardcoding them into the Go code directly as shown in the examples above.

Client type has to match with the type that was used during the credentials creation (`legacy` or `client_secret`)

```sh
export APCA_API_CLIENT_ID=xxxxx
export APCA_API_CLIENT_SECRET=yyyyy
```

## Endpoint

For paper trading, set the environment variable `APCA_API_BASE_URL` or set the
`BaseURL` option when constructing the client.

```sh
export APCA_API_BASE_URL=https://paper-api.alpaca.markets
```

### Broker API

For broker partners, set the base URL to

- `broker-api.alpaca.markets` for production
- `broker-api.sandbox.alpaca.markets` for sandbox
- `data.alpaca.markets` for production marketdata
- `data.sandbox.alpaca.markets` for sandbox marketdata

## Documentation

For a more in-depth look at the SDK, see the [package documentation](https://pkg.go.dev/github.com/alpacahq/alpaca-trade-api-go/v3).
