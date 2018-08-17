# alpaca-trade-api-go

`alpaca-trade-api-go` is a Go library for the Alpaca trade API. It allows rapid trading algo development easily, with support for the both REST and streaming interfaces. For details of each API behavior, please see the online API document.

## Installation

```
$ go get github.com/alpacahq/alpaca-trade-api-go
```

## Example

In order to call Alpaca's trade API, you need to obtain an API key pair. Replace <key_id> and <secret_key> with what you get from the web console.

### REST example

```go
import (
    "os"
    "fmt"

    "github.com/alpacahq/alpaca-trade-api-go/alpaca"
    "github.com/alpacahq/alpaca-trade-api-go/common"
)

func init() {
    os.Setenv(common.EnvApiKeyID, "<key_id>")
    os.Setenv(common.EnvApiSecretKey, "<secret_key>")

    fmt.Printf("Running w/ credentials [%v %v]\n", common.Credentials().ID, common.Credentials().Secret)
}

func main() {
    acct, err := alpaca.GetAccount()
    if err != nil {
        panic(err)
    }

    fmt.Println(*acct)
}
```

### Streaming example

The SDK provides a unified streaming interface for both Polygon data updates, and Alpaca's trade/account updates. The following example subscribes to trade updates, and prints any messages received, and subscribes to live quotes for AAPL, and prints any quotes received. The main function also ends with an empty `select{}` statement which causes the program to run indefinitely.

```go
import (
    "os"
    "fmt"

    "github.com/alpacahq/alpaca-trade-api-go/alpaca"
    "github.com/alpacahq/alpaca-trade-api-go/polygon"
    "github.com/alpacahq/alpaca-trade-api-go/stream"
    nats "github.com/nats-io/go-nats"
)

func main() {
    os.Setenv(common.EnvApiKeyID, "<key_id>")
    os.Setenv(common.EnvApiSecretKey, "<secret_key>")

    if err := stream.Register(alpaca.TradeUpdates, tradeHandler); err != nil {
        panic(err)
    }

    if err := stream.Register("Q.AAPL", quoteHandler); err != nil {
        panic(err)
    }

    select{}
}

func tradeHandler(msg interface{}) {
    fmt.Println(msg)
}

func quoteHandler(msg interface{}) {
    quote := polygon.StreamQuote{}

    if err := json.Unmarshal(msg.(*nats.Msg).Data, &quote); err != nil {
        panic(err)
    }

    fmt.Println(quote.Symbol, quote.BidPrice, quote.BidSize, quote.AskPrice, quote.AskSize)
}
```

## API Document

The HTTP API document is located at https://docs.alpaca.markets/

## Authentication

The Alpaca API requires API key ID and secret key, which you can obtain from the web console after you sign in. This key pair can then be applied to the SDK either by setting environment variables (`APCA_API_KEY_ID=<key_id>` and `APCA_API_SECRET_KEY=<secret_key>`), or hardcoding them into the Go code directly as shown in the examples above.

## GoDoc

For a more in-depth look at the SDK, see the [GoDoc](https://godoc.org/github.com/alpacahq/alpaca-trade-api-go)