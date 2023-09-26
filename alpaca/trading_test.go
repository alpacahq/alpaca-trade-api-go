package alpaca

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func Test_calculateLimitPrice(t *testing.T) {
	tests := []struct {
		name  string
		price decimal.Decimal
		side  Side
		exp   decimal.Decimal
	}{
		{
			name:  "buy expensive",
			price: decimal.RequireFromString("41.085"),
			side:  Buy,
			exp:   decimal.RequireFromString("41.09"),
		},
		{
			name:  "buy cheap no rounding",
			price: decimal.RequireFromString("0.9999"),
			side:  Buy,
			exp:   decimal.RequireFromString("0.9999"),
		},
		{
			name:  "buy cheap rounding",
			price: decimal.RequireFromString("0.12182"),
			side:  Buy,
			exp:   decimal.RequireFromString("0.1219"),
		},
		{
			name:  "sell expensive",
			price: decimal.RequireFromString("41.085"),
			side:  Sell,
			exp:   decimal.RequireFromString("41.08"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundLimitPrice(tt.price, tt.side)
			assert.True(t, tt.exp.Equal(*got), "expected: %s, got: %s", tt.exp.String(), got.String())
		})
	}
}
