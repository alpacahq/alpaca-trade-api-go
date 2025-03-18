package main

import (
	"fmt"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

func main() {
	ec, err := marketdata.GetExchangeCodes()
	if err != nil {
		panic(err)
	}
	fmt.Println(ec)
}
