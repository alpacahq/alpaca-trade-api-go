package marketdata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient() *client {
	return NewClient(ClientOpts{}).(*client)
}

func mockResp(resp string) func(c *client, req *http.Request) (*http.Response, error) {
	return func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
}

func mockErrResp() func(c *client, req *http.Request) (*http.Response, error) {
	return func(c *client, req *http.Request) (*http.Response, error) {
		return &http.Response{}, fmt.Errorf("fail")
	}
}

func TestTradesWithGzip(t *testing.T) {
	c := testClient()

	f, err := os.Open("testdata/trades.json.gz")
	require.NoError(t, err)
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
		return &http.Response{
			Body: f,
			Header: http.Header{
				"Content-Encoding": []string{"gzip"},
			},
		}, nil
	}
	got, err := c.GetTrades("AAPL", GetTradesParams{
		Start:      time.Date(2021, 10, 13, 0, 0, 0, 0, time.UTC),
		TotalLimit: 5,
		PageLimit:  5,
	})
	require.NoError(t, err)
	require.Len(t, got, 5)
	trade := got[0]
	assert.Equal(t, "P", trade.Exchange)
	assert.EqualValues(t, 140.2, trade.Price)
	assert.EqualValues(t, 595, trade.Size)
	assert.Equal(t, "C", trade.Tape)
}

func TestTradesWithoutGzip(t *testing.T) {
	c := testClient()

	resp := `{"trades":[{"t":"2021-10-13T08:00:00.08960768Z","x":"P","p":140.2,"s":595,"c":["@","T"],"i":1,"z":"C"}],"symbol":"AAPL","next_page_token":"QUFQTHwyMDIxLTEwLTEzVDA4OjAwOjAwLjA4OTYwNzY4MFp8UHwwOTIyMzM3MjAzNjg1NDc3NTgwOQ=="}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
		// Even though we request gzip encoding, the server may decide to not use it
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetTrades("AAPL", GetTradesParams{
		Start:      time.Date(2021, 10, 13, 0, 0, 0, 0, time.UTC),
		TotalLimit: 1,
		PageLimit:  1,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	trade := got[0]
	assert.EqualValues(t, 1, trade.ID)
}

func TestLatestBar(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"symbol":"AAPL","bar":{"t":"2021-10-11T23:59:00Z","o":142.59,"h":142.63,"l":142.57,"c":142.59,"v":2714,"n":22,"vw":142.589071}}`)
	got, err := c.GetLatestBar("AAPL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, Bar{
		Timestamp:  time.Date(2021, 10, 11, 23, 59, 0, 0, time.UTC),
		Open:       142.59,
		High:       142.63,
		Low:        142.57,
		Close:      142.59,
		Volume:     2714,
		TradeCount: 22,
		VWAP:       142.589071,
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestBar("AAPL")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestBarFeed(t *testing.T) {
	c := NewClient(ClientOpts{Feed: "iex"}).(*client)

	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "https://data.alpaca.markets/v2/stocks/AAPL/bars/latest?feed=iex", req.URL.String())
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(
				`{"symbol":"AAPL","bar":{"t":"2021-10-11T19:59:00Z","o":142.9,"h":142.91,"l":142.77,"c":142.8,"v":13886,"n":108,"vw":142.856726}}`,
			)),
		}, nil
	}
	_, err := c.GetLatestBar("AAPL")
	require.NoError(t, err)
}

func TestLatestBars(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"bars":{"NIO":{"t":"2021-10-11T23:59:00Z","o":35.57,"h":35.6,"l":35.56,"c":35.6,"v":1288,"n":9,"vw":35.586483},"AAPL":{"t":"2021-10-11T23:59:00Z","o":142.59,"h":142.63,"l":142.57,"c":142.59,"v":2714,"n":22,"vw":142.589071}}}`)
	got, err := c.GetLatestBars([]string{"AAPL", "NIO"})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 2)
	assert.Equal(t, Bar{
		Timestamp:  time.Date(2021, 10, 11, 23, 59, 0, 0, time.UTC),
		Open:       142.59,
		High:       142.63,
		Low:        142.57,
		Close:      142.59,
		Volume:     2714,
		TradeCount: 22,
		VWAP:       142.589071,
	}, got["AAPL"])
	assert.EqualValues(t, 35.6, got["NIO"].Close)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestBars([]string{"IBM", "MSFT"})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestTrade(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"symbol": "AAPL","trade": {"t": "2021-04-20T12:40:34.484136Z","x": "J","p": 134.7,"s": 20,"c": ["@","T","I"],"i": 32,"z": "C"}}`)
	got, err := c.GetLatestTrade("AAPL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, Trade{
		ID:         32,
		Exchange:   "J",
		Price:      134.7,
		Size:       20,
		Timestamp:  time.Date(2021, 4, 20, 12, 40, 34, 484136000, time.UTC),
		Conditions: []string{"@", "T", "I"},
		Tape:       "C",
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestTrade("AAPL")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestTrades(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"trades":{"IBM":{"t":"2021-10-11T23:42:47.895547Z","x":"K","p":142.2,"s":197,"c":[" ","F","T"],"i":52983525503560,"z":"A"},"MSFT":{"t":"2021-10-11T23:59:39.380716032Z","x":"P","p":294.1,"s":100,"c":["@","T"],"i":28693,"z":"C"}}}`)
	got, err := c.GetLatestTrades([]string{"IBM", "MSFT"})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 2)
	assert.Equal(t, Trade{
		ID:         52983525503560,
		Exchange:   "K",
		Price:      142.2,
		Size:       197,
		Timestamp:  time.Date(2021, 10, 11, 23, 42, 47, 895547000, time.UTC),
		Conditions: []string{" ", "F", "T"},
		Tape:       "A",
	}, got["IBM"])
	assert.EqualValues(t, 294.1, got["MSFT"].Price)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestTrades([]string{"IBM", "MSFT"})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestQuote(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"symbol": "AAPL","quote": {"t": "2021-04-20T13:01:57.822745906Z","ax": "Q","ap": 134.68,"as": 1,"bx": "K","bp": 134.66,"bs": 29,"c": ["R"]}}`)
	got, err := c.GetLatestQuote("AAPL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, Quote{
		BidExchange: "K",
		BidPrice:    134.66,
		BidSize:     29,
		AskExchange: "Q",
		AskPrice:    134.68,
		AskSize:     1,
		Timestamp:   time.Date(2021, 04, 20, 13, 1, 57, 822745906, time.UTC),
		Conditions:  []string{"R"},
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestQuote("AAPL")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestQuotes(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"quotes":{"F":{"t":"2021-10-12T00:00:00.002071Z","ax":"P","ap":15.07,"as":3,"bx":"P","bp":15.01,"bs":3,"c":["R"],"z":"A"},"TSLA":{"t":"2021-10-11T23:59:58.02063232Z","ax":"P","ap":792.6,"as":1,"bx":"P","bp":792,"bs":67,"c":["R"],"z":"C"},"GE":{"t":"2021-10-11T23:02:28.423505152Z","ax":"P","ap":104.06,"as":2,"bx":"P","bp":104.03,"bs":5,"c":["R"],"z":"A"}}}`)
	got, err := c.GetLatestQuotes([]string{"F", "GE", "TSLA"})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got, 3)
	assert.Equal(t, Quote{
		Timestamp:   time.Date(2021, 10, 12, 0, 0, 0, 2071000, time.UTC),
		BidPrice:    15.01,
		BidSize:     3,
		BidExchange: "P",
		AskPrice:    15.07,
		AskSize:     3,
		AskExchange: "P",
		Conditions:  []string{"R"},
		Tape:        "A",
	}, got["F"])
	assert.EqualValues(t, 5, got["GE"].BidSize)
	assert.EqualValues(t, 792, got["TSLA"].BidPrice)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestQuotes([]string{"F", "GE", "TSLA"})
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestSnapshot(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"symbol": "AAPL","latestTrade": {"t": "2021-05-03T14:45:50.456Z","x": "D","p": 133.55,"s": 200,"c": ["@"],"i": 61462,"z": "C"},"latestQuote": {"t": "2021-05-03T14:45:50.532316972Z","ax": "P","ap": 133.55,"as": 7,"bx": "Q","bp": 133.54,"bs": 9,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:44:00Z","o": 133.485,"h": 133.4939,"l": 133.42,"c": 133.445,"v": 182818},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 132.04,"h": 134.07,"l": 131.83,"c": 133.445,"v": 25094213},"prevDailyBar": {"t": "2021-04-30T04:00:00Z","o": 131.82,"h": 133.56,"l": 131.065,"c": 131.46,"v": 109506363}}`)
	got, err := c.GetSnapshot("AAPL")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, Snapshot{
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
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetSnapshot("AAPL")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestSnapshots(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"AAPL": {"latestTrade": {"t": "2021-05-03T14:48:06.563Z","x": "D","p": 133.4201,"s": 145,"c": ["@"],"i": 62700,"z": "C"},"latestQuote": {"t": "2021-05-03T14:48:07.257820915Z","ax": "Q","ap": 133.43,"as": 7,"bx": "Q","bp": 133.42,"bs": 15,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:47:00Z","o": 133.4401,"h": 133.48,"l": 133.37,"c": 133.42,"v": 207020,"n": 1234,"vw": 133.3987},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 132.04,"h": 134.07,"l": 131.83,"c": 133.42,"v": 25846800,"n": 254678,"vw": 132.568},"prevDailyBar": {"t": "2021-04-30T04:00:00Z","o": 131.82,"h": 133.56,"l": 131.065,"c": 131.46,"v": 109506363,"n": 1012323,"vw": 132.025}},"MSFT": {"latestTrade": {"t": "2021-05-03T14:48:06.36Z","x": "D","p": 253.8738,"s": 100,"c": ["@"],"i": 22973,"z": "C"},"latestQuote": {"t": "2021-05-03T14:48:07.243353456Z","ax": "N","ap": 253.89,"as": 2,"bx": "Q","bp": 253.87,"bs": 2,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:47:00Z","o": 253.78,"h": 253.869,"l": 253.78,"c": 253.855,"v": 25717,"n": 137,"vw": 253.823},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 253.34,"h": 254.35,"l": 251.8,"c": 253.855,"v": 6100459,"n": 33453,"vw": 253.0534},"prevDailyBar": null},"INVALID": null}`)

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
	c.do = mockErrResp()
	got, err = c.GetSnapshots([]string{"AAPL", "CLDR"})
	assert.Error(t, err)
	assert.Nil(t, got)
}

// TODO: Add more test cases!
