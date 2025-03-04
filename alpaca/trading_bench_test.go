package alpaca

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Benchmark for RoundLimitPrice for Buy orders.
func BenchmarkRoundLimitPrice_Buy(b *testing.B) {
	price := decimal.NewFromFloat(41.085)
	for i := 0; i < b.N; i++ {
		_ = RoundLimitPrice(price, Buy)
	}
}

// Benchmark for RoundLimitPrice for Sell orders.
func BenchmarkRoundLimitPrice_Sell(b *testing.B) {
	price := decimal.NewFromFloat(41.085)
	for i := 0; i < b.N; i++ {
		_ = RoundLimitPrice(price, Sell)
	}
}
