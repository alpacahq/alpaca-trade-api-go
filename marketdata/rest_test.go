package marketdata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"symbol":"SPY","bar":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}`)
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		BaseURL: server.URL,
	})
	bar, err := client.GetLatestBar("SPY")
	require.NoError(t, err)
	assert.Equal(t, 469.11, bar.Low)
}

func TestDefaultDo_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()
	// instead of using the BaseURL opts, we test setting the base URL via environment variables
	originalDataURL := os.Getenv("APCA_API_DATA_URL")
	defer func() { os.Setenv("APCA_API_DATA_URL", originalDataURL) }()
	require.NoError(t, os.Setenv("APCA_API_DATA_URL", server.URL))
	client := NewClient(ClientOpts{
		OAuth: "myoauthkey",
	})
	_, err := client.GetLatestBar("SPY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDefaultDo_Retry(t *testing.T) {
	tryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch tryCount {
		case 0:
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		default:
			fmt.Fprint(w, `{"symbol":"SPY","bar":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}`)
		}
		tryCount++
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		BaseURL:    server.URL,
		RetryDelay: time.Millisecond,
		RetryLimit: 1,
	})
	bar, err := client.GetLatestBar("SPY")
	require.NoError(t, err)
	assert.Equal(t, 469.18, bar.High)
}

func TestDefaultDo_TooMany429s(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	defer server.Close()
	opts := ClientOpts{
		BaseURL:    server.URL,
		RetryDelay: time.Millisecond,
		RetryLimit: 10,
	}
	client := NewClient(opts)
	_, err := client.GetLatestBar("SPY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Equal(t, opts.RetryLimit+1, called) // +1 for the original request
}

func TestDefaultDo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
		fmt.Fprint(w, `{"symbol":"SPY","bar":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}`)
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		Timeout: time.Millisecond,
	})
	_, err := client.GetLatestBar("SPY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timeout")
}

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

func TestGetTrades_Gzip(t *testing.T) {
	c := NewClient(ClientOpts{
		Feed: "sip",
	}).(*client)

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

func TestGetTrades(t *testing.T) {
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

func TestGetTrades_InvalidURL(t *testing.T) {
	c := NewClient(ClientOpts{
		BaseURL: string([]byte{0, 1, 2, 3}),
	}).(*client)
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		require.Fail(t, "the server should not have been called")
		return nil, nil
	}
	_, err := c.GetTrades("AAPL", GetTradesParams{})
	require.Error(t, err)
}

func TestGetTrades_ServerError(t *testing.T) {
	c := testClient()
	c.do = mockErrResp()
	_, err := c.GetTrades("SPY", GetTradesParams{})
	require.Error(t, err)
}

func TestGetTrades_InvalidResponse(t *testing.T) {
	c := testClient()
	c.do = mockResp("not a valid json")
	_, err := c.GetTrades("SPY", GetTradesParams{})
	require.Error(t, err)
}

func TestGetMultiTrades(t *testing.T) {
	c := testClient()
	resp := `{"trades":{"F":[{"t":"2018-06-04T19:18:17.4392Z","x":"D","p":11.715,"s":5,"c":[" ","I"],"i":442254,"z":"A"},{"t":"2018-06-04T19:18:18.7453Z","x":"D","p":11.71,"s":200,"c":[" "],"i":442258,"z":"A"}],"GE":[{"t":"2018-06-04T19:18:18.2305Z","x":"D","p":13.74,"s":100,"c":[" "],"i":933063,"z":"A"},{"t":"2018-06-04T19:18:18.6206Z","x":"D","p":13.7317,"s":100,"c":[" ","4","B"],"i":933066,"z":"A"}]},"next_page_token":null}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "data.alpaca.markets", req.URL.Host)
		assert.Equal(t, "/v2/stocks/trades", req.URL.Path)
		assert.Equal(t, "2018-06-04T19:18:17Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2018-06-04T19:18:19Z", req.URL.Query().Get("end"))
		assert.Equal(t, "F,GE", req.URL.Query().Get("symbols"))
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetMultiTrades([]string{"F", "GE"}, GetTradesParams{
		Start: time.Date(2018, 6, 4, 19, 18, 17, 0, time.UTC),
		End:   time.Date(2018, 6, 4, 19, 18, 19, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	f := got["F"]
	assert.Len(t, f, 2)
	assert.True(t, f[0].Timestamp.Equal(time.Date(2018, 6, 4, 19, 18, 17, 439200000, time.UTC)))
	assert.EqualValues(t, 11.715, f[0].Price)
	assert.EqualValues(t, 5, f[0].Size)
	assert.EqualValues(t, "D", f[0].Exchange)
	assert.EqualValues(t, 442254, f[0].ID)
	assert.EqualValues(t, []string{" ", "I"}, f[0].Conditions)
	assert.EqualValues(t, "A", f[0].Tape)
	ge := got["GE"]
	assert.Len(t, ge, 2)
	assert.Equal(t, []string{" "}, ge[0].Conditions)
	assert.Equal(t, []string{" ", "4", "B"}, ge[1].Conditions)
}

func TestGetQuotes(t *testing.T) {
	c := testClient()
	firstResp := `{"quotes":[{"t":"2021-10-04T18:00:14.012577217Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.016722688Z","ax":"N","ap":143.71,"as":3,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.020123648Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.070107859Z","ax":"N","ap":143.71,"as":2,"bx":"U","bp":143.69,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.0709007Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.179935833Z","ax":"N","ap":143.71,"as":2,"bx":"T","bp":143.69,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.179937077Z","ax":"N","ap":143.71,"as":2,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180278784Z","ax":"N","ap":143.71,"as":1,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180473523Z","ax":"N","ap":143.71,"as":1,"bx":"U","bp":143.69,"bs":3,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180522Z","ax":"N","ap":143.71,"as":1,"bx":"Z","bp":143.69,"bs":6,"c":["R"],"z":"A"}],"symbol":"IBM","next_page_token":"SUJNfDIwMjEtMTAtMDRUMTg6MDA6MTQuMTgwNTIyMDAwWnwxMzQ0OTQ0Mw=="}`
	secondResp := `{"quotes":[{"t":"2021-10-04T18:00:14.180608Z","ax":"N","ap":143.71,"as":1,"bx":"U","bp":143.69,"bs":3,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.210488488Z","ax":"N","ap":143.71,"as":1,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"}],"symbol":"IBM","next_page_token":null}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "data.alpaca.markets", req.URL.Host)
		assert.Equal(t, "/v2/stocks/IBM/quotes", req.URL.Path)
		assert.Equal(t, "2021-10-04T18:00:14Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2021-10-04T18:00:15Z", req.URL.Query().Get("end"))
		assert.Equal(t, "10", req.URL.Query().Get("limit"))
		pageToken := req.URL.Query().Get("page_token")
		resp := firstResp
		if pageToken != "" {
			assert.Equal(t, "SUJNfDIwMjEtMTAtMDRUMTg6MDA6MTQuMTgwNTIyMDAwWnwxMzQ0OTQ0Mw==", pageToken)
			resp = secondResp
		}
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetQuotes("IBM", GetQuotesParams{
		Start:     time.Date(2021, 10, 4, 18, 0, 14, 0, time.UTC),
		End:       time.Date(2021, 10, 4, 18, 0, 15, 0, time.UTC),
		PageLimit: 10,
	})
	require.NoError(t, err)
	require.Len(t, got, 12)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 10, 4, 18, 00, 14, 12577217, time.UTC)))
	assert.EqualValues(t, 143.68, got[0].BidPrice)
	assert.EqualValues(t, 1, got[0].BidSize)
	assert.EqualValues(t, "H", got[0].BidExchange)
	assert.EqualValues(t, 143.71, got[0].AskPrice)
	assert.EqualValues(t, 2, got[0].AskSize)
	assert.EqualValues(t, "N", got[0].AskExchange)
	assert.EqualValues(t, []string{"R"}, got[0].Conditions)
	assert.EqualValues(t, "A", got[0].Tape)
	assert.EqualValues(t, 143.69, got[11].BidPrice)
}

func TestGetMultiQuotes(t *testing.T) {
	c := testClient()
	resp := `{"quotes":{"BA":[{"t":"2021-09-15T17:00:00.010461656Z","ax":"N","ap":212.59,"as":1,"bx":"T","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.010657639Z","ax":"N","ap":212.59,"as":1,"bx":"J","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.184164565Z","ax":"N","ap":212.59,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.18418Z","ax":"N","ap":212.59,"as":1,"bx":"N","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.186067456Z","ax":"Y","ap":212.61,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.186265Z","ax":"N","ap":212.64,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"}]},"next_page_token":"QkF8MjAyMS0wOS0xNVQxNzowMDowMC4xODYyNjUwMDBafEI4ODRBOUM3"}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "data.alpaca.markets", req.URL.Host)
		assert.Equal(t, "/v2/stocks/quotes", req.URL.Path)
		assert.Equal(t, "6", req.URL.Query().Get("limit"))
		assert.Equal(t, "BA,DIS", req.URL.Query().Get("symbols"))
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetMultiQuotes([]string{"BA", "DIS"}, GetQuotesParams{
		Start:      time.Date(2021, 9, 15, 17, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 9, 15, 18, 0, 0, 0, time.UTC),
		TotalLimit: 6,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got["BA"], 6)
	assert.Equal(t, 212.59, got["BA"][2].AskPrice)
}

func TestGetBars(t *testing.T) {
	c := testClient()

	c.do = mockResp(`{"bars":[{"t":"2021-10-15T16:00:00Z","o":3378.14,"h":3380.815,"l":3376.3001,"c":3379.72,"v":211689,"n":5435,"vw":3379.041755},{"t":"2021-10-15T16:15:00Z","o":3379.5241,"h":3383.24,"l":3376.49,"c":3377.82,"v":115850,"n":5544,"vw":3379.638266},{"t":"2021-10-15T16:30:00Z","o":3377.982,"h":3380.86,"l":3377,"c":3380,"v":58531,"n":3679,"vw":3379.100605},{"t":"2021-10-15T16:45:00Z","o":3379.73,"h":3387.17,"l":3378.7701,"c":3386.7615,"v":83180,"n":4736,"vw":3381.838113},{"t":"2021-10-15T17:00:00Z","o":3387.56,"h":3390.74,"l":3382.87,"c":3382.87,"v":134339,"n":5832,"vw":3387.086825}],"symbol":"AMZN","next_page_token":null}`)
	got, err := c.GetBars("AMZN", GetBarsParams{
		TimeFrame:  NewTimeFrame(15, Min),
		Adjustment: Split,
		Start:      time.Date(2021, 10, 15, 16, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 10, 15, 17, 0, 0, 0, time.UTC),
		Feed:       "sip",
	})
	require.NoError(t, err)
	require.Len(t, got, 5)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 10, 15, 16, 0, 0, 0, time.UTC)))
	assert.EqualValues(t, 3378.14, got[0].Open)
	assert.EqualValues(t, 3380.815, got[0].High)
	assert.EqualValues(t, 3376.3001, got[0].Low)
	assert.EqualValues(t, 3379.72, got[0].Close)
	assert.EqualValues(t, 211689, got[0].Volume)
	assert.EqualValues(t, 5435, got[0].TradeCount)
	assert.EqualValues(t, 3379.041755, got[0].VWAP)
	assert.True(t, got[1].Timestamp.Equal(time.Date(2021, 10, 15, 16, 15, 0, 0, time.UTC)))
	assert.True(t, got[2].Timestamp.Equal(time.Date(2021, 10, 15, 16, 30, 0, 0, time.UTC)))
	assert.True(t, got[3].Timestamp.Equal(time.Date(2021, 10, 15, 16, 45, 0, 0, time.UTC)))
	assert.True(t, got[4].Timestamp.Equal(time.Date(2021, 10, 15, 17, 0, 0, 0, time.UTC)))
}

func TestGetMultiBars(t *testing.T) {
	c := testClient()

	c.do = mockResp(`{"bars":{"AAPL":[{"t":"2021-10-13T04:00:00Z","o":141.21,"h":141.4,"l":139.2,"c":140.91,"v":78993712,"n":595435,"vw":140.361873},{"t":"2021-10-14T04:00:00Z","o":142.08,"h":143.88,"l":141.51,"c":143.76,"v":69696731,"n":445634,"vw":143.216983},{"t":"2021-10-15T04:00:00Z","o":144.13,"h":144.895,"l":143.51,"c":144.84,"v":67393148,"n":426182,"vw":144.320565}],"NIO":[{"t":"2021-10-13T04:00:00Z","o":35.75,"h":36.68,"l":35.47,"c":36.24,"v":33394068,"n":177991,"vw":36.275125},{"t":"2021-10-14T04:00:00Z","o":36.09,"h":36.45,"l":35.605,"c":36.28,"v":29890265,"n":166379,"vw":36.00485},{"t":"2021-10-15T04:00:00Z","o":37,"h":38.29,"l":36.935,"c":37.71,"v":48138793,"n":257074,"vw":37.647123}]},"next_page_token":null}`)
	got, err := c.GetMultiBars([]string{"AAPL", "NIO"}, GetBarsParams{
		TimeFrame: OneDay,
		Start:     time.Date(2021, 10, 13, 4, 0, 0, 0, time.UTC),
		End:       time.Date(2021, 10, 17, 4, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, symbol := range []string{"AAPL", "NIO"} {
		assert.Len(t, got[symbol], 3)
		for i := 0; i < 3; i++ {
			assert.True(t, got[symbol][i].Timestamp.Equal(time.Date(2021, 10, 13+i, 4, 0, 0, 0, time.UTC)))
		}
	}
	assert.Equal(t, 140.361873, got["AAPL"][0].VWAP)
	assert.Equal(t, 36.00485, got["NIO"][1].VWAP)
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

func TestGetCryptoTrades(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"trades":[{"t":"2021-09-08T05:04:04.262Z","x":"CBSE","p":46391.58,"s":0.0523,"tks":"S","i":209199073},{"t":"2021-09-08T05:04:04.338Z","x":"CBSE","p":46388.41,"s":0.022,"tks":"S","i":209199074},{"t":"2021-09-08T05:04:04.599Z","x":"CBSE","p":46388.42,"s":0.00039732,"tks":"B","i":209199075}],"symbol":"BTCUSD","next_page_token":"QlRDVVNEfDIwMjEtMDktMDhUMDU6MDQ6MDQuNTk5MDAwMDAwWnxDQlNFfDA5MjIzMzcyMDM3MDYzOTc0ODgz"}`)
	got, err := c.GetCryptoTrades("BTCUSD", GetCryptoTradesParams{
		Start:      time.Date(2021, 9, 8, 5, 4, 3, 0, time.UTC),
		End:        time.Date(2021, 9, 8, 5, 6, 7, 0, time.UTC),
		TotalLimit: 3,
		PageLimit:  100,
		Exchanges:  []string{"CBSE"},
	})
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.True(t, got[1].Timestamp.Equal(time.Date(2021, 9, 8, 5, 4, 4, 338000000, time.UTC)))
	assert.EqualValues(t, 46388.41, got[1].Price)
	assert.EqualValues(t, 0.022, got[1].Size)
	assert.EqualValues(t, "CBSE", got[1].Exchange)
	assert.EqualValues(t, 209199074, got[1].ID)
	assert.EqualValues(t, "S", got[1].TakerSide)
}

func TestGetCryptoQuotes(t *testing.T) {
	c := testClient()
	firstResp := `{"quotes":[{"t":"2021-10-09T05:04:06.216Z","x":"ERSX","bp":3580.61,"bs":13.9501,"ap":3589.22,"as":13.9446},{"t":"2021-10-09T05:04:06.225Z","x":"ERSX","bp":3580.86,"bs":13.9492,"ap":3589.29,"as":13.9443},{"t":"2021-10-09T05:04:06.225Z","x":"ERSX","bp":3580.86,"bs":13.9492,"ap":3589.22,"as":13.9446},{"t":"2021-10-09T05:04:06.234Z","x":"ERSX","bp":3580.81,"bs":13.9493,"ap":3589.22,"as":13.9446},{"t":"2021-10-09T05:04:06.234Z","x":"ERSX","bp":3580.81,"bs":13.9493,"ap":3589.29,"as":13.9443},{"t":"2021-10-09T05:04:06.259Z","x":"ERSX","bp":3581.15,"bs":13.948,"ap":3589.22,"as":13.9446},{"t":"2021-10-09T05:04:06.259Z","x":"ERSX","bp":3581.15,"bs":13.948,"ap":3589.34,"as":41.8365},{"t":"2021-10-09T05:04:06.259Z","x":"ERSX","bp":3581.15,"bs":13.948,"ap":3589.59,"as":13.9431},{"t":"2021-10-09T05:04:06.334Z","x":"ERSX","bp":3580.88,"bs":13.9491,"ap":3589.32,"as":13.9442},{"t":"2021-10-09T05:04:06.334Z","x":"ERSX","bp":3580.88,"bs":13.9491,"ap":3589.59,"as":13.9431}],"symbol":"ETHUSD","next_page_token":"RVRIVVNEfDIwMjEtMTAtMDlUMDU6MDQ6MDYuMzM0MDAwMDAwWnxFUlNYfDMyMEZDQzY3"}`
	secondResp := `{"quotes":[{"t":"2021-10-09T05:04:10.669Z","x":"ERSX","bp":3580.69,"bs":13.9498,"ap":3589.32,"as":13.9442},{"t":"2021-10-09T05:04:10.669Z","x":"ERSX","bp":3580.69,"bs":13.9498,"ap":3589.12,"as":13.9449},{"t":"2021-10-09T05:04:10.805Z","x":"ERSX","bp":3580.87,"bs":13.9491,"ap":3589.31,"as":13.9442},{"t":"2021-10-09T05:04:10.805Z","x":"ERSX","bp":3580.87,"bs":13.9491,"ap":3589.12,"as":13.9449},{"t":"2021-10-09T05:04:11.179Z","x":"ERSX","bp":3580.87,"bs":13.9491,"ap":3589.13,"as":13.9449},{"t":"2021-10-09T05:04:11.211Z","x":"ERSX","bp":3580.64,"bs":13.95,"ap":3589.13,"as":13.9449},{"t":"2021-10-09T05:04:11.932Z","x":"ERSX","bp":3580.83,"bs":13.9493,"ap":3589.13,"as":13.9449},{"t":"2021-10-09T05:04:12.062Z","x":"ERSX","bp":3580.83,"bs":13.9493,"ap":3589.31,"as":13.9442},{"t":"2021-10-09T05:04:12.07Z","x":"ERSX","bp":3580.83,"bs":13.9493,"ap":3589.35,"as":13.944},{"t":"2021-10-09T05:04:12.101Z","x":"ERSX","bp":3581.11,"bs":13.9482,"ap":3589.35,"as":13.944}],"symbol":"ETHUSD","next_page_token":"RVRIVVNEfDIwMjEtMTAtMDlUMDU6MDQ6MTIuMTAxMDAwMDAwWnxFUlNYfEQyMTk4OTZB"}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "data.alpaca.markets", req.URL.Host)
		assert.Equal(t, "/v1beta1/crypto/ETHUSD/quotes", req.URL.Path)
		assert.Equal(t, "2021-10-09T05:04:03Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2021-10-09T05:06:07Z", req.URL.Query().Get("end"))
		assert.Equal(t, "10", req.URL.Query().Get("limit"))
		pageToken := req.URL.Query().Get("page_token")
		resp := firstResp
		if pageToken != "" {
			assert.Equal(t, "RVRIVVNEfDIwMjEtMTAtMDlUMDU6MDQ6MDYuMzM0MDAwMDAwWnxFUlNYfDMyMEZDQzY3", pageToken)
			resp = secondResp
		}
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetCryptoQuotes("ETHUSD", GetCryptoQuotesParams{
		Start:      time.Date(2021, 10, 9, 5, 4, 3, 0, time.UTC),
		End:        time.Date(2021, 10, 9, 5, 6, 7, 0, time.UTC),
		PageLimit:  10,
		TotalLimit: 20,
	})
	require.NoError(t, err)
	require.Len(t, got, 20)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 10, 9, 5, 4, 6, 216000000, time.UTC)))
	assert.EqualValues(t, 3580.61, got[0].BidPrice)
	assert.EqualValues(t, 13.9492, got[1].BidSize)
	assert.EqualValues(t, "ERSX", got[2].Exchange)
	assert.EqualValues(t, 3589.22, got[3].AskPrice)
	assert.EqualValues(t, 13.9443, got[4].AskSize)
	assert.True(t, got[19].Timestamp.Equal(time.Date(2021, 10, 9, 5, 4, 12, 101000000, time.UTC)))
}

func TestGetCryptoBars(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"bars":[{"t":"2021-11-11T11:11:00Z","x":"CBSE","o":679.75,"h":679.76,"l":679.26,"c":679.26,"v":3.67960285,"n":10,"vw":679.6324449731},{"t":"2021-11-11T11:12:00Z","x":"CBSE","o":679.44,"h":679.53,"l":679.44,"c":679.53,"v":0.18841132,"n":8,"vw":679.5228170977},{"t":"2021-11-11T11:13:00Z","x":"CBSE","o":679.61,"h":679.61,"l":679.43,"c":679.49,"v":2.20062522,"n":7,"vw":679.49710414},{"t":"2021-11-11T11:14:00Z","x":"CBSE","o":679.48,"h":679.48,"l":679.22,"c":679.22,"v":1.17646198,"n":3,"vw":679.4148630646},{"t":"2021-11-11T11:15:00Z","x":"CBSE","o":679.19,"h":679.26,"l":679.04,"c":679.26,"v":0.54628614,"n":4,"vw":679.1730029087},{"t":"2021-11-11T11:16:00Z","x":"CBSE","o":679.84,"h":679.85,"l":679.65,"c":679.85,"v":10.73449374,"n":17,"vw":679.7295574889},{"t":"2021-11-11T11:17:00Z","x":"CBSE","o":679.82,"h":679.86,"l":679.23,"c":679.23,"v":10.76066555,"n":14,"vw":679.3284885697},{"t":"2021-11-11T11:18:00Z","x":"CBSE","o":679.05,"h":679.13,"l":678.66,"c":678.81,"v":2.30720435,"n":13,"vw":678.8593098348},{"t":"2021-11-11T11:19:00Z","x":"CBSE","o":678.64,"h":678.68,"l":678.37,"c":678.54,"v":3.12648447,"n":11,"vw":678.3865188897},{"t":"2021-11-11T11:20:00Z","x":"CBSE","o":678.55,"h":679.28,"l":678.41,"c":679.2,"v":1.9829005,"n":14,"vw":678.6421245625},{"t":"2021-11-11T11:21:00Z","x":"CBSE","o":679.48,"h":679.81,"l":679.39,"c":679.71,"v":3.53102371,"n":19,"vw":679.6679296305}],"symbol":"BCHUSD","next_page_token":null}`)
	got, err := c.GetCryptoBars("BCHUSD", GetCryptoBarsParams{
		TimeFrame: OneMin,
		Start:     time.Date(2021, 11, 11, 11, 12, 0, 0, time.UTC),
		End:       time.Date(2021, 11, 11, 11, 21, 7, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Len(t, got, 11)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 11, 11, 11, 11, 0, 0, time.UTC)))
	assert.EqualValues(t, "CBSE", got[1].Exchange)
	assert.EqualValues(t, 679.61, got[2].Open)
	assert.EqualValues(t, 679.48, got[3].High)
	assert.EqualValues(t, 679.04, got[4].Low)
	assert.EqualValues(t, 679.85, got[5].Close)
	assert.EqualValues(t, 10.76066555, got[6].Volume)
	assert.EqualValues(t, 13, got[7].TradeCount)
	assert.EqualValues(t, 678.3865188897, got[8].VWAP)
}

func TestGetCryptoMultiBars(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"bars":{"BCHUSD":[{"t":"2021-11-20T20:00:00Z","x":"CBSE","o":582.48,"h":583.3,"l":580.16,"c":583.29,"v":895.36742328,"n":1442,"vw":581.631507},{"t":"2021-11-20T20:00:00Z","x":"ERSX","o":581.31,"h":581.31,"l":581.31,"c":581.31,"v":4,"n":1,"vw":581.31},{"t":"2021-11-20T20:00:00Z","x":"FTX","o":581.875,"h":582.7,"l":580.05,"c":582.3,"v":315.999,"n":62,"vw":581.17328}],"BTCUSD":[{"t":"2021-11-20T20:00:00Z","x":"CBSE","o":59488.87,"h":59700,"l":59364.08,"c":59660.38,"v":542.20811667,"n":34479,"vw":59522.345185},{"t":"2021-11-20T20:00:00Z","x":"ERSX","o":59446.7,"h":59654.1,"l":59446.7,"c":59654.1,"v":1.1046,"n":4,"vw":59513.516151},{"t":"2021-11-20T20:00:00Z","x":"FTX","o":59488,"h":59683,"l":59374,"c":59638,"v":73.079,"n":264,"vw":59501.646613}],"ETHUSD":[{"t":"2021-11-20T20:00:00Z","x":"CBSE","o":4402.71,"h":4435.25,"l":4392.96,"c":4432.48,"v":9115.28075256,"n":29571,"vw":4411.486276},{"t":"2021-11-20T20:00:00Z","x":"ERSX","o":4404.11,"h":4434.87,"l":4404.11,"c":4434.87,"v":68.8337,"n":49,"vw":4412.167596},{"t":"2021-11-20T20:00:00Z","x":"FTX","o":4402.4,"h":4434,"l":4395.4,"c":4433.8,"v":643.603,"n":405,"vw":4408.340722}],"LTCUSD":[{"t":"2021-11-20T20:00:00Z","x":"CBSE","o":225.78,"h":227.09,"l":225.07,"c":225.79,"v":22495.52449682,"n":7007,"vw":226.00074},{"t":"2021-11-20T20:00:00Z","x":"ERSX","o":226.07,"h":226.67,"l":225.75,"c":225.75,"v":228.2211,"n":5,"vw":226.337181},{"t":"2021-11-20T20:00:00Z","x":"FTX","o":225.805,"h":226.975,"l":225.135,"c":225.865,"v":1792,"n":149,"vw":225.944729}]},"next_page_token":null}`)
	got, err := c.GetCryptoMultiBars([]string{"BTCUSD", "LTCUSD", "BCHUSD", "ETHUSD"}, GetCryptoBarsParams{
		TimeFrame: NewTimeFrame(2, Hour),
		Start:     time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Len(t, got, 4)
	assert.True(t, got["BCHUSD"][0].Timestamp.Equal(time.Date(2021, 11, 20, 20, 0, 0, 0, time.UTC)))
	assert.EqualValues(t, "ERSX", got["BCHUSD"][1].Exchange)
	assert.EqualValues(t, 581.875, got["BCHUSD"][2].Open)
	assert.EqualValues(t, 59700, got["BTCUSD"][0].High)
	assert.EqualValues(t, 59446.7, got["BTCUSD"][1].Low)
	assert.EqualValues(t, 59638, got["BTCUSD"][2].Close)
	assert.EqualValues(t, 9115.28075256, got["ETHUSD"][0].Volume)
	assert.EqualValues(t, 7007, got["LTCUSD"][0].TradeCount)
	assert.EqualValues(t, 226.337181, got["LTCUSD"][1].VWAP)
}

func TestLatestCryptoTrade(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"symbol":"BTCUSD","trade":{"t":"2021-11-22T08:32:39.313396Z","x":"FTX","p":57527,"s":0.0755,"tks":"B","i":17209535}}`)
	got, err := c.GetLatestCryptoTrade("BTCUSD", "FTX")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoTrade{
		ID:        17209535,
		Exchange:  "FTX",
		Price:     57527,
		Size:      0.0755,
		Timestamp: time.Date(2021, 11, 22, 8, 32, 39, 313396000, time.UTC),
		TakerSide: "B",
	}, *got)
}

func TestLatestCryptoQuote(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"symbol":"BCHUSD","quote":{"t":"2021-11-22T08:36:35.117453693Z","x":"ERSX","bp":564.52,"bs":44.2403,"ap":565.87,"as":44.2249}}`)
	got, err := c.GetLatestCryptoQuote("BCHUSD", "ERSX")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoQuote{
		Timestamp: time.Date(2021, 11, 22, 8, 36, 35, 117453693, time.UTC),
		Exchange:  "ERSX",
		BidPrice:  564.52,
		BidSize:   44.2403,
		AskPrice:  565.87,
		AskSize:   44.2249,
	}, *got)
}

func TestLatestCryptoXBBO(t *testing.T) {
	c := testClient()
	c.do = mockResp(`{"symbol":"ETHUSD","xbbo":{"t":"2021-11-22T08:38:40.635798272Z","ax":"ERSX","ap":4209.23,"as":11.8787,"bx":"FTX","bp":4209.3,"bs":3.9}}`)
	got, err := c.GetLatestCryptoXBBO("ETHUSD", nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoXBBO{
		Timestamp:   time.Date(2021, 11, 22, 8, 38, 40, 635798272, time.UTC),
		BidExchange: "FTX",
		BidPrice:    4209.3,
		BidSize:     3.9,
		AskExchange: "ERSX",
		AskPrice:    4209.23,
		AskSize:     11.8787,
	}, *got)
}

func TestCryptoSnapshot(t *testing.T) {
	c := testClient()

	// successful
	c.do = mockResp(`{"symbol":"ETHUSD","latestTrade":{"t":"2021-12-08T19:26:58.703892Z","x":"CBSE","p":4393.18,"s":0.04299154,"tks":"S","i":191026243},"latestQuote":{"t":"2021-12-08T21:39:50.999Z","x":"CBSE","bp":4405.27,"bs":0.32420683,"ap":4405.28,"as":0.54523826},"minuteBar":{"t":"2021-12-08T19:26:00Z","x":"CBSE","o":4393.62,"h":4396.45,"l":4390.81,"c":4393.18,"v":132.02049802,"n":278,"vw":4393.9907155981},"dailyBar":{"t":"2021-12-08T06:00:00Z","x":"CBSE","o":4329.11,"h":4455.62,"l":4231.55,"c":4393.18,"v":95466.0903448,"n":186155,"vw":4367.7642299555},"prevDailyBar":{"t":"2021-12-07T06:00:00Z","x":"CBSE","o":4350.15,"h":4433.99,"l":4261.39,"c":4329.11,"v":152391.30635034,"n":326203,"vw":4344.2956259855}}`)
	got, err := c.GetCryptoSnapshot("ETHUSD", "CBSE")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoSnapshot{
		LatestTrade: &CryptoTrade{
			ID:        191026243,
			Exchange:  "CBSE",
			Price:     4393.18,
			Size:      0.04299154,
			TakerSide: "S",
			Timestamp: time.Date(2021, 12, 8, 19, 26, 58, 703892000, time.UTC),
		},
		LatestQuote: &CryptoQuote{
			Exchange:  "CBSE",
			BidPrice:  4405.27,
			BidSize:   0.32420683,
			AskPrice:  4405.28,
			AskSize:   0.54523826,
			Timestamp: time.Date(2021, 12, 8, 21, 39, 50, 999000000, time.UTC),
		},
		MinuteBar: &CryptoBar{
			Exchange:   "CBSE",
			Open:       4393.62,
			High:       4396.45,
			Low:        4390.81,
			Close:      4393.18,
			Volume:     132.02049802,
			TradeCount: 278,
			VWAP:       4393.9907155981,
			Timestamp:  time.Date(2021, 12, 8, 19, 26, 0, 0, time.UTC),
		},
		DailyBar: &CryptoBar{
			Exchange:   "CBSE",
			Open:       4329.11,
			High:       4455.62,
			Low:        4231.55,
			Close:      4393.18,
			Volume:     95466.0903448,
			TradeCount: 186155,
			VWAP:       4367.7642299555,
			Timestamp:  time.Date(2021, 12, 8, 6, 0, 0, 0, time.UTC),
		},
		PrevDailyBar: &CryptoBar{
			Exchange:   "CBSE",
			Open:       4350.15,
			High:       4433.99,
			Low:        4261.39,
			Close:      4329.11,
			Volume:     152391.30635034,
			TradeCount: 326203,
			VWAP:       4344.2956259855,
			Timestamp:  time.Date(2021, 12, 7, 6, 0, 0, 0, time.UTC),
		},
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetCryptoSnapshot("ETHUSD", "CBSE")
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestGetNews(t *testing.T) {
	c := testClient()
	firstResp := `{"news":[{"id":20472678,"headline":"CEO John Krafcik Leaves Waymo","author":"Bibhu Pattnaik","created_at":"2021-04-03T15:35:21Z","updated_at":"2021-04-03T15:35:21Z","summary":"Waymo\u0026#39;s chief technology officer and its chief operating officer will serve as co-CEOs.","url":"https://www.benzinga.com/news/21/04/20472678/ceo-john-krafcik-leaves-waymo","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/waymo_2.jpeg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/waymo_2.jpeg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/waymo_2.jpeg"}],"symbols":["GOOG","GOOGL","TSLA"]},{"id":20472512,"headline":"Benzinga's Bulls And Bears Of The Week: Apple, GM, JetBlue, Lululemon, Tesla And More","author":"Nelson Hem","created_at":"2021-04-03T15:20:12Z","updated_at":"2021-04-03T15:20:12Z","summary":"\n\tBenzinga has examined the prospects for many investor favorite stocks over the past week. \n\tThe past week\u0026#39;s bullish calls included airlines, Chinese EV makers and a consumer electronics giant.\n","url":"https://www.benzinga.com/trading-ideas/long-ideas/21/04/20472512/benzingas-bulls-and-bears-of-the-week-apple-gm-jetblue-lululemon-tesla-and-more","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/pexels-burst-373912_0.jpg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/pexels-burst-373912_0.jpg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/pexels-burst-373912_0.jpg"}],"symbols":["AAPL","ARKX","BMY","CS","GM","JBLU","JCI","LULU","NIO","TSLA","XPEV"]}],"next_page_token":"MTYxNzQ2MzIxMjAwMDAwMDAwMHwyMDQ3MjUxMg=="}`
	secondResp := `{"news":[{"id":20471562,"headline":"Is Now The Time To Buy Stock In Tesla, Netflix, Alibaba, Ford Or Facebook?","author":"Henry Khederian","created_at":"2021-04-03T12:31:15Z","updated_at":"2021-04-03T12:31:16Z","summary":"One of the most common questions traders have about stocks is “Why Is It Moving?”\n\nThat’s why Benzinga created the Why Is It Moving, or WIIM, feature in Benzinga Pro. WIIMs are a one-sentence description as to why that stock is moving.","url":"https://www.benzinga.com/analyst-ratings/analyst-color/21/04/20471562/is-now-the-time-to-buy-stock-in-tesla-netflix-alibaba-ford-or-facebook","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"}],"symbols":["BABA","NFLX","TSLA"]}],"next_page_token":null}`
	c.do = func(c *client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "data.alpaca.markets", req.URL.Host)
		assert.Equal(t, "/v1beta1/news", req.URL.Path)
		assert.Equal(t, "2021-04-03T00:00:00Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2021-04-04T05:00:00Z", req.URL.Query().Get("end"))
		assert.Equal(t, "2", req.URL.Query().Get("limit"))
		pageToken := req.URL.Query().Get("page_token")
		resp := firstResp
		if pageToken != "" {
			assert.Equal(t, "MTYxNzQ2MzIxMjAwMDAwMDAwMHwyMDQ3MjUxMg==", pageToken)
			resp = secondResp
		}
		return &http.Response{
			Body: ioutil.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetNews(GetNewsParams{
		Symbols:    []string{"AAPL", "TSLA"},
		Start:      time.Date(2021, 4, 3, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 4, 4, 5, 0, 0, 0, time.UTC),
		TotalLimit: 5,
		PageLimit:  2,
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.EqualValues(t, "Bibhu Pattnaik", got[0].Author)
	assert.EqualValues(t, "2021-04-03T15:20:12Z", got[1].CreatedAt.Format(time.RFC3339))
	assert.EqualValues(t, "2021-04-03T12:31:16Z", got[2].UpdatedAt.Format(time.RFC3339))
	assert.EqualValues(t, "CEO John Krafcik Leaves Waymo", got[0].Headline)
	assert.EqualValues(t, "One of the most common questions traders have about stocks is “Why Is It Moving?”\n\nThat’s why Benzinga created the Why Is It Moving, or WIIM, feature in Benzinga Pro. WIIMs are a one-sentence description as to why that stock is moving.", got[2].Summary)
	assert.EqualValues(t, "", got[0].Content)
	assert.ElementsMatch(t, []NewsImage{
		{
			Size: "large",
			URL:  "https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/waymo_2.jpeg",
		},
		{
			Size: "small",
			URL:  "https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/waymo_2.jpeg",
		},
		{
			Size: "thumb",
			URL:  "https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/waymo_2.jpeg",
		},
	}, got[0].Images)
	assert.EqualValues(t, "https://www.benzinga.com/analyst-ratings/analyst-color/21/04/20471562/is-now-the-time-to-buy-stock-in-tesla-netflix-alibaba-ford-or-facebook", got[2].URL)
	assert.EqualValues(t, []string{"GOOG", "GOOGL", "TSLA"}, got[0].Symbols)
}
