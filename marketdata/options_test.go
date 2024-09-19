package marketdata

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOptionTrades(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		resp := `{"next_page_token":"QUFQTDI0MDMwOEMwMDE3MjUwMHwxNzA5NjU3OTk5MTA5MzE5NDI0fFU=","trades":{"AAPL240308C00172500":[{"c":"g","p":0.98,"s":4,"t":"2024-03-05T16:59:59.816906752Z","x":"C"},{"c":"I","p":0.99,"s":1,"t":"2024-03-05T16:59:59.109319424Z","x":"U"}]}}` //nolint:lll
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetOptionTrades("AAPL240308C00172500", GetOptionTradesRequest{
		Start:      time.Date(2024, 3, 5, 16, 0, 0, 0, time.UTC),
		End:        time.Date(2024, 3, 5, 17, 0, 0, 0, time.UTC),
		TotalLimit: 2,
		Sort:       SortDesc,
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, OptionTrade{
		Timestamp: time.Date(2024, 3, 5, 16, 59, 59, 816906752, time.UTC),
		Price:     0.98,
		Size:      4,
		Exchange:  "C",
		Condition: "g",
	}, got[0])
	assert.Equal(t, OptionTrade{
		Timestamp: time.Date(2024, 3, 5, 16, 59, 59, 109319424, time.UTC),
		Price:     0.99,
		Size:      1,
		Exchange:  "U",
		Condition: "I",
	}, got[1])
}

func TestGetOptionBars(t *testing.T) {
	c := DefaultClient
	resp := `{"bars":{"AAPL240308C00172500":[{"c":1.1,"h":1.26,"l":1.1,"n":15,"o":1.23,"t":"2024-03-05T14:00:00Z","v":82,"vw":1.187683},{"c":0.99,"h":1.14,"l":0.83,"n":1545,"o":1,"t":"2024-03-05T15:00:00Z","v":9959,"vw":0.959978},{"c":0.98,"h":1.15,"l":0.85,"n":1075,"o":0.93,"t":"2024-03-05T16:00:00Z","v":7637,"vw":0.965448},{"c":0.97,"h":1.15,"l":0.93,"n":1096,"o":0.99,"t":"2024-03-05T17:00:00Z","v":8483,"vw":1.028201},{"c":0.91,"h":1.1,"l":0.88,"n":903,"o":0.96,"t":"2024-03-05T18:00:00Z","v":7925,"vw":0.96723},{"c":0.9,"h":1,"l":0.88,"n":423,"o":0.9,"t":"2024-03-05T19:00:00Z","v":2895,"vw":0.931516},{"c":0.97,"h":1,"l":0.87,"n":543,"o":0.92,"t":"2024-03-05T20:00:00Z","v":5669,"vw":0.9383}]},"next_page_token":null}` //nolint:lll
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta1/options/bars", req.URL.Path)
		assert.Equal(t, "AAPL240308C00172500", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2024-03-05T00:00:00Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2024-03-06T00:00:00Z", req.URL.Query().Get("end"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}

	got, err := c.GetOptionBars("AAPL240308C00172500", GetOptionBarsRequest{
		TimeFrame: OneDay,
		Start:     time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2024, 3, 6, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, got, 7)
	for i := 0; i < 7; i++ {
		assert.Equal(t, fmt.Sprintf("2024-03-05T%d:00:00Z", 14+i), got[i].Timestamp.Format(time.RFC3339))
	}
	assert.EqualValues(t, 1.23, got[0].Open)
	assert.EqualValues(t, 1.26, got[0].High)
	assert.EqualValues(t, 1.1, got[0].Low)
	assert.EqualValues(t, 1.1, got[0].Close)
	assert.EqualValues(t, 82, got[0].Volume)
	assert.EqualValues(t, 15, got[0].TradeCount)
	assert.EqualValues(t, 1.187683, got[0].VWAP)
}

func TestGetLatestOptionTrade(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(
		`{"trades":{"BABA260116P00125000":{"c":"f","p":49.61,"s":1,"t":"2024-02-26T18:23:18.79373184Z","x":"D"}}}`)
	got, err := c.GetLatestOptionTrade("BABA260116P00125000", GetLatestOptionTradeRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, OptionTrade{
		Exchange:  "D",
		Price:     49.61,
		Size:      1,
		Timestamp: time.Date(2024, 2, 26, 18, 23, 18, 793731840, time.UTC),
		Condition: "f",
	}, *got)

	c.do = mockResp(`{"trades":{}}`)
	got, err = c.GetLatestOptionTrade("BABA260116P00125000", GetLatestOptionTradeRequest{
		Feed: Indicative,
	})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestGetLatestOptionQuote(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"quotes":{"SPXW240327P04925000":{"ap":11.7,"as":103,"ax":"C","bp":11.4,"bs":172,"bx":"C","c":"A","t":"2024-03-07T13:54:51.985563136Z"}}}`) //nolint:lll
	got, err := c.GetLatestOptionQuote("SPXW240327P04925000", GetLatestOptionQuoteRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, OptionQuote{
		BidExchange: "C",
		BidPrice:    11.4,
		BidSize:     172,
		AskExchange: "C",
		AskPrice:    11.7,
		AskSize:     103,
		Timestamp:   time.Date(2024, 3, 7, 13, 54, 51, 985563136, time.UTC),
		Condition:   "A",
	}, *got)
}

func TestGetOptionSnapshot(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"snapshots":{"SPXW240327P04925000":{"latestQuote":{"ap":11.6,"as":59,"ax":"C","bp":11.3,"bs":180,"bx":"C","c":"A","t":"2024-03-07T13:56:22.278961408Z"},"latestTrade":{"c":"g","p":14.85,"s":2,"t":"2024-03-05T16:36:57.709309696Z","x":"C"}}}}`) //nolint:lll
	got, err := c.GetOptionSnapshot("SPXW240327P04925000", GetOptionSnapshotRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, OptionQuote{
		BidExchange: "C",
		BidPrice:    11.3,
		BidSize:     180,
		AskExchange: "C",
		AskPrice:    11.6,
		AskSize:     59,
		Timestamp:   time.Date(2024, 3, 7, 13, 56, 22, 278961408, time.UTC),
		Condition:   "A",
	}, *got.LatestQuote)
	assert.Equal(t, OptionTrade{
		Timestamp: time.Date(2024, 3, 5, 16, 36, 57, 709309696, time.UTC),
		Price:     14.85,
		Size:      2,
		Exchange:  "C",
		Condition: "g",
	}, *got.LatestTrade)
}

func TestGetOptionChain(t *testing.T) {
	defer func() {
		DefaultClient = NewClient(ClientOpts{})
	}()
	DefaultClient.do = mockResp(`{"snapshots":{"NIO240719P00002000":{"latestQuote":{"ap":1.24,"as":2183,"ax":"A","bp":0.03,"bs":1143,"bx":"A","c":"A","t":"2024-03-06T20:59:05.378523136Z"}},"NIO240405P00004000":{"latestQuote":{"ap":0.05,"as":3041,"ax":"D","bp":0.03,"bs":1726,"bx":"D","c":"C","t":"2024-03-06T20:59:59.678798848Z"},"latestTrade":{"c":"f","p":0.05,"s":17,"t":"2024-03-07T15:53:37.134486784Z","x":"I"}}}}`) //nolint:lll
	got, err := GetOptionChain("NIO", GetOptionChainRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 2)
	require.Contains(t, got, "NIO240719P00002000")
	require.Contains(t, got, "NIO240405P00004000")
	assert.Nil(t, got["NIO240719P00002000"].LatestTrade)
	assert.Equal(t, 1.24, got["NIO240719P00002000"].LatestQuote.AskPrice)
	assert.EqualValues(t, 1726, got["NIO240405P00004000"].LatestQuote.BidSize)
	assert.EqualValues(t, 17, got["NIO240405P00004000"].LatestTrade.Size)
}

func TestGetOptionChainWithFilters(t *testing.T) {
	c := DefaultClient
	//nolint:lll
	firstResp := `{"next_page_token":"QUFQTDI0MDQyNkMwMDE1MjUwMA==","snapshots":{"AAPL240426C00152500":{"latestQuote":{"ap":17,"as":91,"ax":"B","bp":16.25,"bs":80,"bx":"B","c":" ","t":"2024-04-24T19:59:59.782060288Z"},"latestTrade":{"c":"a","p":15.87,"s":1,"t":"2024-04-24T16:46:16.763406848Z","x":"I"}}}}`
	secondResp := `{"next_page_token":null,"snapshots":{"AAPL240426C00155000":{"greeks":{"delta":0.9567110374646104,"gamma":0.010515010903989475,"rho":0.004041091409185355,"theta":-0.42275702792812153,"vega":0.008131530784084512},"impliedVolatility":0.9871160931510816,"latestQuote":{"ap":14.5,"as":86,"ax":"Q","bp":13.6,"bs":91,"bx":"B","c":" ","t":"2024-04-24T19:59:59.794910976Z"},"latestTrade":{"c":"a","p":14.28,"s":1,"t":"2024-04-24T19:42:55.36938496Z","x":"X"}}}}` //nolint:lll
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta1/options/snapshots/AAPL", req.URL.Path)
		q := req.URL.Query()
		assert.Equal(t, "1", q.Get("limit"))
		assert.Equal(t, "151.123", q.Get("strike_price_gte"))
		assert.Equal(t, "155", q.Get("strike_price_lte"))
		assert.Equal(t, "1", q.Get("limit"))
		assert.Equal(t, "call", q.Get("type"))
		assert.Equal(t, "2024-04-26", q.Get("expiration_date_lte"))
		pageToken := q.Get("page_token")
		var resp string
		switch pageToken {
		case "":
			resp = firstResp
		case "QUFQTDI0MDQyNkMwMDE1MjUwMA==":
			resp = secondResp
		default:
			require.Fail(t, "unexpected page token: "+pageToken)
		}
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetOptionChain("AAPL", GetOptionChainRequest{
		PageLimit:         1,
		Type:              Call,
		StrikePriceGte:    151.123,
		StrikePriceLte:    155,
		ExpirationDateLte: civil.Date{Year: 2024, Month: 4, Day: 26},
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 2)
	require.Contains(t, got, "AAPL240426C00152500")
	s := got["AAPL240426C00152500"]
	assert.EqualValues(t, 17, s.LatestQuote.AskPrice)
	assert.EqualValues(t, 15.87, s.LatestTrade.Price)
	require.Contains(t, got, "AAPL240426C00155000")
	s = got["AAPL240426C00155000"]
	d := 0.1
	assert.InDelta(t, 0.9871, s.ImpliedVolatility, d)
	if assert.NotNil(t, s.Greeks) {
		assert.InDelta(t, 0.9567, s.Greeks.Delta, d)
		assert.InDelta(t, 0.0105, s.Greeks.Gamma, 0.1)
		assert.InDelta(t, 0.004, s.Greeks.Rho, 0.1)
		assert.InDelta(t, -0.422, s.Greeks.Theta, 0.1)
		assert.InDelta(t, 0.0081, s.Greeks.Vega, 0.1)
	}
}
