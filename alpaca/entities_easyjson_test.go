package alpaca

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWatchlistUnmarshalNullAssetsClearsExistingSlice(t *testing.T) {
	watchlist := Watchlist{
		Assets: []Asset{{Symbol: "AAPL"}},
	}

	require.NoError(t, json.Unmarshal([]byte(`{"assets":null}`), &watchlist))
	require.Nil(t, watchlist.Assets)
}
