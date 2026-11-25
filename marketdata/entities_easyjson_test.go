package marketdata

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyAuctionsUnmarshalNullOpeningClearsExistingSlice(t *testing.T) {
	auctions := DailyAuctions{
		Opening: []Auction{{Price: 123.45}},
	}

	require.NoError(t, json.Unmarshal([]byte(`{"o":null}`), &auctions))
	require.Nil(t, auctions.Opening)
}
