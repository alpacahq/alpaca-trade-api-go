package polygon

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
)

const (
	aggURL    = "%v/historic/agg/%v/%v"
	tradesURL = "%v/historic/trades/%v/%v"
	quotesURL = "%v/historic/quotes/%v/%v"
)

var (
	base string
	get  = func(u *url.URL) (*http.Response, error) {
		return http.Get(u.String())
	}
)

func init() {
	if s := os.Getenv("POLYGON_BASE_URL"); s != "" {
		base = s
	} else {
		base = "https://api.polygon.io"
	}
}

// GetHistoricAggregates requests polygon's REST API for historic aggregates
// for the provided resolution based on the provided query parameters.
func GetHistoricAggregates(
	symbol string,
	resolution AggType,
	from, to *time.Time,
	limit *int) (*HistoricAggregates, error) {

	u, err := url.Parse(fmt.Sprintf(aggURL, base, resolution, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", common.Credentials().ID)

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

// GetHistoricTrades requests polygon's REST API for historic trades
// on the provided date.
func GetHistoricTrades(symbol, date string, limit, offset *int) (*HistoricTrades, error) {
	u, err := url.Parse(fmt.Sprintf(tradesURL, base, symbol, date))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", common.Credentials().ID)

	if limit != nil {
		q.Set("limit", strconv.FormatInt(int64(*limit), 10))
	}

	if offset != nil {
		q.Set("offset", strconv.FormatInt(int64(*offset), 10))
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	trades := &HistoricTrades{}

	if err = unmarshal(resp, trades); err != nil {
		return nil, err
	}

	return trades, nil
}

// GetHistoricQuotes requests polygon's REST API for historic quotes
// on the provided date.
func GetHistoricQuotes(symbol, date string, limit, offset *int) (*HistoricQuotes, error) {
	u, err := url.Parse(fmt.Sprintf(quotesURL, base, symbol, date))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("apiKey", common.Credentials().ID)

	if limit != nil {
		q.Set("limit", strconv.FormatInt(int64(*limit), 10))
	}

	if offset != nil {
		q.Set("offset", strconv.FormatInt(int64(*offset), 10))
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("status code %v", resp.StatusCode)
	}

	quotes := &HistoricQuotes{}

	if err = unmarshal(resp, quotes); err != nil {
		return nil, err
	}

	return quotes, nil
}

func unmarshal(resp *http.Response, data interface{}) error {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, data)
}
