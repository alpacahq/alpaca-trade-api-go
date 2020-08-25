package polygon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
	try "gopkg.in/matryer/try.v1"
)

const (
	aggURL       = "%v/v1/historic/agg/%v/%v"
	aggv2URL     = "%v/v2/aggs/ticker/%v/range/%v/%v/%v/%v"
	tradesURL    = "%v/v1/historic/trades/%v/%v"
	tradesv2URL  = "%v/v2/ticks/stocks/trades/%v/%v"
	quotesURL    = "%v/v1/historic/quotes/%v/%v"
	quotesv2URL  = "%v/v2/ticks/stocks/nbbo/%v/%v"
	exchangeURL  = "%v/v1/meta/exchanges"
	SnapShotAll  = "%v/v2/snapshot/locale/us/markets/stocks/tickers"
	Top20Gainers = "%v/v2/snapshot/locale/us/markets/stocks/gainers"
	Top20Losers  = "%v/v2/snapshot/locale/us/markets/stocks/losers"
)

var (
	// DefaultClient is the default Polygon client using the
	// environment variable set credentials
	DefaultClient = NewClient(common.Credentials())
	base          = "https://api.polygon.io"
	get           = func(u *url.URL) (*http.Response, error) {
		return http.Get(u.String())
	}
)

func init() {
	if s := os.Getenv("POLYGON_BASE_URL"); s != "" {
		base = s
	}
}

// APIError wraps the detailed code and message supplied
// by Polygon's API for debugging purposes
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Client is a Polygon REST API client
type Client struct {
	credentials *common.APIKey
}

// NewClient creates a new Polygon client with specified
// credentials
func NewClient(credentials *common.APIKey) *Client {
	return &Client{credentials: credentials}
}

// GetHistoricAggregates requests Polygon's v1 REST API for historic aggregates
// for the provided resolution based on the provided query parameters.
func (c *Client) GetHistoricAggregates(
	symbol string,
	resolution AggType,
	from, to *time.Time,
	limit *int) (*HistoricAggregates, error) {

	u, err := url.Parse(fmt.Sprintf(aggURL, base, resolution, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", c.credentials.PolygonKeyID)

	if from != nil {
		q.Set("from", from.Format(time.RFC3339))
	}

	if to != nil {
		q.Set("to", to.Format(time.RFC3339))
	}

	if limit != nil {
		q.Set("limit", strconv.FormatInt(int64(*limit), 10))
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	agg := &HistoricAggregates{}

	if err = unmarshal(resp, agg); err != nil {
		return nil, err
	}

	return agg, nil
}

// GetHistoricAggregates requests Polygon's v2 REST API for historic aggregates
// for the provided resolution based on the provided query parameters.
func (c *Client) GetHistoricAggregatesV2(
	symbol string,
	multiplier int,
	resolution AggType,
	from, to *time.Time,
	unadjusted *bool) (*HistoricAggregatesV2, error) {

	u, err := url.Parse(fmt.Sprintf(aggv2URL, base, symbol, multiplier, resolution, from.Unix()*1000, to.Unix()*1000))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", c.credentials.PolygonKeyID)

	if unadjusted != nil {
		q.Set("unadjusted", strconv.FormatBool(*unadjusted))
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	agg := &HistoricAggregatesV2{}

	if err = unmarshal(resp, agg); err != nil {
		return nil, err
	}

	return agg, nil
}

// GetHistoricTrades requests polygon's REST API for historic trades
// on the provided date.
//
// Deprecated: This v1 endpoint should no longer be used, as it will be removed from the Polygon API
// in the future. Please use GetHistoricTradesV2 instead.
func (c *Client) GetHistoricTrades(
	symbol string,
	date string,
	opts *GetHistoricTradesParams) (totalTrades *HistoricTrades, err error) {

	offset := int64(0)
	limit := int64(10000)
	if opts != nil {
		offset = opts.Offset
		if opts.Limit != 0 {
			limit = opts.Limit
		}
	}
	for {
		u, err := url.Parse(fmt.Sprintf(tradesURL, base, symbol, date))
		if err != nil {
			return nil, err
		}

		q := u.Query()
		q.Set("apiKey", c.credentials.PolygonKeyID)
		q.Set("limit", strconv.FormatInt(limit, 10))

		if offset > 0 {
			q.Set("offset", strconv.FormatInt(offset, 10))
		}

		u.RawQuery = q.Encode()

		var resp *http.Response

		if err = try.Do(func(attempt int) (bool, error) {
			resp, err = get(u)
			return (attempt < 3), err
		}); err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("status code %v", resp.StatusCode)
		}

		trades := &HistoricTrades{}

		if err = unmarshal(resp, trades); err != nil {
			return nil, err
		}

		if totalTrades == nil {
			totalTrades = trades
		} else {
			totalTrades.Ticks = append(totalTrades.Ticks, trades.Ticks...)
		}

		if len(trades.Ticks) == 10000 {
			offset = trades.Ticks[len(trades.Ticks)-1].Timestamp
		} else {
			break
		}
	}

	return totalTrades, nil
}

// GetHistoricTradesV2 requests polygon's REST API for historic trades
// on the provided date.
func (c *Client) GetHistoricTradesV2(ticker string, date string, opts *HistoricTicksV2Params) (*HistoricTradesV2, error) {
	u, err := url.Parse(fmt.Sprintf(tradesv2URL, base, ticker, date))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", c.credentials.PolygonKeyID)
	u.RawQuery = q.Encode()

	resp, err := c.get(u, opts)
	if err != nil {
		return nil, err
	}

	trades := &HistoricTradesV2{}

	if err = unmarshal(resp, trades); err != nil {
		return nil, err
	}

	return trades, nil
}

// GetHistoricQuotes requests polygon's REST API for historic quotes
// on the provided date.
//
// Deprecated: This v1 endpoint should no longer be used, as it will be removed from the Polygon API
// in the future. Please use GetHistoricQuotesV2 instead.
func (c *Client) GetHistoricQuotes(symbol, date string) (totalQuotes *HistoricQuotes, err error) {
	offset := int64(0)
	for {
		u, err := url.Parse(fmt.Sprintf(quotesURL, base, symbol, date))
		if err != nil {
			return nil, err
		}

		q := u.Query()
		q.Set("apiKey", c.credentials.PolygonKeyID)
		q.Set("limit", strconv.FormatInt(10000, 10))

		if offset > 0 {
			q.Set("offset", strconv.FormatInt(offset, 10))
		}

		u.RawQuery = q.Encode()

		var resp *http.Response

		if err = try.Do(func(attempt int) (bool, error) {
			resp, err = get(u)
			return (attempt < 3), err
		}); err != nil {
			return nil, err
		}

		if resp.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("status code %v", resp.StatusCode)
		}

		quotes := &HistoricQuotes{}

		if err = unmarshal(resp, quotes); err != nil {
			return nil, err
		}

		if totalQuotes == nil {
			totalQuotes = quotes
		} else {
			totalQuotes.Ticks = append(totalQuotes.Ticks, quotes.Ticks...)
		}

		if len(quotes.Ticks) == 10000 {
			offset = quotes.Ticks[len(quotes.Ticks)-1].Timestamp
		} else {
			break
		}
	}

	return totalQuotes, nil
}

// GetHistoricQuotesV2 requests polygon's REST API for historic trades
// on the provided date.
func (c *Client) GetHistoricQuotesV2(ticker string, date string, opts *HistoricTicksV2Params) (*HistoricQuotesV2, error) {
	u, err := url.Parse(fmt.Sprintf(quotesv2URL, base, ticker, date))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", c.credentials.PolygonKeyID)
	u.RawQuery = q.Encode()

	resp, err := c.get(u, opts)
	if err != nil {
		return nil, err
	}

	quotes := &HistoricQuotesV2{}

	if err = unmarshal(resp, quotes); err != nil {
		return nil, err
	}

	return quotes, nil
}

// GetStockExchanges requests available stock and equity exchanges on polygon.io
func (c *Client) GetStockExchanges() ([]StockExchange, error) {
	u, err := url.Parse(fmt.Sprintf(exchangeURL, base))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", c.credentials.PolygonKeyID)

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	var exchanges []StockExchange
	if err = unmarshal(resp, &exchanges); err != nil {
		return nil, err
	}

	return exchanges, nil

}

// GetHistoricAggregates requests polygon's REST API for historic aggregates
// for the provided resolution based on the provided query parameters using
// the default Polygon client.
func GetHistoricAggregates(
	symbol string,
	resolution AggType,
	from, to *time.Time,
	limit *int) (*HistoricAggregates, error) {
	return DefaultClient.GetHistoricAggregates(symbol, resolution, from, to, limit)
}

// GetHistoricTrades requests polygon's REST API for historic trades
// on the provided date using the default Polygon client.
func GetHistoricTrades(
	symbol string,
	date string,
	opts *GetHistoricTradesParams) (totalTrades *HistoricTrades, err error) {
	return DefaultClient.GetHistoricTrades(symbol, date, opts)
}

// GetHistoricQuotes requests polygon's REST API for historic quotes
// on the provided date using the default Polygon client.
func GetHistoricQuotes(symbol, date string) (totalQuotes *HistoricQuotes, err error) {
	return DefaultClient.GetHistoricQuotes(symbol, date)
}

// GetStockExchanges queries Polygon.io REST API for information on available
// stock and equities exchanges
func GetStockExchanges() ([]StockExchange, error) {
	return DefaultClient.GetStockExchanges()
}

// GetAllTickers queries Polygon.io REST API for a snapshot of all tickers
// current minute aggregate, daily aggregate and last trade.
// As well as previous days aggregate and calculated change for today.
func GetAllTickers() (map[string]Ticker, error) {
	u, err := url.Parse(fmt.Sprintf(SnapShotAll, base))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("apiKey", DefaultClient.credentials.PolygonKeyID)
	u.RawQuery = q.Encode()
	resp, err := get(u)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New(fmt.Sprintf("status code %v", resp.StatusCode))
	}
	tickers := &Tickers{}
	if err = unmarshal(resp, tickers); err != nil {
		return nil, err
	}
	returnValue := map[string]Ticker{}
	for _, ticker := range tickers.Tickers {
		returnValue[ticker.Ticker] = ticker
	}
	return returnValue, nil
}

// GetTop20Gainers queries Polygon.io REST API for the current snapshot of
// the top 20 gainers of the day at the moment.
func GetTop20Gainers() (map[string]Ticker, error) {
	u, err := url.Parse(fmt.Sprintf(Top20Gainers, base))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("apiKey", DefaultClient.credentials.PolygonKeyID)
	u.RawQuery = q.Encode()
	resp, err := get(u)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New(fmt.Sprintf("status code %v", resp.StatusCode))
	}
	tickers := &Tickers{}
	if err = unmarshal(resp, tickers); err != nil {
		return nil, err
	}
	returnValue := map[string]Ticker{}
	for _, ticker := range tickers.Tickers {
		returnValue[ticker.Ticker] = ticker
	}
	return returnValue, nil
}

// GetTop20Losers queries Polygon.io REST API for the current snapshot of
// the top 20 losers of the day at the moment.
func GetTop20Losers() (map[string]Ticker, error) {
	u, err := url.Parse(fmt.Sprintf(Top20Losers, base))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("apiKey", DefaultClient.credentials.PolygonKeyID)
	u.RawQuery = q.Encode()
	resp, err := get(u)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New(fmt.Sprintf("status code %v", resp.StatusCode))
	}
	tickers := &Tickers{}
	if err = unmarshal(resp, tickers); err != nil {
		return nil, err
	}
	returnValue := map[string]Ticker{}
	for _, ticker := range tickers.Tickers {
		returnValue[ticker.Ticker] = ticker
	}
	return returnValue, nil
}

func unmarshal(resp *http.Response, data interface{}) error {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, data)
}

func verify(resp *http.Response) (err error) {
	if resp.StatusCode >= http.StatusMultipleChoices {
		var body []byte
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		fmt.Println(string(body))

		apiErr := APIError{}

		err = json.Unmarshal(body, &apiErr)
		if err == nil {
			err = &apiErr
		}
	}

	return
}

// Gets data with request body marshalling
func (c *Client) get(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if err = verify(resp); err != nil {
		return nil, err
	}

	return resp, nil
}
