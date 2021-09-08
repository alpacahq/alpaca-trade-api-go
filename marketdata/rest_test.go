package marketdata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient() *client {
	return NewClient(ClientOpts{}).(*client)
}

func TestLatestTrade(t *testing.T) {
	c := testClient()

	// successful
	latestTradeJSON := `{
		"symbol": "AAPL",
		"trade": {
			"t": "2021-04-20T12:40:34.484136Z",
			"x": "J",
			"p": 134.7,
			"s": 20,
			"c": [
				"@",
				"T",
				"I"
			],
			"i": 32,
			"z": "C"
		}
	}`
	expectedLatestTrade := Trade{
		ID:         32,
		Exchange:   "J",
		Price:      134.7,
		Size:       20,
		Timestamp:  time.Date(2021, 4, 20, 12, 40, 34, 484136000, time.UTC),
		Conditions: []string{"@", "T", "I"},
		Tape:       "C",
	}
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(latestTradeJSON)),
		}, nil
	}

	actualLatestTrade, err := c.GetLatestTrade("AAPL")
	require.NoError(t, err)
	require.NotNil(t, actualLatestTrade)
	assert.Equal(t, expectedLatestTrade, *actualLatestTrade)

	// api failure
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	actualLatestTrade, err = c.GetLatestTrade("AAPL")
	assert.Error(t, err)
	assert.Nil(t, actualLatestTrade)
}

func TesLatestQuote(t *testing.T) {
	c := testClient()
	// successful
	latestQuoteJSON := `{
		"symbol": "AAPL",
		"quote": {
			"t": "2021-04-20T13:01:57.822745906Z",
			"ax": "Q",
			"ap": 134.68,
			"as": 1,
			"bx": "K",
			"bp": 134.66,
			"bs": 29,
			"c": [
				"R"
			]
		}
	}`
	expectedLatestQuote := Quote{
		BidExchange: "K",
		BidPrice:    134.66,
		BidSize:     29,
		AskExchange: "Q",
		AskPrice:    134.68,
		AskSize:     1,
		Timestamp:   time.Date(2021, 04, 20, 13, 1, 57, 822745906, time.UTC),
		Conditions:  []string{"R"},
	}
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(latestQuoteJSON)),
		}, nil
	}

	actualLatestQuote, err := c.GetLatestQuote("AAPL")
	require.NoError(t, err)
	require.NotNil(t, actualLatestQuote)
	assert.Equal(t, expectedLatestQuote, *actualLatestQuote)

	// api failure
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	actualLatestQuote, err = c.GetLatestQuote("AAPL")
	assert.Error(t, err)
	assert.Nil(t, actualLatestQuote)
}

func TestSnapshot(t *testing.T) {
	c := testClient()
	// successful
	snapshotJSON := `{
		"symbol": "AAPL",
		"latestTrade": {
			"t": "2021-05-03T14:45:50.456Z",
			"x": "D",
			"p": 133.55,
			"s": 200,
			"c": [
				"@"
			],
			"i": 61462,
			"z": "C"
		},
		"latestQuote": {
			"t": "2021-05-03T14:45:50.532316972Z",
			"ax": "P",
			"ap": 133.55,
			"as": 7,
			"bx": "Q",
			"bp": 133.54,
			"bs": 9,
			"c": [
				"R"
			]
		},
		"minuteBar": {
			"t": "2021-05-03T14:44:00Z",
			"o": 133.485,
			"h": 133.4939,
			"l": 133.42,
			"c": 133.445,
			"v": 182818
		},
		"dailyBar": {
			"t": "2021-05-03T04:00:00Z",
			"o": 132.04,
			"h": 134.07,
			"l": 131.83,
			"c": 133.445,
			"v": 25094213
		},
		"prevDailyBar": {
			"t": "2021-04-30T04:00:00Z",
			"o": 131.82,
			"h": 133.56,
			"l": 131.065,
			"c": 131.46,
			"v": 109506363
		}
	}`
	expected := Snapshot{
		LatestTrade: &Trade{
			ID:         61462,
			Exchange:   "D",
			Price:      133.55,
			Size:       200,
			Timestamp:  time.Date(2021, 5, 3, 14, 45, 50, 456000000, time.UTC),
			Conditions: []string{"@"},
			Tape:       "C",
		},
		LatestQuote: &Quote{
			BidExchange: "Q",
			BidPrice:    133.54,
			BidSize:     9,
			AskExchange: "P",
			AskPrice:    133.55,
			AskSize:     7,
			Timestamp:   time.Date(2021, 5, 3, 14, 45, 50, 532316972, time.UTC),
			Conditions:  []string{"R"},
		},
		MinuteBar: &Bar{
			Open:      133.485,
			High:      133.4939,
			Low:       133.42,
			Close:     133.445,
			Volume:    182818,
			Timestamp: time.Date(2021, 5, 3, 14, 44, 0, 0, time.UTC),
		},
		DailyBar: &Bar{
			Open:      132.04,
			High:      134.07,
			Low:       131.83,
			Close:     133.445,
			Volume:    25094213,
			Timestamp: time.Date(2021, 5, 3, 4, 0, 0, 0, time.UTC),
		},
		PrevDailyBar: &Bar{
			Open:      131.82,
			High:      133.56,
			Low:       131.065,
			Close:     131.46,
			Volume:    109506363,
			Timestamp: time.Date(2021, 4, 30, 4, 0, 0, 0, time.UTC),
		},
	}
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(snapshotJSON)),
		}, nil
	}

	got, err := c.GetSnapshot("AAPL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, expected, *got)

	// api failure
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	got, err = c.GetSnapshot("AAPL")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestSnapshots(t *testing.T) {
	c := testClient()
	// successful
	snapshotsJSON := `{
		"AAPL": {
			"latestTrade": {
				"t": "2021-05-03T14:48:06.563Z",
				"x": "D",
				"p": 133.4201,
				"s": 145,
				"c": [
					"@"
				],
				"i": 62700,
				"z": "C"
			},
			"latestQuote": {
				"t": "2021-05-03T14:48:07.257820915Z",
				"ax": "Q",
				"ap": 133.43,
				"as": 7,
				"bx": "Q",
				"bp": 133.42,
				"bs": 15,
				"c": [
					"R"
				]
			},
			"minuteBar": {
				"t": "2021-05-03T14:47:00Z",
				"o": 133.4401,
				"h": 133.48,
				"l": 133.37,
				"c": 133.42,
				"v": 207020,
				"n": 1234,
				"vw": 133.3987
			},
			"dailyBar": {
				"t": "2021-05-03T04:00:00Z",
				"o": 132.04,
				"h": 134.07,
				"l": 131.83,
				"c": 133.42,
				"v": 25846800,
				"n": 254678,
				"vw": 132.568
			},
			"prevDailyBar": {
				"t": "2021-04-30T04:00:00Z",
				"o": 131.82,
				"h": 133.56,
				"l": 131.065,
				"c": 131.46,
				"v": 109506363,
				"n": 1012323,
				"vw": 132.025
			}
		},
		"MSFT": {
			"latestTrade": {
				"t": "2021-05-03T14:48:06.36Z",
				"x": "D",
				"p": 253.8738,
				"s": 100,
				"c": [
					"@"
				],
				"i": 22973,
				"z": "C"
			},
			"latestQuote": {
				"t": "2021-05-03T14:48:07.243353456Z",
				"ax": "N",
				"ap": 253.89,
				"as": 2,
				"bx": "Q",
				"bp": 253.87,
				"bs": 2,
				"c": [
					"R"
				]
			},
			"minuteBar": {
				"t": "2021-05-03T14:47:00Z",
				"o": 253.78,
				"h": 253.869,
				"l": 253.78,
				"c": 253.855,
				"v": 25717,
				"n": 137,
				"vw": 253.823
			},
			"dailyBar": {
				"t": "2021-05-03T04:00:00Z",
				"o": 253.34,
				"h": 254.35,
				"l": 251.8,
				"c": 253.855,
				"v": 6100459,
				"n": 33453,
				"vw": 253.0534
			},
			"prevDailyBar": null
		},
		"INVALID": null
	}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(snapshotsJSON)),
		}, nil
	}

	got, err := c.GetSnapshots([]string{"AAPL", "MSFT", "INVALID"})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Len(t, got, 3)
	assert.Nil(t, got["INVALID"])
	assert.EqualValues(t, 7, got["AAPL"].LatestQuote.AskSize)
	assert.EqualValues(t, 254678, got["AAPL"].DailyBar.TradeCount)
	assert.EqualValues(t, 132.025, got["AAPL"].PrevDailyBar.VWAP)
	assert.EqualValues(t, 6100459, got["MSFT"].DailyBar.Volume)
	assert.EqualValues(t, 137, got["MSFT"].MinuteBar.TradeCount)
	assert.Nil(t, got["MSFT"].PrevDailyBar)

	// api failure
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}

	got, err = c.GetSnapshots([]string{"AAPL", "CLDR"})
	assert.Error(t, err)
	assert.Nil(t, got)
}

// TODO: Add more test cases!
