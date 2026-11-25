package stream

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCryptoOrderbookUnmarshalNullBidsClearsExistingSlice(t *testing.T) {
	orderbook := CryptoOrderbook{
		Bids: []CryptoOrderbookEntry{{Price: 123.45}},
	}

	require.NoError(t, json.Unmarshal([]byte(`{"bids":null}`), &orderbook))
	require.Nil(t, orderbook.Bids)
}
