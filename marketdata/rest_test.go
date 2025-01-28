//nolint:lll
package marketdata

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"bars":{"SPY":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}}`)
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		BaseURL: server.URL,
	})
	bar, err := client.GetLatestBar("SPY", GetLatestBarRequest{})
	require.NoError(t, err)
	assert.Equal(t, 469.11, bar.Low)
}

func TestDefaultDo_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("APCA_API_DATA_URL", server.URL)
	client := NewClient(ClientOpts{
		OAuth:      "myoauthkey",
		RetryDelay: time.Nanosecond,
	})
	_, err := client.GetLatestBar("SPY", GetLatestBarRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDefaultDo_Retry(t *testing.T) {
	tryCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch tryCount {
		case 0, 2:
			http.Error(w, "too many requests", http.StatusTooManyRequests)
		case 1:
			http.Error(w, "internal server error occurred", http.StatusInternalServerError)
		default:
			fmt.Fprint(w, `{"bars":{"SPY":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}}`)
		}
		tryCount++
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		BaseURL:    server.URL,
		RetryDelay: time.Nanosecond,
		RetryLimit: 5,
	})
	bar, err := client.GetLatestBar("SPY", GetLatestBarRequest{})
	require.NoError(t, err)
	assert.Equal(t, 469.18, bar.High)
}

func TestDefaultDo_TooMany429s(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	_, err := client.GetLatestBar("SPY", GetLatestBarRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
	assert.Equal(t, opts.RetryLimit+1, called) // +1 for the original request
}

func TestDefaultDo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(time.Second)
		fmt.Fprint(w, `{"bars":{"SPY":{"t":"2021-11-20T00:59:00Z","o":469.18,"h":469.18,"l":469.11,"c":469.17,"v":740,"n":11,"vw":469.1355}}}`)
	}))
	defer server.Close()
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{
			Timeout: time.Millisecond,
		},
	})
	_, err := client.GetLatestBar("SPY", GetLatestBarRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Timeout")
}

func mockResp(resp string) func(_ *Client, req *http.Request) (*http.Response, error) {
	return func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
}

func mockErrResp() func(_ *Client, _ *http.Request) (*http.Response, error) {
	return func(_ *Client, _ *http.Request) (*http.Response, error) {
		return &http.Response{}, errors.New("fail")
	}
}

func TestGetTrades_Gzip(t *testing.T) {
	c := NewClient(ClientOpts{
		Feed: SIP,
	})

	f, err := os.Open("testdata/trades.json.gz")
	require.NoError(t, err)
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
		assert.Equal(t, "sip", req.URL.Query().Get("feed"))
		return &http.Response{
			Body: f,
			Header: http.Header{
				"Content-Encoding": []string{"gzip"},
			},
		}, nil
	}
	got, err := c.GetTrades("AAPL", GetTradesRequest{
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
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
		resp := `{"trades":{"AAPL":[{"t":"2021-10-13T08:00:00.08960768Z","x":"P","p":140.2,"s":595,"c":["@","T"],"i":1,"z":"C"}]},"next_page_token":"QUFQTHwyMDIxLTEwLTEzVDA4OjAwOjAwLjA4OTYwNzY4MFp8UHwwOTIyMzM3MjAzNjg1NDc3NTgwOQ=="}`
		// Even though we request gzip encoding, the server may decide to not use it
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetTrades("AAPL", GetTradesRequest{
		Start:      time.Date(2021, 10, 13, 0, 0, 0, 0, time.UTC),
		TotalLimit: 1,
		PageLimit:  1,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	trade := got[0]
	assert.EqualValues(t, 1, trade.ID)
}

func TestGetTrades_Currency(t *testing.T) {
	c := NewClient(ClientOpts{
		Feed:     DelayedSIP,
		Currency: "JPY",
	})
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "delayed_sip", req.URL.Query().Get("feed"))
		assert.Equal(t, "JPY", req.URL.Query().Get("currency"))
		resp := `{"trades":{"AAPL":[{"t":"2021-10-13T08:00:00.08960768Z","x":"P","p":15922.93,"s":595,"c":["@","T"],"i":1,"z":"C"}]},"currency":"JPY","next_page_token":"QUFQTHwyMDIxLTEwLTEzVDA4OjAwOjAwLjA4OTYwNzY4MFp8UHwwOTIyMzM3MjAzNjg1NDc3NTgwOQ=="}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetTrades("AAPL", GetTradesRequest{
		Start:      time.Date(2021, 10, 13, 0, 0, 0, 0, time.UTC),
		TotalLimit: 1,
		PageLimit:  1,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	trade := got[0]
	assert.Equal(t, 15922.93, trade.Price)

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		resp := `{"trades":{},"currency":"MXN","next_page_token":null}`
		assert.Equal(t, "MXN", req.URL.Query().Get("currency"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err = c.GetTrades("AAPL", GetTradesRequest{
		Start:      time.Date(2021, 10, 13, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 10, 13, 0, 0, 0, 1, time.UTC),
		TotalLimit: 1,
		PageLimit:  1,
		Currency:   "MXN",
	})
	require.NoError(t, err)
	assert.Empty(t, got, 0)
}

func TestGetTrades_InvalidURL(t *testing.T) {
	c := NewClient(ClientOpts{
		BaseURL: string([]byte{0, 1, 2, 3}),
	})
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		require.Fail(t, "the server should not have been called")
		return nil, nil
	}
	_, err := c.GetTrades("AAPL", GetTradesRequest{})
	require.Error(t, err)
}

func TestGetTrades_Update(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"trades":{"A":[{"t":"2022-10-21T20:19:03.752176Z","x":"D","p":129.88,"s":5,"c":[" ","T","I"],"i":71697353758036,"z":"A","u":"canceled"},{"t":"2022-10-21T20:19:03.876181Z","x":"D","p":129.88,"s":2,"c":[" ","T","I"],"i":71697353815352,"z":"A","u":"canceled"}]},"next_page_token":null}`)
	got, err := c.GetTrades("A", GetTradesRequest{
		Start: time.Date(2022, 10, 21, 20, 19, 3, 0, time.UTC),
		End:   time.Date(2022, 10, 21, 20, 19, 4, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "canceled", got[0].Update)
	assert.Equal(t, "canceled", got[1].Update)
}

func TestGetTrades_ServerError(t *testing.T) {
	c := DefaultClient
	c.do = mockErrResp()
	_, err := c.GetTrades("SPY", GetTradesRequest{})
	require.Error(t, err)
}

func TestGetTrades_InvalidResponse(t *testing.T) {
	c := DefaultClient
	c.do = mockResp("not a valid json")
	_, err := c.GetTrades("SPY", GetTradesRequest{})
	require.Error(t, err)
}

func TestGetMultiTrades(t *testing.T) {
	c := DefaultClient

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/trades", req.URL.Path)
		assert.Equal(t, "2018-06-04T19:18:17Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2018-06-04T19:18:19Z", req.URL.Query().Get("end"))
		assert.Equal(t, "F,GE", req.URL.Query().Get("symbols"))
		resp := `{"trades":{"F":[{"t":"2018-06-04T19:18:17.4392Z","x":"D","p":11.715,"s":5,"c":[" ","I"],"i":442254,"z":"A"},{"t":"2018-06-04T19:18:18.7453Z","x":"D","p":11.71,"s":200,"c":[" "],"i":442258,"z":"A"}],"GE":[{"t":"2018-06-04T19:18:18.2305Z","x":"D","p":13.74,"s":100,"c":[" "],"i":933063,"z":"A"},{"t":"2018-06-04T19:18:18.6206Z","x":"D","p":13.7317,"s":100,"c":[" ","4","B"],"i":933066,"z":"A"}]},"next_page_token":null}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetMultiTrades([]string{"F", "GE"}, GetTradesRequest{
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
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/quotes", req.URL.Path)
		assert.Equal(t, "IBM", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2021-10-04T18:00:14Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2021-10-04T18:00:15Z", req.URL.Query().Get("end"))
		assert.Equal(t, "10", req.URL.Query().Get("limit"))
		resp := `{"quotes":{"IBM":[{"t":"2021-10-04T18:00:14.012577217Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.016722688Z","ax":"N","ap":143.71,"as":3,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.020123648Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.070107859Z","ax":"N","ap":143.71,"as":2,"bx":"U","bp":143.69,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.0709007Z","ax":"N","ap":143.71,"as":2,"bx":"H","bp":143.68,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.179935833Z","ax":"N","ap":143.71,"as":2,"bx":"T","bp":143.69,"bs":1,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.179937077Z","ax":"N","ap":143.71,"as":2,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180278784Z","ax":"N","ap":143.71,"as":1,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180473523Z","ax":"N","ap":143.71,"as":1,"bx":"U","bp":143.69,"bs":3,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.180522Z","ax":"N","ap":143.71,"as":1,"bx":"Z","bp":143.69,"bs":6,"c":["R"],"z":"A"}]},"next_page_token":"SUJNfDIwMjEtMTAtMDRUMTg6MDA6MTQuMTgwNTIyMDAwWnwxMzQ0OTQ0Mw=="}`
		switch req.URL.Query().Get("page_token") {
		case "":
		case "SUJNfDIwMjEtMTAtMDRUMTg6MDA6MTQuMTgwNTIyMDAwWnwxMzQ0OTQ0Mw==":
			resp = `{"quotes":{"IBM":[{"t":"2021-10-04T18:00:14.180608Z","ax":"N","ap":143.71,"as":1,"bx":"U","bp":143.69,"bs":3,"c":["R"],"z":"A"},{"t":"2021-10-04T18:00:14.210488488Z","ax":"N","ap":143.71,"as":1,"bx":"T","bp":143.69,"bs":2,"c":["R"],"z":"A"}]},"next_page_token":null}`
		default:
			assert.Fail(t, "unexpected page_token")
		}
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetQuotes("IBM", GetQuotesRequest{
		Start:     time.Date(2021, 10, 4, 18, 0, 14, 0, time.UTC),
		End:       time.Date(2021, 10, 4, 18, 0, 15, 0, time.UTC),
		PageLimit: 10,
	})
	require.NoError(t, err)
	require.Len(t, got, 12)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 10, 4, 18, 0, 14, 12577217, time.UTC)))
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

func TestGetQuotes_SortDesc(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "desc", req.URL.Query().Get("sort"))
		resp := `{"next_page_token":"VFNMQXw3NTMxMTU5NjM2OTY0OTE0ODc5fFV8MjI4LjMzfDF8UHwyMjguMzV8NXxS","quotes":{"TSLA":[{"ap":228.35,"as":5,"ax":"P","bp":228.34,"bs":2,"bx":"Q","c":["R"],"t":"2023-08-16T18:59:59.185551091Z","z":"C"},{"ap":228.35,"as":5,"ax":"P","bp":228.33,"bs":1,"bx":"U","c":["R"],"t":"2023-08-16T18:59:59.035085121Z","z":"C"}]}}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetQuotes("TSLA", GetQuotesRequest{
		Start:      time.Date(2023, 8, 16, 9, 0, 0, 0, time.UTC),
		End:        time.Date(2023, 8, 16, 19, 0, 0, 0, time.UTC),
		TotalLimit: 2,
		Sort:       SortDesc,
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.True(t, got[0].Timestamp.After(got[1].Timestamp))
	assert.Equal(t, "Q", got[0].BidExchange)
	assert.Equal(t, "U", got[1].BidExchange)
}

func TestGetMultiQuotes(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/quotes", req.URL.Path)
		assert.Equal(t, "6", req.URL.Query().Get("limit"))
		assert.Equal(t, "BA,DIS", req.URL.Query().Get("symbols"))
		resp := `{"quotes":{"BA":[{"t":"2021-09-15T17:00:00.010461656Z","ax":"N","ap":212.59,"as":1,"bx":"T","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.010657639Z","ax":"N","ap":212.59,"as":1,"bx":"J","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.184164565Z","ax":"N","ap":212.59,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.18418Z","ax":"N","ap":212.59,"as":1,"bx":"N","bp":212.56,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.186067456Z","ax":"Y","ap":212.61,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"},{"t":"2021-09-15T17:00:00.186265Z","ax":"N","ap":212.64,"as":1,"bx":"T","bp":212.57,"bs":1,"c":["R"],"z":"A"}]},"next_page_token":"QkF8MjAyMS0wOS0xNVQxNzowMDowMC4xODYyNjUwMDBafEI4ODRBOUM3"}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetMultiQuotes([]string{"BA", "DIS"}, GetQuotesRequest{
		Start:      time.Date(2021, 9, 15, 17, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 9, 15, 18, 0, 0, 0, time.UTC),
		TotalLimit: 6,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got["BA"], 6)
	assert.Equal(t, 212.59, got["BA"][2].AskPrice)

	c.do = mockErrResp()
	_, err = c.GetMultiQuotes([]string{"BA", "DIS"}, GetQuotesRequest{})
	require.Error(t, err)
}

func TestGetAuctions(t *testing.T) {
	c := DefaultClient
	firstResp := `{"auctions":{"AAPL":[{"d":"2022-10-17","o":[{"t":"2022-10-17T13:30:00.189598208Z","x":"P","p":141.13,"s":10,"c":"Q"},{"t":"2022-10-17T13:30:01.329947459Z","x":"Q","p":141.07,"s":1103165,"c":"O"},{"t":"2022-10-17T13:30:01.334218355Z","x":"Q","p":141.07,"s":1103165,"c":"Q"}],"c":[{"t":"2022-10-17T20:00:00.155310848Z","x":"P","p":142.4,"s":100,"c":"M"},{"t":"2022-10-17T20:00:01.135646791Z","x":"Q","p":142.41,"s":7927137,"c":"6"},{"t":"2022-10-17T20:00:01.742162179Z","x":"Q","p":142.41,"s":7927137,"c":"M"}]},{"d":"2022-10-18","o":[{"t":"2022-10-18T13:30:00.193677568Z","x":"P","p":145.42,"s":1,"c":"Q"},{"t":"2022-10-18T13:30:01.662931714Z","x":"Q","p":145.49,"s":793345,"c":"O"},{"t":"2022-10-18T13:30:01.67388499Z","x":"Q","p":145.49,"s":793345,"c":"Q"}],"c":[{"t":"2022-10-18T20:00:00.15542272Z","x":"P","p":143.79,"s":100,"c":"M"},{"t":"2022-10-18T20:00:00.63129591Z","x":"Q","p":143.75,"s":3979281,"c":"6"},{"t":"2022-10-18T20:00:00.631313365Z","x":"Q","p":143.75,"s":3979281,"c":"M"}]}]},"next_page_token":"QUFQTHwyMDIyLTEwLTE4VDIwOjAwOjAwLjYzMTMxMzM2NVp8UXxATXwxNDMuNzU="}`
	secondResp := `{"auctions":{"AAPL":[{"d":"2022-10-19","o":[{"t":"2022-10-19T13:30:00.206482688Z","x":"P","p":141.69,"s":4,"c":"Q"},{"t":"2022-10-19T13:30:01.350685708Z","x":"Q","p":141.5,"s":517006,"c":"O"},{"t":"2022-10-19T13:30:01.351159286Z","x":"Q","p":141.5,"s":517006,"c":"Q"}],"c":[{"t":"2022-10-19T20:00:00.143265536Z","x":"P","p":143.9,"s":400,"c":"M"},{"t":"2022-10-19T20:00:01.384247418Z","x":"Q","p":143.86,"s":4006543,"c":"6"},{"t":"2022-10-19T20:00:01.384266818Z","x":"Q","p":143.86,"s":4006543,"c":"M"}]},{"d":"2022-10-20","o":[{"t":"2022-10-20T13:30:00.172134656Z","x":"P","p":143.03,"s":6,"c":"Q"},{"t":"2022-10-20T13:30:01.664127742Z","x":"Q","p":142.98,"s":663728,"c":"O"},{"t":"2022-10-20T13:30:01.664575417Z","x":"Q","p":142.98,"s":663728,"c":"Q"}],"c":[{"t":"2022-10-20T20:00:00.137319424Z","x":"P","p":143.33,"s":362,"c":"M"},{"t":"2022-10-20T20:00:00.212258037Z","x":"Q","p":143.39,"s":5250532,"c":"6"},{"t":"2022-10-20T20:00:00.212282215Z","x":"Q","p":143.39,"s":5250532,"c":"M"}]}]},"next_page_token":"QUFQTHwyMDIyLTEwLTIwVDIwOjAwOjAwLjIxMjI4MjIxNVp8UXxATXwxNDMuMzk="}`
	thirdResp := `{"auctions":{"AAPL":[{"d":"2022-10-21","o":[{"t":"2022-10-21T13:30:00.18449664Z","x":"P","p":142.96,"s":59,"c":"Q"},{"t":"2022-10-21T13:30:01.013655041Z","x":"Q","p":142.81,"s":4643721,"c":"O"},{"t":"2022-10-21T13:30:01.025412599Z","x":"Q","p":142.81,"s":4643721,"c":"Q"}],"c":[{"t":"2022-10-21T20:00:00.151828992Z","x":"P","p":147.27,"s":8147,"c":"M"},{"t":"2022-10-21T20:00:00.551850227Z","x":"Q","p":147.27,"s":6395818,"c":"6"},{"t":"2022-10-21T20:00:00.551870027Z","x":"Q","p":147.27,"s":6395818,"c":"M"}]}]},"next_page_token":"QUFQTHwyMDIyLTEwLTIxVDIwOjAwOjAwLjU1MTg3MDAyN1p8UXxATXwxNDcuMjc="}`
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/auctions", req.URL.Path)
		assert.Equal(t, "AAPL", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2022-10-17T00:00:00Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2022-10-28T00:00:00Z", req.URL.Query().Get("end"))
		assert.Equal(t, "sip", req.URL.Query().Get("feed"))
		pageToken := req.URL.Query().Get("page_token")
		var resp string
		switch pageToken {
		case "":
			resp = firstResp
			assert.Equal(t, "2", req.URL.Query().Get("limit"))
		case "QUFQTHwyMDIyLTEwLTE4VDIwOjAwOjAwLjYzMTMxMzM2NVp8UXxATXwxNDMuNzU=":
			resp = secondResp
			assert.Equal(t, "2", req.URL.Query().Get("limit"))
		case "QUFQTHwyMDIyLTEwLTIwVDIwOjAwOjAwLjIxMjI4MjIxNVp8UXxATXwxNDMuMzk=":
			resp = thirdResp
			assert.Equal(t, "1", req.URL.Query().Get("limit"))
		default:
			assert.Fail(t, "unexpected page_token: "+pageToken)
		}
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetAuctions("AAPL", GetAuctionsRequest{
		Start:      time.Date(2022, 10, 17, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2022, 10, 28, 0, 0, 0, 0, time.UTC),
		PageLimit:  2,
		TotalLimit: 5,
	})
	require.NoError(t, err)
	require.Len(t, got, 5)
	first := got[0]
	assert.Equal(t, "2022-10-17", first.Date.String())
	if assert.Len(t, first.Opening, 3) {
		a := first.Opening[0]
		assert.Equal(t, "2022-10-17T13:30:00.189598208Z", a.Timestamp.Format(time.RFC3339Nano))
		assert.Equal(t, "P", a.Exchange)
		assert.Equal(t, 141.13, a.Price)
		assert.EqualValues(t, 10, a.Size)
		assert.Equal(t, "Q", a.Condition)
		assert.Equal(t, "O", first.Opening[1].Condition)
	}
	assert.Len(t, first.Closing, 3)
}

func TestGetMultiAuctions(t *testing.T) {
	c := DefaultClient
	resp := `{"auctions":{"AAPL":[{"d":"2022-10-17","o":[{"t":"2022-10-17T13:30:00.189598208Z","x":"P","p":141.13,"s":10,"c":"Q"},{"t":"2022-10-17T13:30:01.329947459Z","x":"Q","p":141.07,"s":1103165,"c":"O"},{"t":"2022-10-17T13:30:01.334218355Z","x":"Q","p":141.07,"s":1103165,"c":"Q"}],"c":[{"t":"2022-10-17T20:00:00.155310848Z","x":"P","p":142.4,"s":100,"c":"M"},{"t":"2022-10-17T20:00:01.135646791Z","x":"Q","p":142.41,"s":7927137,"c":"6"},{"t":"2022-10-17T20:00:01.742162179Z","x":"Q","p":142.41,"s":7927137,"c":"M"}]}],"IBM":[{"d":"2022-10-17","o":[{"t":"2022-10-17T13:30:00.75936768Z","x":"P","p":121.8,"s":100,"c":"Q"},{"t":"2022-10-17T13:30:00.916387328Z","x":"N","p":121.82,"s":62168,"c":"O"},{"t":"2022-10-17T13:30:00.916387328Z","x":"N","p":121.82,"s":62168,"c":"Q"},{"t":"2022-10-17T13:30:01.093145723Z","x":"T","p":121.66,"s":100,"c":"Q"}],"c":[{"t":"2022-10-17T20:00:00.190113536Z","x":"P","p":121.595,"s":100,"c":"M"},{"t":"2022-10-17T20:00:01.746899562Z","x":"T","p":121.57,"s":4,"c":"M"},{"t":"2022-10-17T20:00:02.02300032Z","x":"N","p":121.52,"s":959421,"c":"6"},{"t":"2022-10-17T20:00:02.136344832Z","x":"N","p":121.52,"s":959421,"c":"M"}]}]},"next_page_token":"SUJNfDIwMjItMTAtMTdUMjM6MDA6MDAuMDAyMjYzODA4WnxOfCBNfDEyMS41Mg=="}`
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/auctions", req.URL.Path)
		assert.Equal(t, "2", req.URL.Query().Get("limit"))
		assert.Equal(t, "AAPL,IBM,TSLA", req.URL.Query().Get("symbols"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetMultiAuctions([]string{"AAPL", "IBM", "TSLA"}, GetAuctionsRequest{
		Start:      time.Date(2022, 10, 17, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2022, 10, 18, 0, 0, 0, 0, time.UTC),
		TotalLimit: 2,
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Len(t, got["IBM"], 1)
	assert.EqualValues(t, 959421, got["IBM"][0].Closing[2].Size)
}

func TestGetBars(t *testing.T) {
	c := DefaultClient

	c.do = mockResp(`{"bars":{"AMZN":[{"t":"2021-10-15T16:00:00Z","o":3378.14,"h":3380.815,"l":3376.3001,"c":3379.72,"v":211689,"n":5435,"vw":3379.041755},{"t":"2021-10-15T16:15:00Z","o":3379.5241,"h":3383.24,"l":3376.49,"c":3377.82,"v":115850,"n":5544,"vw":3379.638266},{"t":"2021-10-15T16:30:00Z","o":3377.982,"h":3380.86,"l":3377,"c":3380,"v":58531,"n":3679,"vw":3379.100605},{"t":"2021-10-15T16:45:00Z","o":3379.73,"h":3387.17,"l":3378.7701,"c":3386.7615,"v":83180,"n":4736,"vw":3381.838113},{"t":"2021-10-15T17:00:00Z","o":3387.56,"h":3390.74,"l":3382.87,"c":3382.87,"v":134339,"n":5832,"vw":3387.086825}]},"next_page_token":null}`)
	got, err := c.GetBars("AMZN", GetBarsRequest{
		TimeFrame:  NewTimeFrame(15, Min),
		Adjustment: Split,
		Start:      time.Date(2021, 10, 15, 16, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 10, 15, 17, 0, 0, 0, time.UTC),
		Feed:       SIP,
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

func TestGetBars_Asof(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v2/stocks/bars", req.URL.Path)
		assert.Equal(t, "2022-06-09", req.URL.Query().Get("asof"))
		resp := `{"bars":{"META":[{"t":"2022-06-08T04:00:00Z","o":194.67,"h":202.03,"l":194.41,"c":196.64,"v":22211813,"n":246906,"vw":198.364578},{"t":"2022-06-09T04:00:00Z","o":194.28,"h":199.45,"l":183.68,"c":184,"v":23458984,"n":281546,"vw":190.750577}]},"next_page_token":"TUVUQXxEfDIwMjItMDYtMDlUMDQ6MDA6MDAuMDAwMDAwMDAwWg=="}`
		switch req.URL.Query().Get("page_token") {
		case "TUVUQXxEfDIwMjItMDYtMDlUMDQ6MDA6MDAuMDAwMDAwMDAwWg==":
			resp = `{"bars":{"META":[{"t":"2022-06-10T04:00:00Z","o":183.04,"h":183.1,"l":175.02,"c":175.57,"v":27398594,"n":365035,"vw":177.335914},{"t":"2022-06-13T04:00:00Z","o":170.59,"h":172.575,"l":164.03,"c":164.26,"v":31514255,"n":436284,"vw":167.257431}]},"next_page_token":"TUVUQXxEfDIwMjItMDYtMTNUMDQ6MDA6MDAuMDAwMDAwMDAwWg=="}`
		case "":
		default:
			assert.Fail(t, "unexpected page token")
		}
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetBars("META", GetBarsRequest{
		TimeFrame:  OneDay,
		Start:      time.Date(2022, 6, 8, 0, 0, 0, 0, time.UTC),
		TotalLimit: 4,
		PageLimit:  2,
		AsOf:       "2022-06-09",
	})
	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, 172.575, got[3].High)
}

func TestGetMultiBars(t *testing.T) {
	c := DefaultClient

	c.do = mockResp(`{"bars":{"AAPL":[{"t":"2021-10-13T04:00:00Z","o":141.21,"h":141.4,"l":139.2,"c":140.91,"v":78993712,"n":595435,"vw":140.361873},{"t":"2021-10-14T04:00:00Z","o":142.08,"h":143.88,"l":141.51,"c":143.76,"v":69696731,"n":445634,"vw":143.216983},{"t":"2021-10-15T04:00:00Z","o":144.13,"h":144.895,"l":143.51,"c":144.84,"v":67393148,"n":426182,"vw":144.320565}],"NIO":[{"t":"2021-10-13T04:00:00Z","o":35.75,"h":36.68,"l":35.47,"c":36.24,"v":33394068,"n":177991,"vw":36.275125},{"t":"2021-10-14T04:00:00Z","o":36.09,"h":36.45,"l":35.605,"c":36.28,"v":29890265,"n":166379,"vw":36.00485},{"t":"2021-10-15T04:00:00Z","o":37,"h":38.29,"l":36.935,"c":37.71,"v":48138793,"n":257074,"vw":37.647123}]},"next_page_token":null}`)
	got, err := c.GetMultiBars([]string{"AAPL", "NIO"}, GetBarsRequest{
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
	c := DefaultClient

	// successful
	c.do = mockResp(`{"bars":{"AAPL":{"t":"2021-10-11T23:59:00Z","o":142.59,"h":142.63,"l":142.57,"c":142.59,"v":2714,"n":22,"vw":142.589071}}}`)
	got, err := c.GetLatestBar("AAPL", GetLatestBarRequest{})
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
	got, err = c.GetLatestBar("AAPL", GetLatestBarRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestBar_Feed(t *testing.T) {
	c := NewClient(ClientOpts{Feed: "iex"})

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "iex", req.URL.Query().Get("feed"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(
				`{"bars":{"AAPL":{"t":"2021-10-11T19:59:00Z","o":142.9,"h":142.91,"l":142.77,"c":142.8,"v":13886,"n":108,"vw":142.856726}}}`,
			)),
		}, nil
	}
	_, err := c.GetLatestBar("AAPL", GetLatestBarRequest{})
	require.NoError(t, err)

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "sip", req.URL.Query().Get("feed"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(
				`{"bars":{"AAPL":{"t":"2021-10-11T19:59:00Z","o":142.8,"h":142.91,"l":142.77,"c":142.8,"v":13886,"n":108,"vw":142.856726}}}`,
			)),
		}, nil
	}
	_, err = c.GetLatestBar("AAPL", GetLatestBarRequest{Feed: SIP})
	require.NoError(t, err)
}

func TestLatestBar_Currency(t *testing.T) {
	c := NewClient(ClientOpts{Currency: "MXN"})

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "MXN", req.URL.Query().Get("currency"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(
				`{"bars":{"AAPL":{"t":"2023-01-14T00:59:00Z","o":2536.46,"h":2536.46,"l":2536.46,"c":2536.46,"v":441,"n":57,"vw":2536.19}},"currency":"MXN"}`,
			)),
		}, nil
	}
	bar, err := c.GetLatestBar("AAPL", GetLatestBarRequest{})
	require.NoError(t, err)
	assert.Equal(t, 2536.46, bar.Open)
	assert.Equal(t, 2536.46, bar.High)
	assert.Equal(t, 2536.19, bar.VWAP)

	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "JPY", req.URL.Query().Get("currency"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(
				`{"bars":{"AAPL":{"t":"2023-01-14T00:59:00Z","o":17289.54,"h":17289.54,"l":17289.54,"c":17289.54,"v":441,"n":57,"vw":17287.69}},"currency":"JPY"}`,
			)),
		}, nil
	}
	bar, err = c.GetLatestBar("AAPL", GetLatestBarRequest{
		Currency: "JPY",
	})
	require.NoError(t, err)
	assert.Equal(t, 17289.54, bar.Open)
	assert.Equal(t, 17289.54, bar.High)
	assert.Equal(t, 17287.69, bar.VWAP)
}

func TestLatestBars(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"bars":{"NIO":{"t":"2021-10-11T23:59:00Z","o":35.57,"h":35.6,"l":35.56,"c":35.6,"v":1288,"n":9,"vw":35.586483},"AAPL":{"t":"2021-10-11T23:59:00Z","o":142.59,"h":142.63,"l":142.57,"c":142.59,"v":2714,"n":22,"vw":142.589071}}}`)
	got, err := c.GetLatestBars([]string{"AAPL", "NIO"}, GetLatestBarRequest{})
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
	got, err = c.GetLatestBars([]string{"IBM", "MSFT"}, GetLatestBarRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestTrade(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"trades":{"AAPL":{"t": "2021-04-20T12:40:34.484136Z","x": "J","p": 134.7,"s": 20,"c": ["@","T","I"],"i": 32,"z": "C"}}}`)
	got, err := c.GetLatestTrade("AAPL", GetLatestTradeRequest{})
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
	got, err = c.GetLatestTrade("AAPL", GetLatestTradeRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestTrades(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"trades":{"IBM":{"t":"2021-10-11T23:42:47.895547Z","x":"K","p":142.2,"s":197,"c":[" ","F","T"],"i":52983525503560,"z":"A"},"MSFT":{"t":"2021-10-11T23:59:39.380716032Z","x":"P","p":294.1,"s":100,"c":["@","T"],"i":28693,"z":"C"}}}`)
	got, err := c.GetLatestTrades([]string{"IBM", "MSFT"}, GetLatestTradeRequest{})
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
	got, err = c.GetLatestTrades([]string{"IBM", "MSFT"}, GetLatestTradeRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestQuote(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"quotes":{"AAPL":{"t": "2021-04-20T13:01:57.822745906Z","ax": "Q","ap": 134.68,"as": 1,"bx": "K","bp": 134.66,"bs": 29,"c": ["R"]}}}`)
	got, err := c.GetLatestQuote("AAPL", GetLatestQuoteRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, Quote{
		BidExchange: "K",
		BidPrice:    134.66,
		BidSize:     29,
		AskExchange: "Q",
		AskPrice:    134.68,
		AskSize:     1,
		Timestamp:   time.Date(2021, 4, 20, 13, 1, 57, 822745906, time.UTC),
		Conditions:  []string{"R"},
	}, *got)

	// api failure
	c.do = mockErrResp()
	got, err = c.GetLatestQuote("AAPL", GetLatestQuoteRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestQuotes(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"quotes":{"F":{"t":"2021-10-12T00:00:00.002071Z","ax":"P","ap":15.07,"as":3,"bx":"P","bp":15.01,"bs":3,"c":["R"],"z":"A"},"TSLA":{"t":"2021-10-11T23:59:58.02063232Z","ax":"P","ap":792.6,"as":1,"bx":"P","bp":792,"bs":67,"c":["R"],"z":"C"},"GE":{"t":"2021-10-11T23:02:28.423505152Z","ax":"P","ap":104.06,"as":2,"bx":"P","bp":104.03,"bs":5,"c":["R"],"z":"A"}}}`)
	got, err := c.GetLatestQuotes([]string{"F", "GE", "TSLA"}, GetLatestQuoteRequest{})
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
	got, err = c.GetLatestQuotes([]string{"F", "GE", "TSLA"}, GetLatestQuoteRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestSnapshot(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"AAPL":{"latestTrade": {"t": "2021-05-03T14:45:50.456Z","x": "D","p": 133.55,"s": 200,"c": ["@"],"i": 61462,"z": "C"},"latestQuote": {"t": "2021-05-03T14:45:50.532316972Z","ax": "P","ap": 133.55,"as": 7,"bx": "Q","bp": 133.54,"bs": 9,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:44:00Z","o": 133.485,"h": 133.4939,"l": 133.42,"c": 133.445,"v": 182818},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 132.04,"h": 134.07,"l": 131.83,"c": 133.445,"v": 25094213},"prevDailyBar": {"t": "2021-04-30T04:00:00Z","o": 131.82,"h": 133.56,"l": 131.065,"c": 131.46,"v": 109506363}}}`)
	got, err := c.GetSnapshot("AAPL", GetSnapshotRequest{})
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
	got, err = c.GetSnapshot("AAPL", GetSnapshotRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestSnapshots(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"AAPL": {"latestTrade": {"t": "2021-05-03T14:48:06.563Z","x": "D","p": 133.4201,"s": 145,"c": ["@"],"i": 62700,"z": "C"},"latestQuote": {"t": "2021-05-03T14:48:07.257820915Z","ax": "Q","ap": 133.43,"as": 7,"bx": "Q","bp": 133.42,"bs": 15,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:47:00Z","o": 133.4401,"h": 133.48,"l": 133.37,"c": 133.42,"v": 207020,"n": 1234,"vw": 133.3987},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 132.04,"h": 134.07,"l": 131.83,"c": 133.42,"v": 25846800,"n": 254678,"vw": 132.568},"prevDailyBar": {"t": "2021-04-30T04:00:00Z","o": 131.82,"h": 133.56,"l": 131.065,"c": 131.46,"v": 109506363,"n": 1012323,"vw": 132.025}},"MSFT": {"latestTrade": {"t": "2021-05-03T14:48:06.36Z","x": "D","p": 253.8738,"s": 100,"c": ["@"],"i": 22973,"z": "C"},"latestQuote": {"t": "2021-05-03T14:48:07.243353456Z","ax": "N","ap": 253.89,"as": 2,"bx": "Q","bp": 253.87,"bs": 2,"c": ["R"]},"minuteBar": {"t": "2021-05-03T14:47:00Z","o": 253.78,"h": 253.869,"l": 253.78,"c": 253.855,"v": 25717,"n": 137,"vw": 253.823},"dailyBar": {"t": "2021-05-03T04:00:00Z","o": 253.34,"h": 254.35,"l": 251.8,"c": 253.855,"v": 6100459,"n": 33453,"vw": 253.0534},"prevDailyBar": null},"INVALID": null}`)

	got, err := c.GetSnapshots([]string{"AAPL", "MSFT", "INVALID"}, GetSnapshotRequest{})
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
	got, err = c.GetSnapshots([]string{"AAPL", "CLDR"}, GetSnapshotRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestGetCryptoTrades(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta3/crypto/us/trades", req.URL.Path)
		assert.Equal(t, "BTC/USD", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2021-09-08T05:04:03Z", req.URL.Query().Get("start"))
		resp := `{"next_page_token":"QlRDL1VTRHwyMDIxLTA5LTA4VDA1OjA0OjAzLjI2OTAwMDAwMFp8MDkyMjMzNzIwMzY4NzUwMTgyNDA=","trades":{"BTC/USD":[{"i":20242430,"p":46386.98,"s":0.035083,"t":"2021-09-08T05:04:03.269Z","tks":"S"},{"i":20242431,"p":46382.43,"s":0.129349,"t":"2021-09-08T05:04:03.269Z","tks":"S"},{"i":20242432,"p":46379.8,"s":0.005868,"t":"2021-09-08T05:04:03.269Z","tks":"S"}]}}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetCryptoTrades("BTC/USD", GetCryptoTradesRequest{
		Start:      time.Date(2021, 9, 8, 5, 4, 3, 0, time.UTC),
		End:        time.Date(2021, 9, 8, 5, 6, 7, 0, time.UTC),
		TotalLimit: 3,
		PageLimit:  100,
	})
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, "2021-09-08T05:04:03.269Z", got[0].Timestamp.Format(time.RFC3339Nano))
	assert.EqualValues(t, 46386.98, got[0].Price)
	assert.EqualValues(t, 0.035083, got[0].Size)
	assert.EqualValues(t, 20242430, got[0].ID)
	assert.EqualValues(t, "S", got[0].TakerSide)
}

func TestGetCryptoMultiTrades(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta3/crypto/us/trades", req.URL.Path)
		assert.Equal(t, "SUSHI/USD,BAT/USD", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2023-01-01T20:00:00Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2023-01-01T21:00:00Z", req.URL.Query().Get("end"))
		resp := ""
		switch req.URL.Query().Get("page_token") {
		case "":
			resp = `{"next_page_token":"QkFUL1VTRHwyMDIzLTAxLTAxVDIwOjU1OjAzLjc1NTAwMDAwMFp8MDkyMjMzNzIwMzY4NTY3MzM2Njg=","trades":{"BAT/USD":[{"i":1957857,"p":0.1684,"s":1929.5,"t":"2023-01-01T20:29:36.793Z","tks":"S"},{"i":1957858,"p":0.1685,"s":90.41,"t":"2023-01-01T20:46:54.089Z","tks":"S"},{"i":1957859,"p":0.169,"s":200,"t":"2023-01-01T20:54:28.301Z","tks":"B"},{"i":1957860,"p":0.169,"s":591.75,"t":"2023-01-01T20:55:03.755Z","tks":"B"}]}}`
		case "QkFUL1VTRHwyMDIzLTAxLTAxVDIwOjU1OjAzLjc1NTAwMDAwMFp8MDkyMjMzNzIwMzY4NTY3MzM2Njg=":
			resp = `{"next_page_token":"U1VTSEkvVVNEfDIwMjMtMDEtMDFUMjA6MjY6NTguMDgwMDAwMDAwWnwwOTIyMzM3MjAzNjg1NzM2NTkzMA==","trades":{"SUSHI/USD":[{"i":2590119,"p":0.94,"s":192.963,"t":"2023-01-01T20:03:20.783Z","tks":"S"},{"i":2590120,"p":0.94,"s":73.687,"t":"2023-01-01T20:03:20.819Z","tks":"S"},{"i":2590121,"p":0.943,"s":35.54,"t":"2023-01-01T20:26:33.516Z","tks":"B"},{"i":2590122,"p":0.943,"s":291.15,"t":"2023-01-01T20:26:58.08Z","tks":"S"}]}}`
		case "U1VTSEkvVVNEfDIwMjMtMDEtMDFUMjA6MjY6NTguMDgwMDAwMDAwWnwwOTIyMzM3MjAzNjg1NzM2NTkzMA==":
			resp = `{"next_page_token":"U1VTSEkvVVNEfDIwMjMtMDEtMDFUMjA6NDQ6MjQuMTY2MDAwMDAwWnwwOTIyMzM3MjAzNjg1NzM2NTkzNA==","trades":{"SUSHI/USD":[{"i":2590123,"p":0.942,"s":260.319,"t":"2023-01-01T20:30:00.864Z","tks":"S"},{"i":2590124,"p":0.942,"s":307.35,"t":"2023-01-01T20:36:10.537Z","tks":"S"},{"i":2590125,"p":0.941,"s":282.1,"t":"2023-01-01T20:39:01.948Z","tks":"S"},{"i":2590126,"p":0.938,"s":19.2,"t":"2023-01-01T20:44:24.166Z","tks":"S"}]}}`
		case "U1VTSEkvVVNEfDIwMjMtMDEtMDFUMjA6NDQ6MjQuMTY2MDAwMDAwWnwwOTIyMzM3MjAzNjg1NzM2NTkzNA==":
			resp = `{"next_page_token":null,"trades":{}}`
		}
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetCryptoMultiTrades([]string{"SUSHI/USD", "BAT/USD"}, GetCryptoTradesRequest{
		Start:     time.Date(2023, 1, 1, 20, 0, 0, 0, time.UTC),
		End:       time.Date(2023, 1, 1, 21, 0, 0, 0, time.UTC),
		PageLimit: 4,
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Len(t, got["BAT/USD"], 4)
	assert.Equal(t, 0.1684, got["BAT/USD"][0].Price)
	require.Len(t, got["SUSHI/USD"], 8)
	require.Equal(t, 0.938, got["SUSHI/USD"][7].Price)
	require.Equal(t, 19.2, got["SUSHI/USD"][7].Size)
}

func TestCryptoQuotes(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta3/crypto/us/quotes", req.URL.Path)
		assert.Equal(t, "ETH/USD", req.URL.Query().Get("symbols"))
		assert.Equal(t, "2023-08-16T00:00:00Z", req.URL.Query().Get("start"))
		assert.Equal(t, "2023-08-16T19:00:00Z", req.URL.Query().Get("end"))
		assert.Equal(t, "2", req.URL.Query().Get("limit"))
		assert.Equal(t, "desc", req.URL.Query().Get("sort"))
		resp := `{"next_page_token":"RVRIL1VTRHw3NTMxMTU5NjM3MzM4MDk0NzAyfDE4MjAuOHw0LjM5OTQwNnwxODIyLjU1NDU3OHw0LjMyOTk5MDI=","quotes":{"ETH/USD":[{"ap":1822.352602,"as":8.76015633,"bp":1820.8,"bs":4.399406,"t":"2023-08-16T18:59:58.662421578Z"},{"ap":1822.554578,"as":4.3299902,"bp":1820.8,"bs":4.399406,"t":"2023-08-16T18:59:58.661905298Z"}]}}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetCryptoQuotes("ETH/USD", GetCryptoQuotesRequest{
		Start:      time.Date(2023, 8, 16, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2023, 8, 16, 19, 0, 0, 0, time.UTC),
		TotalLimit: 2,
		Sort:       SortDesc,
	})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, "2023-08-16T18:59:58.662421578Z", got[0].Timestamp.Format(time.RFC3339Nano))
	assert.Equal(t, "2023-08-16T18:59:58.661905298Z", got[1].Timestamp.Format(time.RFC3339Nano))
	assert.Equal(t, 1820.8, got[1].BidPrice)
	assert.Equal(t, 4.399406, got[1].BidSize)
	assert.Equal(t, 1822.554578, got[1].AskPrice)
	assert.Equal(t, 4.3299902, got[1].AskSize)
}

func TestGetCryptoBars(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"bars":{"BCH/USD":[{"t":"2021-11-11T11:11:00Z","o":679.75,"h":679.76,"l":679.26,"c":679.26,"v":3.67960285,"n":10,"vw":679.6324449731},{"t":"2021-11-11T11:12:00Z","o":679.44,"h":679.53,"l":679.44,"c":679.53,"v":0.18841132,"n":8,"vw":679.5228170977},{"t":"2021-11-11T11:13:00Z","o":679.61,"h":679.61,"l":679.43,"c":679.49,"v":2.20062522,"n":7,"vw":679.49710414},{"t":"2021-11-11T11:14:00Z","o":679.48,"h":679.48,"l":679.22,"c":679.22,"v":1.17646198,"n":3,"vw":679.4148630646},{"t":"2021-11-11T11:15:00Z","o":679.19,"h":679.26,"l":679.04,"c":679.26,"v":0.54628614,"n":4,"vw":679.1730029087},{"t":"2021-11-11T11:16:00Z","o":679.84,"h":679.85,"l":679.65,"c":679.85,"v":10.73449374,"n":17,"vw":679.7295574889},{"t":"2021-11-11T11:17:00Z","o":679.82,"h":679.86,"l":679.23,"c":679.23,"v":10.76066555,"n":14,"vw":679.3284885697},{"t":"2021-11-11T11:18:00Z","o":679.05,"h":679.13,"l":678.66,"c":678.81,"v":2.30720435,"n":13,"vw":678.8593098348},{"t":"2021-11-11T11:19:00Z","o":678.64,"h":678.68,"l":678.37,"c":678.54,"v":3.12648447,"n":11,"vw":678.3865188897},{"t":"2021-11-11T11:20:00Z","o":678.55,"h":679.28,"l":678.41,"c":679.2,"v":1.9829005,"n":14,"vw":678.6421245625},{"t":"2021-11-11T11:21:00Z","o":679.48,"h":679.81,"l":679.39,"c":679.71,"v":3.53102371,"n":19,"vw":679.6679296305}]},"next_page_token":null}`)
	got, err := c.GetCryptoBars("BCH/USD", GetCryptoBarsRequest{
		TimeFrame: OneMin,
		Start:     time.Date(2021, 11, 11, 11, 12, 0, 0, time.UTC),
		End:       time.Date(2021, 11, 11, 11, 21, 7, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Len(t, got, 11)
	assert.True(t, got[0].Timestamp.Equal(time.Date(2021, 11, 11, 11, 11, 0, 0, time.UTC)))
	assert.EqualValues(t, 679.61, got[2].Open)
	assert.EqualValues(t, 679.48, got[3].High)
	assert.EqualValues(t, 679.04, got[4].Low)
	assert.EqualValues(t, 679.85, got[5].Close)
	assert.EqualValues(t, 10.76066555, got[6].Volume)
	assert.EqualValues(t, 13, got[7].TradeCount)
	assert.EqualValues(t, 678.3865188897, got[8].VWAP)
}

func TestGetCryptoMultiBars(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"bars":{"BCH/USD":[{"t":"2021-11-20T20:00:00Z","o":582.48,"h":583.3,"l":580.16,"c":583.29,"v":895.36742328,"n":1442,"vw":581.631507},{"t":"2021-11-20T20:00:00Z","o":581.31,"h":581.31,"l":581.31,"c":581.31,"v":4,"n":1,"vw":581.31},{"t":"2021-11-20T20:00:00Z","o":581.875,"h":582.7,"l":580.05,"c":582.3,"v":315.999,"n":62,"vw":581.17328}],"BTC/USD":[{"t":"2021-11-20T20:00:00Z","o":59488.87,"h":59700,"l":59364.08,"c":59660.38,"v":542.20811667,"n":34479,"vw":59522.345185},{"t":"2021-11-20T20:00:00Z","o":59446.7,"h":59654.1,"l":59446.7,"c":59654.1,"v":1.1046,"n":4,"vw":59513.516151},{"t":"2021-11-20T20:00:00Z","o":59488,"h":59683,"l":59374,"c":59638,"v":73.079,"n":264,"vw":59501.646613}],"ETH/USD":[{"t":"2021-11-20T20:00:00Z","o":4402.71,"h":4435.25,"l":4392.96,"c":4432.48,"v":9115.28075256,"n":29571,"vw":4411.486276},{"t":"2021-11-20T20:00:00Z","o":4404.11,"h":4434.87,"l":4404.11,"c":4434.87,"v":68.8337,"n":49,"vw":4412.167596},{"t":"2021-11-20T20:00:00Z","o":4402.4,"h":4434,"l":4395.4,"c":4433.8,"v":643.603,"n":405,"vw":4408.340722}],"LTC/USD":[{"t":"2021-11-20T20:00:00Z","o":225.78,"h":227.09,"l":225.07,"c":225.79,"v":22495.52449682,"n":7007,"vw":226.00074},{"t":"2021-11-20T20:00:00Z","o":226.07,"h":226.67,"l":225.75,"c":225.75,"v":228.2211,"n":5,"vw":226.337181},{"t":"2021-11-20T20:00:00Z","o":225.805,"h":226.975,"l":225.135,"c":225.865,"v":1792,"n":149,"vw":225.944729}]},"next_page_token":null}`)
	got, err := c.GetCryptoMultiBars([]string{"BTC/USD", "LTC/USD", "BCH/USD", "ETH/USD"}, GetCryptoBarsRequest{
		TimeFrame: NewTimeFrame(2, Hour),
		Start:     time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	assert.Len(t, got, 4)
	assert.True(t, got["BCH/USD"][0].Timestamp.Equal(time.Date(2021, 11, 20, 20, 0, 0, 0, time.UTC)))
	assert.EqualValues(t, 581.875, got["BCH/USD"][2].Open)
	assert.EqualValues(t, 59700, got["BTC/USD"][0].High)
	assert.EqualValues(t, 59446.7, got["BTC/USD"][1].Low)
	assert.EqualValues(t, 59638, got["BTC/USD"][2].Close)
	assert.EqualValues(t, 9115.28075256, got["ETH/USD"][0].Volume)
	assert.EqualValues(t, 7007, got["LTC/USD"][0].TradeCount)
	assert.EqualValues(t, 226.337181, got["LTC/USD"][1].VWAP)
}

func TestLatestCryptoBar(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1beta3/crypto/us/latest/bars", req.URL.Path)
		resp := `{"bars":{"BTC/USD":{"t":"2022-02-25T12:50:00Z","o":38899.6,"h":39300,"l":38892.2,"c":39278.88,"v":74.02830613,"n":1086,"vw":39140.0960796263}}}`
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetLatestCryptoBar("BTC/USD", GetLatestCryptoBarRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoBar{
		Timestamp:  time.Date(2022, 2, 25, 12, 50, 0, 0, time.UTC),
		Open:       38899.6,
		High:       39300,
		Low:        38892.2,
		Close:      39278.88,
		Volume:     74.02830613,
		TradeCount: 1086,
		VWAP:       39140.0960796263,
	}, *got)
}

func TestLatestCryptoBars(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"bars":{"SUSHI/USD":{"t":"2022-02-25T12:46:00Z","o":3.2109,"h":3.2109,"l":3.2093,"c":3.2093,"v":6,"n":2,"vw":3.2105},"DOGE/USD":{"t":"2022-02-25T12:40:00Z","o":0.124078,"h":0.124078,"l":0.124078,"c":0.124078,"v":16,"n":1,"vw":0.124078},"BAT/USD":{"t":"2022-02-25T12:36:00Z","o":0.67675,"h":0.67675,"l":0.67675,"c":0.67675,"v":411,"n":1,"vw":0.67675}}}`)
	got, err := c.GetLatestCryptoBars([]string{"BAT/USD", "DOGE/USD", "SUSHI/USD"}, GetLatestCryptoBarRequest{})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, CryptoBar{
		Timestamp:  time.Date(2022, 2, 25, 12, 46, 0, 0, time.UTC),
		Open:       3.2109,
		High:       3.2109,
		Low:        3.2093,
		Close:      3.2093,
		Volume:     6,
		TradeCount: 2,
		VWAP:       3.2105,
	}, got["SUSHI/USD"])
	assert.Equal(t, 0.124078, got["DOGE/USD"].Open)
	assert.Equal(t, 0.67675, got["BAT/USD"].Low)
}

func TestLatestCryptoTrade(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"trades":{"BTC/USD":{"t":"2021-11-22T08:32:39.313396Z","p":57527,"s":0.0755,"tks":"B","i":17209535}}}`)
	got, err := c.GetLatestCryptoTrade("BTC/USD", GetLatestCryptoTradeRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoTrade{
		ID:        17209535,
		Price:     57527,
		Size:      0.0755,
		Timestamp: time.Date(2021, 11, 22, 8, 32, 39, 313396000, time.UTC),
		TakerSide: "B",
	}, *got)
}

func TestLatestCryptoTrades(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"trades":{"ETH/USD":{"t":"2022-02-25T12:54:12.412144626Z","p":2709.1,"s":18,"tks":"B","i":0},"BCH/USD":{"t":"2022-02-25T10:00:07.491340366Z","p":295.42,"s":0.6223,"tks":"B","i":0}}}`)
	got, err := c.GetLatestCryptoTrades([]string{"BCH/USD", "ETH/USD"}, GetLatestCryptoTradeRequest{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, CryptoTrade{
		Price:     295.42,
		Size:      0.6223,
		Timestamp: time.Date(2022, 2, 25, 10, 0, 7, 491340366, time.UTC),
		TakerSide: "B",
	}, got["BCH/USD"])
	assert.Equal(t, 2709.1, got["ETH/USD"].Price)
}

func TestLatestCryptoQuote(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"quotes":{"BCH/USD":{"t":"2021-11-22T08:36:35.117453693Z","bp":564.52,"bs":44.2403,"ap":565.87,"as":44.2249}}}`)
	got, err := c.GetLatestCryptoQuote("BCH/USD", GetLatestCryptoQuoteRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoQuote{
		Timestamp: time.Date(2021, 11, 22, 8, 36, 35, 117453693, time.UTC),
		BidPrice:  564.52,
		BidSize:   44.2403,
		AskPrice:  565.87,
		AskSize:   44.2249,
	}, *got)
}

func TestLatestCryptoQuotes(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"quotes":{"BTC/USD":{"t":"2022-02-25T12:56:22.338903764Z","bp":39381.18,"bs":1.522012,"ap":39463.42,"as":1.5},"LTC/USD":{"t":"2022-02-25T12:56:22.318022772Z","bp":105.98,"bs":377.013267,"ap":106.24,"as":376.902245}}}`)
	got, err := c.GetLatestCryptoQuotes([]string{"BTC/USD", "LTC/USD"}, GetLatestCryptoQuoteRequest{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, CryptoQuote{
		Timestamp: time.Date(2022, 2, 25, 12, 56, 22, 318022772, time.UTC),
		BidPrice:  105.98,
		BidSize:   377.013267,
		AskPrice:  106.24,
		AskSize:   376.902245,
	}, got["LTC/USD"])
	assert.Equal(t, 1.522012, got["BTC/USD"].BidSize)
}

func TestCryptoSnapshot(t *testing.T) {
	c := DefaultClient

	// successful
	c.do = mockResp(`{"snapshots":{"ETH/USD":{"latestTrade":{"t":"2021-12-08T19:26:58.703892Z","p":4393.18,"s":0.04299154,"tks":"S","i":191026243},"latestQuote":{"t":"2021-12-08T21:39:50.999Z","bp":4405.27,"bs":0.32420683,"ap":4405.28,"as":0.54523826},"minuteBar":{"t":"2021-12-08T19:26:00Z","o":4393.62,"h":4396.45,"l":4390.81,"c":4393.18,"v":132.02049802,"n":278,"vw":4393.9907155981},"dailyBar":{"t":"2021-12-08T06:00:00Z","o":4329.11,"h":4455.62,"l":4231.55,"c":4393.18,"v":95466.0903448,"n":186155,"vw":4367.7642299555},"prevDailyBar":{"t":"2021-12-07T06:00:00Z","o":4350.15,"h":4433.99,"l":4261.39,"c":4329.11,"v":152391.30635034,"n":326203,"vw":4344.2956259855}}}}`)
	got, err := c.GetCryptoSnapshot("ETH/USD", GetCryptoSnapshotRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoSnapshot{
		LatestTrade: &CryptoTrade{
			ID:        191026243,
			Price:     4393.18,
			Size:      0.04299154,
			TakerSide: "S",
			Timestamp: time.Date(2021, 12, 8, 19, 26, 58, 703892000, time.UTC),
		},
		LatestQuote: &CryptoQuote{
			BidPrice:  4405.27,
			BidSize:   0.32420683,
			AskPrice:  4405.28,
			AskSize:   0.54523826,
			Timestamp: time.Date(2021, 12, 8, 21, 39, 50, 999000000, time.UTC),
		},
		MinuteBar: &CryptoBar{
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
	got, err = c.GetCryptoSnapshot("ETH/USD", GetCryptoSnapshotRequest{})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLatestCryptoSnapshots(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"snapshots":{"LTC/USD":{"latestTrade":{"t":"2022-02-25T13:37:01.642928Z","p":106.8,"s":25,"tks":"S","i":25661025},"latestQuote":{"t":"2022-02-25T13:37:13.222241536Z","bp":106.745,"bs":55,"ap":106.82,"as":55.86},"minuteBar":{"t":"2022-02-25T13:36:00Z","o":106.745,"h":106.9,"l":106.745,"c":106.9,"v":11.55,"n":4,"vw":106.8133030303},"dailyBar":{"t":"2022-02-25T06:00:00Z","o":103.425,"h":106.9,"l":101.7,"c":106.9,"v":5566.94,"n":274,"vw":104.4620249455},"prevDailyBar":{"t":"2022-02-24T06:00:00Z","o":95.315,"h":107.835,"l":91.64,"c":103.455,"v":36939.92,"n":1401,"vw":98.6918021939}},"BTC/USD":{"latestTrade":{"t":"2022-02-25T13:36:40.670492Z","p":39712,"s":0.0012,"tks":"B","i":25661005},"latestQuote":{"t":"2022-02-25T13:37:13.223584768Z","bp":39651,"bs":0.405,"ap":39668,"as":0.405},"minuteBar":{"t":"2022-02-25T13:36:00Z","o":39635,"h":39721,"l":39635,"c":39712,"v":1.977,"n":9,"vw":39691.4923621649},"dailyBar":{"t":"2022-02-25T06:00:00Z","o":38453,"h":39721,"l":38001,"c":39712,"v":472.5707,"n":1989,"vw":39105.5248569156},"prevDailyBar":{"t":"2022-02-24T06:00:00Z","o":34708,"h":39810,"l":34421,"c":38379,"v":3004.0718,"n":10341,"vw":37321.0250683755}}}}`)
	got, err := c.GetCryptoSnapshots([]string{"BTC/USD", "LTC/USD"}, GetCryptoSnapshotRequest{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, 106.8, got["LTC/USD"].LatestTrade.Price)
	assert.Equal(t, 0.405, got["BTC/USD"].LatestQuote.BidSize)
}

func TestGetNews(t *testing.T) {
	c := DefaultClient
	firstResp := `{"news":[{"id":20472678,"headline":"CEO John Krafcik Leaves Waymo","author":"Bibhu Pattnaik","created_at":"2021-04-03T15:35:21Z","updated_at":"2021-04-03T15:35:21Z","summary":"Waymo\u0026#39;s chief technology officer and its chief operating officer will serve as co-CEOs.","url":"https://www.benzinga.com/news/21/04/20472678/ceo-john-krafcik-leaves-waymo","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/waymo_2.jpeg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/waymo_2.jpeg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/waymo_2.jpeg"}],"symbols":["GOOG","GOOGL","TSLA"]},{"id":20472512,"headline":"Benzinga's Bulls And Bears Of The Week: Apple, GM, JetBlue, Lululemon, Tesla And More","author":"Nelson Hem","created_at":"2021-04-03T15:20:12Z","updated_at":"2021-04-03T15:20:12Z","summary":"\n\tBenzinga has examined the prospects for many investor favorite stocks over the past week. \n\tThe past week\u0026#39;s bullish calls included airlines, Chinese EV makers and a consumer electronics giant.\n","url":"https://www.benzinga.com/trading-ideas/long-ideas/21/04/20472512/benzingas-bulls-and-bears-of-the-week-apple-gm-jetblue-lululemon-tesla-and-more","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/pexels-burst-373912_0.jpg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/pexels-burst-373912_0.jpg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/pexels-burst-373912_0.jpg"}],"symbols":["AAPL","ARKX","BMY","CS","GM","JBLU","JCI","LULU","NIO","TSLA","XPEV"]}],"next_page_token":"MTYxNzQ2MzIxMjAwMDAwMDAwMHwyMDQ3MjUxMg=="}`
	secondResp := `{"news":[{"id":20471562,"headline":"Is Now The Time To Buy Stock In Tesla, Netflix, Alibaba, Ford Or Facebook?","author":"Henry Khederian","created_at":"2021-04-03T12:31:15Z","updated_at":"2021-04-03T12:31:16Z","summary":"One of the most common questions traders have about stocks is “Why Is It Moving?”\n\nThat’s why Benzinga created the Why Is It Moving, or WIIM, feature in Benzinga Pro. WIIMs are a one-sentence description as to why that stock is moving.","url":"https://www.benzinga.com/analyst-ratings/analyst-color/21/04/20471562/is-now-the-time-to-buy-stock-in-tesla-netflix-alibaba-ford-or-facebook","images":[{"size":"large","url":"https://cdn.benzinga.com/files/imagecache/2048x1536xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"},{"size":"small","url":"https://cdn.benzinga.com/files/imagecache/1024x768xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"},{"size":"thumb","url":"https://cdn.benzinga.com/files/imagecache/250x187xUP/images/story/2012/freestocks-11sgh7u6tmi-unsplash_3_0_0.jpg"}],"symbols":["BABA","NFLX","TSLA"]}],"next_page_token":null}`
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
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
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetNews(GetNewsRequest{
		Symbols:    []string{"AAPL", "TSLA"},
		Start:      time.Date(2021, 4, 3, 0, 0, 0, 0, time.UTC),
		End:        time.Date(2021, 4, 4, 5, 0, 0, 0, time.UTC),
		TotalLimit: 5,
		PageLimit:  2,
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.EqualValues(t, 20472678, got[0].ID)
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

func TestGetNews_ClientSideValidationErrors(t *testing.T) {
	c := DefaultClient
	c.do = func(_ *Client, _ *http.Request) (*http.Response, error) {
		assert.Fail(t, "the server should not have been called")
		return nil, nil
	}
	for _, tc := range []struct {
		name          string
		params        GetNewsRequest
		expectedError string
	}{
		{
			name:          "NegativeTotalLimit",
			params:        GetNewsRequest{TotalLimit: -1},
			expectedError: "negative total limit",
		},
		{
			name:          "NegativePageLimit",
			params:        GetNewsRequest{PageLimit: -5},
			expectedError: "negative page limit",
		},
		{
			name:          "NoTotalLimitWithNonZeroTotalLimit",
			params:        GetNewsRequest{TotalLimit: 100, NoTotalLimit: true},
			expectedError: "both NoTotalLimit and non-zero TotalLimit specified",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.GetNews(tc.params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestGetCorporateActions(t *testing.T) {
	resp := `{
		"corporate_actions": {
			"forward_splits": [
				{
					"due_bill_redemption_date": "2024-03-14",
					"ex_date": "2024-03-13",
					"new_rate": 5,
					"old_rate": 1,
					"payable_date": "2024-03-12",
					"process_date": "2024-03-13",
					"record_date": "2024-03-11",
					"symbol": "FBL"
				},
				{
					"due_bill_redemption_date": "2024-03-14",
					"ex_date": "2024-03-13",
					"new_rate": 6,
					"old_rate": 1,
					"payable_date": "2024-03-12",
					"process_date": "2024-03-13",
					"record_date": "2024-03-11",
					"symbol": "NVDL"
				}
			],
			"name_changes": [
				{
					"new_symbol": "ZEO",
					"old_symbol": "ESAC",
					"process_date": "2024-03-14"
				},
				{
					"new_symbol": "ZEOWW",
					"old_symbol": "ESACW",
					"process_date": "2024-03-14"
				},
				{
					"new_symbol": "XTIA",
					"old_symbol": "INPX",
					"process_date": "2024-03-13"
				},
				{
					"new_symbol": "IRRXW",
					"old_symbol": "IRRX.WS",
					"process_date": "2024-03-12"
				},
				{
					"new_symbol": "NMHI",
					"old_symbol": "LBBB",
					"process_date": "2024-03-12"
				},
				{
					"new_symbol": "NMHIW",
					"old_symbol": "LBBBW",
					"process_date": "2024-03-12"
				},
				{
					"new_symbol": "POLCQ",
					"old_symbol": "POLC",
					"process_date": "2024-03-11"
				},
				{
					"new_symbol": "NRDE",
					"old_symbol": "RIDEQ",
					"process_date": "2024-03-14"
				},
				{
					"new_symbol": "NTRP",
					"old_symbol": "SASI",
					"process_date": "2024-03-13"
				}
			],
			"stock_mergers": [
				{
					"acquiree_rate": 1,
					"acquiree_symbol": "FAZE",
					"acquirer_rate": 0.13091,
					"acquirer_symbol": "GAME",
					"effective_date": "2024-03-11",
					"payable_date": "2024-03-11",
					"process_date": "2024-03-11"
				},
				{
					"acquiree_rate": 1,
					"acquiree_symbol": "LBBBR",
					"acquirer_rate": 0.1,
					"acquirer_symbol": "NMHI",
					"effective_date": "2024-03-12",
					"payable_date": "2024-03-12",
					"process_date": "2024-03-12"
				}
			],
			"worthless_removals": [
				{
					"process_date": "2024-03-12",
					"symbol": "EACPW"
				},
				{
					"process_date": "2024-03-12",
					"symbol": "ZSANQ"
				}
			]
		},
		"next_page_token": null
	}`
	c := DefaultClient
	c.do = func(_ *Client, req *http.Request) (*http.Response, error) {
		assert.Equal(t, "/v1/corporate-actions", req.URL.Path)
		assert.Equal(t, "forward_split,name_change,worthless_removal,stock_merger",
			req.URL.Query().Get("types"))
		assert.Equal(t, "2024-03-10", req.URL.Query().Get("start"))
		assert.Equal(t, "2024-03-14", req.URL.Query().Get("end"))
		return &http.Response{
			Body: io.NopCloser(strings.NewReader(resp)),
		}, nil
	}
	got, err := c.GetCorporateActions(GetCorporateActionsRequest{
		Types:      []string{"forward_split", "name_change", "worthless_removal", "stock_merger"},
		Start:      civil.Date{Year: 2024, Month: 3, Day: 10},
		End:        civil.Date{Year: 2024, Month: 3, Day: 14},
		TotalLimit: 5,
		PageLimit:  2,
	})
	require.NoError(t, err)
	if assert.Len(t, got.ForwardSplits, 2) {
		assert.Equal(t, ForwardSplit{
			Symbol:                "FBL",
			NewRate:               5,
			OldRate:               1,
			ProcessDate:           civil.Date{Year: 2024, Month: 3, Day: 13},
			ExDate:                civil.Date{Year: 2024, Month: 3, Day: 13},
			RecordDate:            &civil.Date{Year: 2024, Month: 3, Day: 11},
			PayableDate:           &civil.Date{Year: 2024, Month: 3, Day: 12},
			DueBillRedemptionDate: &civil.Date{Year: 2024, Month: 3, Day: 14},
		}, got.ForwardSplits[0])
	}
	if assert.Len(t, got.NameChanges, 9) {
		assert.Equal(t, NameChange{
			NewSymbol:   "NTRP",
			OldSymbol:   "SASI",
			ProcessDate: civil.Date{Year: 2024, Month: 3, Day: 13},
		}, got.NameChanges[8])
	}
	if assert.Len(t, got.StockMergers, 2) {
		assert.Equal(t, StockMerger{
			AcquirerSymbol: "GAME",
			AcquirerRate:   0.13091,
			AcquireeSymbol: "FAZE",
			AcquireeRate:   1,
			ProcessDate:    civil.Date{Year: 2024, Month: 3, Day: 11},
			EffectiveDate:  civil.Date{Year: 2024, Month: 3, Day: 11},
			PayableDate:    &civil.Date{Year: 2024, Month: 3, Day: 11},
		}, got.StockMergers[0])
	}
	if assert.Len(t, got.WorthlessRemovals, 2) {
		assert.Equal(t, "EACPW", got.WorthlessRemovals[0].Symbol)
	}
}

func TestLatestCryptoPerpBar(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"bars": {"BTC-PERP": {"c": 101785.3,"h": 101807.9,"l": 101762.8,"n": 314,"o": 101789.1,"t": "2024-12-19T09:52:00Z","v": 25.9751,"vw": 101783.9767854599}}}`)
	got, err := c.GetLatestCryptoPerpBar("BTC-PERP", GetLatestCryptoBarRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoPerpBar{
		Timestamp:  time.Date(2024, 12, 19, 9, 52, 0, 0, time.UTC),
		Open:       101789.1,
		High:       101807.9,
		Low:        101762.8,
		Close:      101785.3,
		Volume:     25.9751,
		TradeCount: 314,
		VWAP:       101783.9767854599,
	}, *got)
}

func TestLatestCryptoPerpTrade(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"trades": {"BTC-PERP": {"i": 1805227019,"p": 101761.4,"s": 0.0011,"t": "2024-12-19T09:33:36.311Z","tks": "B"}}}`)
	got, err := c.GetLatestCryptoPerpTrade("BTC-PERP", GetLatestCryptoTradeRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoPerpTrade{
		ID:        1805227019,
		Price:     101761.4,
		Size:      0.0011,
		Timestamp: time.Date(2024, 12, 19, 9, 33, 36, 311000000, time.UTC),
		TakerSide: "B",
	}, *got)
}

func TestLatestCryptoPerpTrades(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"trades": {"ETH-PERP": {"i": 1028100310,"p": 3678.81,"s": 0.01,"t": "2024-12-19T10:16:20.124Z","tks": "B"},"BTC-PERP": {"i": 1805344202,"p": 101868,"s": 0.0009,"t": "2024-12-19T10:16:19.31Z","tks": "S"}}}`)
	got, err := c.GetLatestCryptoPerpTrades([]string{"BTC-PERP", "ETH-PERP"}, GetLatestCryptoTradeRequest{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, CryptoPerpTrade{
		Price:     101868,
		Size:      0.0009,
		Timestamp: time.Date(2024, 12, 19, 10, 16, 19, 310000000, time.UTC),
		TakerSide: "S",
		ID:        1805344202,
	}, got["BTC-PERP"])
	assert.Equal(t, 3678.81, got["ETH-PERP"].Price)
}

func TestLatestCryptoPerpQuote(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"quotes": {"BTC-PERP": {"ap": 101675.1,"as": 3.087,"bp": 101674.7,"bs": 1.4496,"t": "2024-12-19T09:43:04.092Z"}}}`)
	got, err := c.GetLatestCryptoPerpQuote("BTC-PERP", GetLatestCryptoQuoteRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoPerpQuote{
		Timestamp: time.Date(2024, 12, 19, 9, 43, 0o4, 92000000, time.UTC),
		BidPrice:  101674.7,
		BidSize:   1.4496,
		AskPrice:  101675.1,
		AskSize:   3.087,
	}, *got)
}

func TestLatestCryptoPerpQuotes(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"quotes": {"ETH-PERP": {"ap": 3676.89,"as": 38.655,"bp": 3676.82,"bs": 36.765,"t": "2024-12-19T10:15:50.436Z"},"BTC-PERP": {"ap": 101851.3,"as": 1.2372,"bp": 101850.9,"bs": 1.9428,"t": "2024-12-19T10:15:50.438Z"}}}`)
	got, err := c.GetLatestCryptoPerpQuotes([]string{"BTC-PERP", "ETH-PERP"}, GetLatestCryptoQuoteRequest{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, CryptoPerpQuote{
		Timestamp: time.Date(2024, 12, 19, 10, 15, 50, 436000000, time.UTC),
		BidPrice:  3676.82,
		BidSize:   36.765,
		AskPrice:  3676.89,
		AskSize:   38.655,
	}, got["ETH-PERP"])
	assert.Equal(t, 1.9428, got["BTC-PERP"].BidSize)
}

func TestGetLatestCryptoPerpPricing(t *testing.T) {
	c := DefaultClient
	c.do = mockResp(`{"pricing": {"BTC-PERP": {"t": "2024-12-19T09:33:36.311Z", "ft": "2024-12-19T10:33:36.311Z", "oi": 90.7367, "ip": 50702.8, "mp": 50652.3553, "fr": 0.000565699}}}`)
	got, err := c.GetLatestCryptoPerpPricing("BTC-PERP", GetLatestCryptoPerpPricingRequest{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, CryptoPerpPricing{
		IndexPrice:      50702.8,
		MarkPrice:       50652.3553,
		OpenInterest:    90.7367,
		FundingRate:     0.000565699,
		Timestamp:       time.Date(2024, 12, 19, 9, 33, 36, 311000000, time.UTC),
		NextFundingTime: time.Date(2024, 12, 19, 10, 33, 36, 311000000, time.UTC),
	}, *got)
}
