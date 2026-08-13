# alpaca-trade-api-go

[![GitHub Status](https://github.com/alpacahq/alpaca-trade-api-go/actions/workflows/go.yml/badge.svg)](https://github.com/alpacahq/alpaca-trade-api-go/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/alpacahq/alpaca-trade-api-go)](https://goreportcard.com/report/github.com/alpacahq/alpaca-trade-api-go)
[![Forum](https://img.shields.io/badge/Forum-Alpaca%20Community-blue?logo=data%3Aimage%2Fpng%3Bbase64%2CiVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAGuElEQVRogcWaXYhdVxXHf2udr3vHVCmtlcYHsXaKD%2B2DVixJDdpWIT4oFakQLMUvRH1QfEkp9sEXH0qFqohFaAolgWraYEvQRiwRUjC2hSKmtEUSG6pNI5hOTZM795x99lo%2B3HNn7tzM3LlnvvKHO9y79%2Fr4r73P2nvtfUbYGFydZdntieoO3D4qcL2LXCmqVwC42XnB54CToK9G979WVXUUOLdex7JGHQfeWxTZV0C%2BnqjuaGnPAY9mxzF%2FtAzhSeD8iO1WZNriqiLLfiiJfkdFrmocrsUOQ11zP%2BfRflWG8BAwR4tA2jhOsiy7O1F9UFXez%2FqIj8MBcfd3LNq9ZQiPAvU0ilMR6NL9oOfxEVHZLSIRSNZBdhKiuyfu%2FEFK%2FfY882%2BuprBqAGma7kwTfUJVt7Oxo74SBo%2BV2Zt1tLvquj4%2BSXgimaJIPquSPiUi3UZ2s8kP4YC7exmt%2FnJVxWdWElyRUFEkn1NJj4iIAzpJdpPggLk75vXusozPLie0LKks4%2BNpUhy7DCM%2FjuFM9OpY7gqBv40LLJeM1xR5fkREP9D8bkveAHHnn7g9C1wNYO5PAkFErmX6XBIAESkSSe4o6vibCnqjAjqmUHSL%2FKci%2BpEhkZbkAUxEAHtjvqy%2Ba86t5v60Jzze75c3x2j7G7s2pb2BrMhsLPIHgXy0c8kM5EmyW5PkARGxZYJrE4C6%2B6m0js%2BUMZ4WkTMYb5vZWVU9qSI7VPRaIE7pZxjwx9Q5Ht1PDjtGlbdpkvxiMHob88xLM8p1Xb8UQngZkBDCCYd%2FNR7alA0iImia%2FhyYGTamwy9Flu3RRK9j4LTt6A92UrM3DD8I%2BqqInuzD201%2FYKSGEnxIoM1ACWCqMltk2Z4yhH2jAXQkkb3N9zbkm2T0GGt7qAzh%2FobssG9UTgFy8u0gV7bwMQoFENV7gQNAOTCaJLep6PW0m9IheY9mD5Qh7G3ahp9xRCCpqF6ro33DzF6RQQ5Om8wLflVltkiS22BgQNM8%2B7GK3NQITDutBqhF%2F2NZhW82tuJqzoHMzM6koucR%2BYyIzNC%2BRBEX8Rjj0woUInJnS%2FIOJO7%2Brog91bQp081gDWT9EA6481zT1mYWmlVGvgTkmqbpJ3QwCm3gTahnapMXW5JYCFI8HnX3HoPZa3WQUZVtaZrerKnqrjaKS2i4nw0hnBonNqU2Fv0f4gsrVasAAFLVXerCjW0dA2YxPjxfVl9g8ShoeZ7vnZnperdTnEg76c5GdrlyxQGiyH9dlpYGbeDCjanADS3Ii7udxvh%2BvwqHR8g5kEM8Vdf8MkY5FGP9FwZ5sWJiNzu%2Btz8JN%2FpwQwpsn5Y8YOby27IqDwMZi2s%2BQFVV8RDEQyNtbZfIttiuiGybQnBYi6jitzZtW3E6mwyRK1RG6orVxBulm4qs%2BBaD5XCzzsbTEppRoJpeHhOR92ni93XT9JNc5iDc%2FV119wstdBRwEb3OEz3cybK7mb4k3nCIMKfAmbZ6AKp6jabJ%2Fk6Rvdbt5PvyPP9i059O0N1QuHNOwV9fjxHVZFZEv6YidzRNW%2FZICZxWXC45KLfAQpmMy7mRti1BdJ5Xi%2FGF9RpyvHLxuY0g1QZm9qJWMT7n7uUabXjz96KIbekMuHu%2FruuXFOi525%2FW49yhZ2Zziz83FYNC0P13QE8Bi8b%2BtVob3HrJRbOtzQGv4wHAFCCE8HszO0v70sARcLxX17pVOSBm9lYV41FY3IB65vazBVLt7CHORai2YgYMwKP9BChhMQCvqvrXZvafNZkV7zG4Qpn2WLlWqJn9u6zrx4Z%2BRkuAdxy%2Fj3bXfg4gyPBQ0vpo2AIDTuI%2FAhbKnyU1TFmGx93jkaZ9miDE3XHn%2FHL2NhCDGxCPh%2Fr9cHC0Y9xh3%2Frhe%2B4LCT1pNBd2YV%2B8gdsMDE%2BCb0k%2F%2FADoj3ZeMmIlvB7q6k53D6z%2BOAi4I76ZCWzububVV%2Bfhkndmy055XfN8NL%2BLVc60AO70xDdtF45AEs33lCV%2FXk5gxWe2qqrDVsd73H1oaBxDsj3fnE0sAonV8Z6qqp5YSWhS0nk%2FhP2hjreb%2B%2F9Y6c7T6bnZuv9lYInFwWulC9HC5%2FshTKwSVl016ro%2BFqN92sz%2FztgS29zxzZvqmi%2BnxmCAmPmJJM12lmU8sprCVG9HQggv98vyljraXnfvM5yNwcuQXkhC20fIAUIIZ4GLTf1i7l7W0e7vl%2BUtFy5ceIXNuPUoCj5cFPnBbqdTvmem692iOAYMr2ba7AMJQCdNP9XN832dPH84z%2FPZDSc8AR8qivyxTp4%2F0vy%2BHO%2BS%2BT8lMTeMbm%2FRxQAAAABJRU5ErkJggg%3D%3D)](https://forum.alpaca.markets/)
[![Slack](https://img.shields.io/badge/Slack-Alpaca%20Community-4A154B?logo=slack&logoColor=white)](https://alpaca.markets/slack)

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
		// APCA_API_KEY_ID and APCA_API_SECRET_KEY environment variables
		APIKey:    "YOUR_API_KEY",
		APISecret: "YOUR_API_SECRET",
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

The Alpaca API requires API key ID and secret key, which you can obtain from
the web console after you sign in. This key pair can then be applied to the SDK
either by setting environment variables (`APCA_API_KEY_ID=<key_id>` and `APCA_API_SECRET_KEY=<secret_key>`),
or hardcoding them into the Go code directly as shown in the examples above.

```sh
export APCA_API_KEY_ID=xxxxx
export APCA_API_SECRET_KEY=yyyyy
```

### Broker auth

You use your Broker API key and secret for authentication.
However, for this to work make sure you're using the appropriate base URL
(for more details check the next section)!

```go
client := marketdata.NewClient(marketdata.ClientOpts{
	BrokerKey:    "CK...",                               // Sandbox broker key
	BrokerSecret: "<your secret>",                       // Sandbox broker secret
	BaseURL:      "https://data.sandbox.alpaca.markets", // Sandbox url
})
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

## Support

- **Library / SDK issues:** Bugs, feature requests, or questions specific to this Go library → [GitHub Issues](https://github.com/alpacahq/alpaca-trade-api-go/issues).
- **General Alpaca support & API discussion:** Account questions, platform issues, or broader API topics → [Alpaca Community Forum](https://forum.alpaca.markets/).
- **Slack community:** Chat with other developers and the Alpaca community on [Slack](https://alpaca.markets/slack).
