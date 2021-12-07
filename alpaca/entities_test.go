package alpaca

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNonTradeActivities(t *testing.T) {
	// https://alpaca.markets/docs/api-documentation/api-v2/account-activities/#nontradeactivity-entity
	nta := map[string]interface{}{
		"activity_type":    "DIV",
		"id":               "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf",
		"date":             "2019-08-01",
		"net_amount":       "1.02",
		"symbol":           "T",
		"qty":              "2",
		"per_share_amount": "0.51",
	}

	ntaBody, _ := json.Marshal(nta)
	var accActivity AccountActivity
	json.Unmarshal(ntaBody, &accActivity)

	assert.Equal(t, civil.Date{Year: 2019, Month: 8, Day: 1}, accActivity.Date)
	assert.Equal(t, "DIV", accActivity.ActivityType)
	assert.Equal(t, "20190801011955195::5f596936-6f23-4cef-bdf1-3806aae57dbf", accActivity.ID)
	assert.True(t, decimal.NewFromFloat(1.02).Equal(accActivity.NetAmount))
	assert.Equal(t, "T", accActivity.Symbol)
	assert.Equal(t, decimal.NewFromInt(2), accActivity.Qty)
	assert.Equal(t, decimal.NewFromFloat32(0.51), accActivity.PerShareAmount)
}
