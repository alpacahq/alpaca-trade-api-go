package alpaca

import (
	"github.com/shopspring/decimal"
)

// RoundLimitPrice calculates the limit price that respects the minimum price variance rule.
//
// Orders received in excess of the minimum price variance will be rejected.
//
//	Limit price >= $1.00: Max Decimals = 2
//	Limit price <  $1.00: Max Decimals = 4
//
// https://docs.alpaca.markets/docs/orders-at-alpaca#sub-penny-increments-for-limit-orders
func RoundLimitPrice(price decimal.Decimal, side Side) *decimal.Decimal {
	limitPrice := price.Copy()
	maxDecimals := int32(2)
	if price.LessThan(decimal.NewFromInt(1)) {
		maxDecimals = 4
	}
	switch side {
	case Buy:
		limitPrice = price.RoundCeil(maxDecimals)
	case Sell:
		limitPrice = price.RoundFloor(maxDecimals)
	}
	return &limitPrice
}
