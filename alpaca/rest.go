package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is the alpaca client.
type Client interface {
	GetAccount() (*Account, error)
	GetAccountConfigurations() (*AccountConfigurations, error)
	UpdateAccountConfigurations(newConfigs AccountConfigurationsRequest) (*AccountConfigurations, error)
	GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActivity, error)
	GetPortfolioHistory(period *string, timeframe *RangeFreq, dateEnd *time.Time, extendedHours bool) (*PortfolioHistory, error)
	ListPositions() ([]Position, error)
	GetPosition(symbol string) (*Position, error)
	CloseAllPositions() error
	ClosePosition(symbol string) error
	GetClock() (*Clock, error)
	GetCalendar(start, end *string) ([]CalendarDay, error)
	ListOrders(status *string, until *time.Time, limit *int, nested *bool) ([]Order, error)
	PlaceOrder(req PlaceOrderRequest) (*Order, error)
	GetOrder(orderID string) (*Order, error)
	GetOrderByClientOrderID(clientOrderID string) (*Order, error)
	ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error)
	CancelOrder(orderID string) error
	CancelAllOrders() error
	ListAssets(status *string) ([]Asset, error)
	GetAsset(symbol string) (*Asset, error)
	StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error
	StreamTradeUpdatesInBackground(ctx context.Context, handler func(TradeUpdate))
}

// ClientOpts contains options for the alpaca client
type ClientOpts struct {
	ApiKey     string
	ApiSecret  string
	OAuth      string
	BaseURL    string
	Timeout    time.Duration
	RetryLimit int
	RetryDelay time.Duration
}

type client struct {
	opts ClientOpts

	do func(c *client, req *http.Request) (*http.Response, error)
}

// NewClient creates a new Alpaca trading client using the given opts.
func NewClient(opts ClientOpts) Client {
	if opts.ApiKey == "" {
		opts.ApiKey = os.Getenv("APCA_API_KEY_ID")
	}
	if opts.ApiSecret == "" {
		opts.ApiSecret = os.Getenv("APCA_API_SECRET_KEY")
	}
	if opts.OAuth == "" {
		opts.OAuth = os.Getenv("APCA_API_OAUTH")
	}
	if opts.BaseURL == "" {
		if s := os.Getenv("APCA_API_BASE_URL"); s != "" {
			opts.BaseURL = s
		} else {
			opts.BaseURL = "https://api.alpaca.markets"
		}
	}
	if opts.RetryLimit == 0 {
		opts.RetryLimit = 3
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = time.Second
	}
	return &client{
		opts: opts,

		do: defaultDo,
	}
}

// DefaultClient uses options from environment variables, or the defaults.
var DefaultClient = NewClient(ClientOpts{})

const (
	apiVersion = "v2"
)

func defaultDo(c *client, req *http.Request) (*http.Response, error) {
	if c.opts.OAuth != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	} else {
		req.Header.Set("APCA-API-KEY-ID", c.opts.ApiKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.opts.ApiSecret)
	}

	client := &http.Client{
		Timeout: c.opts.Timeout,
	}
	var resp *http.Response
	var err error
	for i := 0; ; i++ {
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		if i >= c.opts.RetryLimit {
			break
		}
		time.Sleep(c.opts.RetryDelay)
	}

	if err = verify(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// GetAccount returns the user's account information.
func (c *client) GetAccount() (*Account, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account", c.opts.BaseURL, apiVersion))
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
func (c *client) GetAccountConfigurations() (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", c.opts.BaseURL, apiVersion))
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
func (c *client) UpdateAccountConfigurations(newConfigs AccountConfigurationsRequest) (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", c.opts.BaseURL, apiVersion))
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

func (c *client) GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActivity, error) {
	var u *url.URL
	var err error
	if activityType == nil {
		u, err = url.Parse(fmt.Sprintf("%s/%s/account/activities", c.opts.BaseURL, apiVersion))
	} else {
		u, err = url.Parse(fmt.Sprintf("%s/%s/account/activities/%s", c.opts.BaseURL, apiVersion, *activityType))
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
			q.Set("page_size", strconv.Itoa(*opts.PageSize))
		}
	}

	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	activities := []AccountActivity{}

	if err = unmarshal(resp, &activities); err != nil {
		return nil, err
	}
	return activities, nil
}

func (c *client) GetPortfolioHistory(period *string, timeframe *RangeFreq, dateEnd *time.Time, extendedHours bool) (*PortfolioHistory, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/portfolio/history", c.opts.BaseURL, apiVersion))

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
func (c *client) ListPositions() ([]Position, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", c.opts.BaseURL, apiVersion))
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
func (c *client) GetPosition(symbol string) (*Position, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", c.opts.BaseURL, apiVersion, symbol))
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

// CloseAllPositions liquidates all open positions at market price.
func (c *client) CloseAllPositions() error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", c.opts.BaseURL, apiVersion))
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
func (c *client) ClosePosition(symbol string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", c.opts.BaseURL, apiVersion, symbol))
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
func (c *client) GetClock() (*Clock, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/clock", c.opts.BaseURL, apiVersion))
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
func (c *client) GetCalendar(start, end *string) ([]CalendarDay, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/calendar", c.opts.BaseURL, apiVersion))
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
func (c *client) ListOrders(status *string, until *time.Time, limit *int, nested *bool) ([]Order, error) {
	urlString := fmt.Sprintf("%s/%s/orders", c.opts.BaseURL, apiVersion)
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
func (c *client) PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders", c.opts.BaseURL, apiVersion))
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
func (c *client) GetOrder(orderID string) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", c.opts.BaseURL, apiVersion, orderID))
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

// GetOrderByClientOrderID submits a request to get an order by the client order ID.
func (c *client) GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders:by_client_order_id", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("client_order_id", clientOrderID)
	u.RawQuery = q.Encode()

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
func (c *client) ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", c.opts.BaseURL, apiVersion, orderID))
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
func (c *client) CancelOrder(orderID string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", c.opts.BaseURL, apiVersion, orderID))
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
func (c *client) CancelAllOrders() error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders", c.opts.BaseURL, apiVersion))
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
func (c *client) ListAssets(status *string) ([]Asset, error) {
	// TODO: support different asset classes
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets", c.opts.BaseURL, apiVersion))
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
func (c *client) GetAsset(symbol string) (*Asset, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets/%v", c.opts.BaseURL, apiVersion, symbol))
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

func GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActivity, error) {
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

// GetOrderByClientOrderID returns a single order for the given
// `clientOrderID` using the default Alpaca client.
func GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
	return DefaultClient.GetOrderByClientOrderID(clientOrderID)
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

func (c *client) get(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.do(c, req)
}

func (c *client) post(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	return c.do(c, req)
}

func (c *client) patch(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPatch, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	return c.do(c, req)
}

func (c *client) delete(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.do(c, req)
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

func verify(resp *http.Response) error {
	if resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		var apiErr APIError
		err = json.Unmarshal(body, &apiErr)
		if err != nil {
			// If the error is not in our JSON format, we simply return the HTTP response
			return fmt.Errorf("HTTP %s: %s", resp.Status, body)
		}
		return &apiErr
	}
	return nil
}

func unmarshal(resp *http.Response, data interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(data)
}
