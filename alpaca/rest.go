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
	// DefaultClient is the default Alpaca client using the
	// environment variable set credentials
	DefaultClient = NewClient(common.Credentials())
	base          = "https://api.alpaca.markets"
	dataUrl       = "https://data.alpaca.markets"
	apiVersion    = "v2"
	do            = func(c *Client, req *http.Request) (*http.Response, error) {
		if c.credentials.OAuth != "" {
			req.Header.Set("Authorization", "Bearer "+c.credentials.OAuth)
		} else {
			req.Header.Set("APCA-API-KEY-ID", c.credentials.ID)
			req.Header.Set("APCA-API-SECRET-KEY", c.credentials.Secret)
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
)

func init() {
	if s := os.Getenv("APCA_API_BASE_URL"); s != "" {
		base = s
	} else if s := os.Getenv("ALPACA_BASE_URL"); s != "" {
		// legacy compatibility...
		base = s
	}
	if s := os.Getenv("APCA_DATA_URL"); s != "" {
		dataUrl = s
	}
	if s := os.Getenv("APCA_API_VERSION"); s != "" {
		apiVersion = s
	}
}

// APIError wraps the detailed code and message supplied
// by Alpaca's API for debugging purposes
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Client is an Alpaca REST API client
type Client struct {
	credentials *common.APIKey
}

func SetBaseUrl(baseUrl string) {
	base = baseUrl
}

// NewClient creates a new Alpaca client with specified
// credentials
func NewClient(credentials *common.APIKey) *Client {
	return &Client{credentials: credentials}
}

// GetAccount returns the user's account information.
func (c *Client) GetAccount() (*Account, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	account := &Account{}

	if err = unmarshal(resp, account); err != nil {
		return nil, err
	}

	return account, nil
}

// GetConfigs returns the current account configurations
func (c *Client) GetAccountConfigurations() (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	configs := &AccountConfigurations{}

	if err = unmarshal(resp, configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// EditConfigs patches the account configs
func (c *Client) UpdateAccountConfigurations(newConfigs AccountConfigurationsRequest) (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.patch(u, newConfigs)
	if err != nil {
		return nil, err
	}

	configs := &AccountConfigurations{}

	if err = unmarshal(resp, configs); err != nil {
		return nil, err
	}

	return configs, nil
}

func (c *Client) GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActvity, error) {
	var u *url.URL
	var err error
	if activityType == nil {
		u, err = url.Parse(fmt.Sprintf("%s/%s/account/activities", base, apiVersion))
	} else {
		u, err = url.Parse(fmt.Sprintf("%s/%s/account/activities/%s", base, apiVersion, *activityType))
	}
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if opts != nil {
		if opts.ActivityTypes != nil {
			q.Set("activity_types", strings.Join(*opts.ActivityTypes, ","))
		}
		if opts.Date != nil {
			q.Set("date", opts.Date.String())
		}
		if opts.Until != nil {
			q.Set("until", opts.Until.String())
		}
		if opts.After != nil {
			q.Set("after", opts.After.String())
		}
		if opts.Direction != nil {
			q.Set("direction", *opts.Direction)
		}
		if opts.PageSize != nil {
			q.Set("page_size", string(*opts.PageSize))
		}
	}

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	activities := []AccountActvity{}

	if err = unmarshal(resp, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

func (c *Client) GetPortfolioHistory(period *string, timeframe *RangeFreq, dateEnd *time.Time, extendedHours bool) (*PortfolioHistory, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/portfolio/history", base, apiVersion))

	if err != nil {
		return nil, err
	}

	query := u.Query()

	if period != nil {
		query.Set("period", *period)
	}

	if timeframe != nil {
		query.Set("timeframe", string(*timeframe))
	}

	if dateEnd != nil {
		query.Set("date_end", dateEnd.Format("2006-01-02"))
	}

	query.Set("extended_hours", strconv.FormatBool(extendedHours))

	// update the rawquery with the encoded params
	u.RawQuery = query.Encode()

	resp, err := c.get(u)

	if err != nil {
		return nil, err
	}

	var history PortfolioHistory

	if err = unmarshal(resp, &history); err != nil {
		return nil, err
	}

	return &history, nil
}

// ListPositions lists the account's open positions.
func (c *Client) ListPositions() ([]Position, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	positions := []Position{}

	if err = unmarshal(resp, &positions); err != nil {
		return nil, err
	}

	return positions, nil
}

// GetPosition returns the account's position for the provided symbol.
func (c *Client) GetPosition(symbol string) (*Position, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", base, apiVersion, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	q.Set("symbol", symbol)

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	position := &Position{}

	if err = unmarshal(resp, &position); err != nil {
		return nil, err
	}

	return position, nil
}

// GetAggregates returns the bars for the given symbol, timespan and date-range
func (c *Client) GetAggregates(symbol, timespan, from, to string) (*Aggregates, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/aggs/ticker/%s/range/1/%s/%s/%s",
		dataUrl, symbol, timespan, from, to))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	q.Set("symbol", symbol)
	q.Set("timespan", timespan)
	q.Set("from", from)
	q.Set("to", to)

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	aggregate := &Aggregates{}

	if err = unmarshal(resp, &aggregate); err != nil {
		return nil, err
	}

	return aggregate, nil
}

// GetLastQuote returns the last quote for the given symbol
func (c *Client) GetLastQuote(symbol string) (*LastQuoteResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/last_quote/stocks/%s", dataUrl, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	q.Set("symbol", symbol)

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	lastQuote := &LastQuoteResponse{}

	if err = unmarshal(resp, &lastQuote); err != nil {
		return nil, err
	}

	return lastQuote, nil
}

// GetLastTrade returns the last trade for the given symbol
func (c *Client) GetLastTrade(symbol string) (*LastTradeResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/last/stocks/%s", dataUrl, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	q.Set("symbol", symbol)

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	lastTrade := &LastTradeResponse{}

	if err = unmarshal(resp, &lastTrade); err != nil {
		return nil, err
	}

	return lastTrade, nil
}

// CloseAllPositions liquidates all open positions at market price.
func (c *Client) CloseAllPositions() error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", base, apiVersion))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// ClosePosition liquidates the position for the given symbol at market price.
func (c *Client) ClosePosition(symbol string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", base, apiVersion, symbol))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// GetClock returns the current market clock.
func (c *Client) GetClock() (*Clock, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/clock", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
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
// and end dates.
func (c *Client) GetCalendar(start, end *string) ([]CalendarDay, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/calendar", base, apiVersion))
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

	resp, err := c.get(u)
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
func (c *Client) ListOrders(status *string, until *time.Time, limit *int, nested *bool) ([]Order, error) {
	urlString := fmt.Sprintf("%s/%s/orders", base, apiVersion)
	if nested != nil {
		urlString += fmt.Sprintf("?nested=%v", *nested)
	}
	u, err := url.Parse(urlString)
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

	resp, err := c.get(u)
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
func (c *Client) PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders", base, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.post(u, req)
	if err != nil {
		return nil, err
	}

	order := &Order{}

	if err = unmarshal(resp, order); err != nil {
		return nil, err
	}

	return order, nil
}

// GetOrder submits a request to get an order by the order ID.
func (c *Client) GetOrder(orderID string) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", base, apiVersion, orderID))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	order := &Order{}

	if err = unmarshal(resp, order); err != nil {
		return nil, err
	}

	return order, nil
}

// ReplaceOrder submits a request to replace an order by id
func (c *Client) ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", base, apiVersion, orderID))
	if err != nil {
		return nil, err
	}

	resp, err := c.patch(u, req)
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
func (c *Client) CancelOrder(orderID string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", base, apiVersion, orderID))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// CancelAllOrders submits a request to cancel an open order.
func (c *Client) CancelAllOrders() error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders", base, apiVersion))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// ListAssets returns the list of assets, filtered by
// the input parameters.
func (c *Client) ListAssets(status *string) ([]Asset, error) {
	// TODO: support different asset classes
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets", base, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()

	if status != nil {
		q.Set("status", *status)
	}

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
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
func (c *Client) GetAsset(symbol string) (*Asset, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets/%v", base, apiVersion, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	asset := &Asset{}

	if err = unmarshal(resp, asset); err != nil {
		return nil, err
	}

	return asset, nil
}

// ListBars returns a list of bar lists corresponding to the provided
// symbol list, and filtered by the provided parameters.
func (c *Client) ListBars(symbols []string, opts ListBarParams) (map[string][]Bar, error) {
	vals := url.Values{}
	vals.Add("symbols", strings.Join(symbols, ","))

	if opts.Timeframe == "" {
		return nil, fmt.Errorf("timeframe is required for the bars endpoint")
	}

	if opts.StartDt != nil {
		vals.Set("start", opts.StartDt.Format(time.RFC3339))
	}

	if opts.EndDt != nil {
		vals.Set("end", opts.EndDt.Format(time.RFC3339))
	}

	if opts.Limit != nil {
		vals.Set("limit", strconv.FormatInt(int64(*opts.Limit), 10))
	}

	u, err := url.Parse(fmt.Sprintf("%s/v1/bars/%s?%v", dataUrl, opts.Timeframe, vals.Encode()))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}
	var bars map[string][]Bar

	if err = unmarshal(resp, &bars); err != nil {
		return nil, err
	}

	return bars, nil
}

// GetSymbolBars is a convenience method for getting the market
// data for one symbol
func (c *Client) GetSymbolBars(symbol string, opts ListBarParams) ([]Bar, error) {
	symbolList := []string{symbol}

	barsMap, err := c.ListBars(symbolList, opts)
	if err != nil {
		return nil, err
	}

	return barsMap[symbol], nil
}

// GetAccount returns the user's account information
// using the default Alpaca client.
func GetAccount() (*Account, error) {
	return DefaultClient.GetAccount()
}

// GetAccountConfigurations returns the account configs
// using the default Alpaca client.
func GetAccountConfigurations() (*AccountConfigurations, error) {
	return DefaultClient.GetAccountConfigurations()
}

// UpdateAccountConfigurations changes the account configs and returns the
// new configs using the default Alpaca client
func UpdateAccountConfigurations(newConfigs AccountConfigurationsRequest) (*AccountConfigurations, error) {
	return DefaultClient.UpdateAccountConfigurations(newConfigs)
}

func GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActvity, error) {
	return DefaultClient.GetAccountActivities(activityType, opts)
}

func GetPortfolioHistory(period *string, timeframe *RangeFreq, dateEnd *time.Time, extendedHours bool) (*PortfolioHistory, error) {
	return DefaultClient.GetPortfolioHistory(period, timeframe, dateEnd, extendedHours)
}

// ListPositions lists the account's open positions
// using the default Alpaca client.
func ListPositions() ([]Position, error) {
	return DefaultClient.ListPositions()
}

// GetAggregates returns the bars for the given symbol, timespan and date-range
func GetAggregates(symbol, timespan, from, to string) (*Aggregates, error) {
	return DefaultClient.GetAggregates(symbol, timespan, from, to)
}

// GetLastQuote returns the last quote for the given symbol
func GetLastQuote(symbol string) (*LastQuoteResponse, error) {
	return DefaultClient.GetLastQuote(symbol)
}

// GetLastTrade returns the last trade for the given symbol
func GetLastTrade(symbol string) (*LastTradeResponse, error) {
	return DefaultClient.GetLastTrade(symbol)
}

// GetPosition returns the account's position for the
// provided symbol using the default Alpaca client.
func GetPosition(symbol string) (*Position, error) {
	return DefaultClient.GetPosition(symbol)
}

// GetClock returns the current market clock
// using the default Alpaca client.
func GetClock() (*Clock, error) {
	return DefaultClient.GetClock()
}

// GetCalendar returns the market calendar, sliced by the start
// and end dates using the default Alpaca client.
func GetCalendar(start, end *string) ([]CalendarDay, error) {
	return DefaultClient.GetCalendar(start, end)
}

// ListOrders returns the list of orders for an account,
// filtered by the input parameters using the default
// Alpaca client.
func ListOrders(status *string, until *time.Time, limit *int, nested *bool) ([]Order, error) {
	return DefaultClient.ListOrders(status, until, limit, nested)
}

// PlaceOrder submits an order request to buy or sell an asset
// with the default Alpaca client.
func PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	return DefaultClient.PlaceOrder(req)
}

// GetOrder returns a single order for the given
// `orderID` using the default Alpaca client.
func GetOrder(orderID string) (*Order, error) {
	return DefaultClient.GetOrder(orderID)
}

// ReplaceOrder changes an order by order id
// using the default Alpaca client.
func ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error) {
	return DefaultClient.ReplaceOrder(orderID, req)
}

// CancelOrder submits a request to cancel an open order with
// the default Alpaca client.
func CancelOrder(orderID string) error {
	return DefaultClient.CancelOrder(orderID)
}

// ListAssets returns the list of assets, filtered by
// the input parameters with the default Alpaca client.
func ListAssets(status *string) ([]Asset, error) {
	return DefaultClient.ListAssets(status)
}

// GetAsset returns an asset for the given symbol with
// the default Alpaca client.
func GetAsset(symbol string) (*Asset, error) {
	return DefaultClient.GetAsset(symbol)
}

// ListBars returns a map of bar lists corresponding to the provided
// symbol list that is filtered by the provided parameters with the default
// Alpaca client.
func ListBars(symbols []string, opts ListBarParams) (map[string][]Bar, error) {
	return DefaultClient.ListBars(symbols, opts)
}

// GetSymbolBars returns a list of bars corresponding to the provided
// symbol that is filtered by the provided parameters with the default
// Alpaca client.
func GetSymbolBars(symbol string, opts ListBarParams) ([]Bar, error) {
	return DefaultClient.GetSymbolBars(symbol, opts)
}

func (c *Client) get(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return do(c, req)
}

func (c *Client) post(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	return do(c, req)
}

func (c *Client) patch(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPatch, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	return do(c, req)
}

func (c *Client) delete(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return do(c, req)
}

func (bar *Bar) GetTime() time.Time {
	return time.Unix(bar.Time, 0)
}

func verify(resp *http.Response) (err error) {
	if resp.StatusCode >= http.StatusMultipleChoices {
		var body []byte
		defer resp.Body.Close()

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		apiErr := APIError{}

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
