package alpaca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/common"
)

var (
	base string
	get  = func(u *url.URL) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}

		return do(req)
	}
	post = func(u *url.URL, data interface{}) (*http.Response, error) {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buf))
		if err != nil {
			return nil, err
		}

		return do(req)
	}
	delete = func(u *url.URL) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
		if err != nil {
			return nil, err
		}

		return do(req)
	}
)

func init() {
	if s := os.Getenv("ALPACA_BASE_URL"); s != "" {
		base = s
	} else {
		base = "https://api.alpaca.markets"
	}
}

type ApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *ApiError) Error() string {
	return e.Message
}

// GetAccount returns the user's account information
func GetAccount() (*Account, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/account", base))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	account := &Account{}

	if err = unmarshal(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// ListPositions lists the account's open positions
func ListPositions() ([]Position, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/positions", base))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	positions := []Position{}

	if err = unmarshal(resp, &positions); err != nil {
		return nil, err
	}

	return positions, nil
}

// GetClock returns the current market clock
func GetClock() (*Clock, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/clock", base))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	clock := &Clock{}

	if err = unmarshal(resp, &clock); err != nil {
		return nil, err
	}

	return clock, nil
}

// GetCalendar returns the market calendar, sliced by the start
// and end dates
func GetCalendar(start, end *string) ([]CalendarDay, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/calendar", base))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	if start != nil {
		q.Set("start", *start)
	}

	if end != nil {
		q.Set("end", *end)
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	calendar := []CalendarDay{}

	if err = unmarshal(resp, &calendar); err != nil {
		return nil, err
	}

	return calendar, nil
}

// ListOrders returns the list of orders for an account,
// filtered by the input parameters.
func ListOrders(status *string, until *time.Time, limit *int) ([]Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/orders", base))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	if status != nil {
		q.Set("status", *status)
	}

	if until != nil {
		q.Set("until", until.Format(time.RFC3339))
	}

	if limit != nil {
		q.Set("limit", strconv.FormatInt(int64(*limit), 10))
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	orders := []Order{}

	if err = unmarshal(resp, &orders); err != nil {
		return nil, err
	}

	return orders, nil
}

// PlaceOrder submits an order request to buy or sell an asset.
func PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/orders", base))
	if err != nil {
		return nil, err
	}

	resp, err := post(u, req)
	if err != nil {
		return nil, err
	}

	order := &Order{}

	if err = unmarshal(resp, order); err != nil {
		return nil, err
	}

	return order, nil
}

// CancelOrder submits a request to cancel an open order.
func CancelOrder(orderID string) error {
	u, err := url.Parse(fmt.Sprintf("%s/v1/orders/%s", base, orderID))
	if err != nil {
		return err
	}

	resp, err := delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// ListAssets returns the list of assets, filtered by
// the input parameters.
func ListAssets(status *string) ([]Asset, error) {
	// TODO: support different asset classes
	u, err := url.Parse(fmt.Sprintf("%v/v1/assets", base))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	if status != nil {
		q.Set("status", *status)
	}

	u.RawQuery = q.Encode()

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	assets := []Asset{}

	if err = unmarshal(resp, &assets); err != nil {
		return nil, err
	}

	return assets, nil
}

// GetAsset returns an asset for the given symbol.
func GetAsset(symbol string) (*Asset, error) {
	u, err := url.Parse(fmt.Sprintf("%v/v1/assets/%v", base, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	asset := &Asset{}

	if err = unmarshal(resp, asset); err != nil {
		return nil, err
	}

	return asset, nil
}

// ListBarLists returns a list of bar lists corresponding to the provided
// symbol list, and filtered by the provided parameters.
func ListBarLists(symbols []string, opts BarListParams) ([]BarList, error) {
	vals := url.Values{}
	vals.Add("symbols", strings.Join(symbols, ","))

	u, err := url.Parse(fmt.Sprintf("%v/v1/bars?%v", base, vals.Encode()))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	bars := []BarList{}

	if err = unmarshal(resp, &bars); err != nil {
		return nil, err
	}

	return bars, nil
}

// GetBarList returns a list of bars corresponding to the provided
// symbol, and filtered by the provided parameters.
func GetBarList(symbol string, opts BarListParams) (*BarList, error) {
	vals := url.Values{}
	vals.Set("timeframe", opts.Timeframe)
	if opts.StartDt != nil {
		vals.Set("start_dt", opts.StartDt.Format(time.RFC3339))
	}

	if opts.EndDt != nil {
		vals.Set("end_dt", opts.EndDt.Format(time.RFC3339))
	}

	if opts.Limit != nil {
		vals.Set("limit", strconv.FormatInt(int64(*opts.Limit), 10))
	}

	u, err := url.Parse(fmt.Sprintf("%v/v1/assets/%s/bars?%v", base, symbol, vals.Encode()))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	bars := &BarList{}

	if err = unmarshal(resp, bars); err != nil {
		return nil, err
	}

	return bars, nil
}

// ListQuotes returns a list of quotes corresponding to the
// provided list of symbols.
func ListQuotes(symbols []string) ([]Quote, error) {
	vals := url.Values{}
	vals.Add("symbols", strings.Join(symbols, ","))

	u, err := url.Parse(fmt.Sprintf("%v/v1/quotes?%v", base, vals.Encode()))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	quotes := []Quote{}

	if err = unmarshal(resp, &quotes); err != nil {
		return nil, err
	}

	return quotes, nil
}

// GetQuote returns a quote corresponding to the provided symbol.
func GetQuote(symbol string) (*Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%v/v1/assets/%s/quote", base, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := get(u)
	if err != nil {
		return nil, err
	}

	quote := &Quote{}

	if err = unmarshal(resp, quote); err != nil {
		return nil, err
	}

	return quote, nil
}

func do(req *http.Request) (*http.Response, error) {
	req.Header.Set("APCA-API-KEY-ID", common.Credentials().ID)
	req.Header.Set("APCA-API-SECRET-KEY", common.Credentials().Secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if err = verify(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func verify(resp *http.Response) (err error) {
	if resp.StatusCode >= http.StatusMultipleChoices {
		var body []byte
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		apiErr := ApiError{}

		err = json.Unmarshal(body, &apiErr)
		if err == nil {
			err = &apiErr
		}
	}

	return
}

func unmarshal(resp *http.Response, data interface{}) error {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, data)
}
