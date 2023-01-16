package alpaca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// ClientOpts contains options for the alpaca client
type ClientOpts struct {
	APIKey     string
	APISecret  string
	OAuth      string
	BaseURL    string
	RetryLimit int
	RetryDelay time.Duration
	// HTTPClient to be used for each http request.
	HTTPClient *http.Client
}

// Client is the alpaca trading client
type Client struct {
	opts       ClientOpts
	httpClient *http.Client

	do func(c *Client, req *http.Request) (*http.Response, error)
}

// NewClient creates a new Alpaca trading client using the given opts.
func NewClient(opts ClientOpts) *Client {
	if opts.APIKey == "" {
		opts.APIKey = os.Getenv("APCA_API_KEY_ID")
	}
	if opts.APISecret == "" {
		opts.APISecret = os.Getenv("APCA_API_SECRET_KEY")
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
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &Client{
		opts:       opts,
		httpClient: httpClient,

		do: defaultDo,
	}
}

// DefaultClient uses options from environment variables, or the defaults.
var DefaultClient = NewClient(ClientOpts{})

const (
	apiVersion = "v2"
)

func defaultDo(c *Client, req *http.Request) (*http.Response, error) {
	if c.opts.OAuth != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	} else {
		req.Header.Set("APCA-API-KEY-ID", c.opts.APIKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.opts.APISecret)
	}

	var resp *http.Response
	var err error
	for i := 0; ; i++ {
		resp, err = c.httpClient.Do(req)
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
func (c *Client) GetAccount() (*Account, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var account Account
	if err = unmarshal(resp, &account); err != nil {
		return nil, err
	}
	return &account, nil
}

// GetConfigs returns the current account configurations
func (c *Client) GetAccountConfigurations() (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var configs AccountConfigurations
	if err = unmarshal(resp, configs); err != nil {
		return nil, err
	}
	return &configs, nil
}

type UpdateAccountConfigurationsRequest struct {
	DtbpCheck         string `json:"dtbp_check"`
	TradeConfirmEmail string `json:"trade_confirm_email"`
	SuspendTrade      bool   `json:"suspend_trade"`
	NoShorting        bool   `json:"no_shorting"`
	FractionalTrading bool   `json:"fractional_trading"`
}

// EditConfigs patches the account configs
func (c *Client) UpdateAccountConfigurations(req UpdateAccountConfigurationsRequest) (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.patch(u, req)
	if err != nil {
		return nil, err
	}

	var configs AccountConfigurations
	if err = unmarshal(resp, &configs); err != nil {
		return nil, err
	}
	return &configs, nil
}

type GetAccountActivitiesRequest struct {
	ActivityTypes []string  `json:"activity_types"`
	Date          time.Time `json:"date"`
	Until         time.Time `json:"until"`
	After         time.Time `json:"after"`
	Direction     string    `json:"direction"`
	PageSize      int       `json:"page_size"`
}

func (c *Client) GetAccountActivities(req GetAccountActivitiesRequest) ([]AccountActivity, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/activities", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if len(req.ActivityTypes) > 0 {
		q.Set("activity_types", strings.Join(req.ActivityTypes, ","))
	}
	if !req.Date.IsZero() {
		q.Set("date", req.Date.UTC().Format(time.RFC3339Nano))
	}
	if !req.Until.IsZero() {
		q.Set("until", req.Until.UTC().Format(time.RFC3339Nano))
	}
	if !req.After.IsZero() {
		q.Set("after", req.After.UTC().Format(time.RFC3339Nano))
	}
	if req.Direction != "" {
		q.Set("direction", req.Direction)
	}
	if req.PageSize != 0 {
		q.Set("page_size", strconv.Itoa(req.PageSize))
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

type GetPortfolioHistoryRequest struct {
	Period        string
	TimeFrame     TimeFrame
	DateEnd       time.Time
	ExtendedHours bool
}

func (c *Client) GetPortfolioHistory(req GetPortfolioHistoryRequest) (*PortfolioHistory, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/portfolio/history", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	query := u.Query()
	if req.Period != "" {
		query.Set("period", req.Period)
	}
	if req.TimeFrame != "" {
		query.Set("timeframe", string(req.TimeFrame))
	}
	if !req.DateEnd.IsZero() {
		query.Set("date_end", req.DateEnd.Format("2006-01-02"))
	}
	query.Set("extended_hours", strconv.FormatBool(req.ExtendedHours))
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

// GetPositions returns the account's open positions.
func (c *Client) GetPositions() ([]Position, error) {
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
func (c *Client) GetPosition(symbol string) (*Position, error) {
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

	var position Position
	if err = unmarshal(resp, &position); err != nil {
		return nil, err
	}
	return &position, nil
}

type CloseAllPositionsRequest struct {
	CancelOrders bool
}

// CloseAllPositions liquidates all open positions at market price.
func (c *Client) CloseAllPositions(req CloseAllPositionsRequest) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", c.opts.BaseURL, apiVersion))
	if err != nil {
		return err
	}

	q := u.Query()
	q.Set("cancel_orders", strconv.FormatBool(req.CancelOrders))
	u.RawQuery = q.Encode()

	resp, err := c.delete(u)
	if err != nil {
		return err
	}
	return verify(resp)
}

type ClosePositionRequest struct {
	// Qty is the number of shares to liquidate. Can accept up to 9 decimal points.
	// Cannot work with percentage.
	Qty decimal.Decimal
	// Percentage of position to liquidate. Must be between 0 and 100.
	// Would only sell fractional if position is originally fractional.
	// Can accept up to 9 decimal points. Cannot work with qty.
	Percentage decimal.Decimal
}

// ClosePosition liquidates the position for the given symbol at market price.
func (c *Client) ClosePosition(symbol string, req ClosePositionRequest) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", c.opts.BaseURL, apiVersion, symbol))
	if err != nil {
		return err
	}

	q := u.Query()
	if !req.Qty.IsZero() {
		q.Set("qty", req.Qty.String())
	}
	if !req.Percentage.IsZero() {
		q.Set("percentage", req.Percentage.String())
	}
	u.RawQuery = q.Encode()

	resp, err := c.delete(u)
	if err != nil {
		return err
	}

	return verify(resp)
}

// GetClock returns the current market clock.
func (c *Client) GetClock() (*Clock, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/clock", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var clock Clock
	if err = unmarshal(resp, &clock); err != nil {
		return nil, err
	}
	return &clock, nil
}

type GetCalendarRequest struct {
	Start time.Time
	End   time.Time
}

// GetCalendar returns the market calendar.
func (c *Client) GetCalendar(req GetCalendarRequest) ([]CalendarDay, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/calendar", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if !req.Start.IsZero() {
		q.Set("start", req.Start.Format("2006-01-02"))
	}
	if !req.End.IsZero() {
		q.Set("end", req.End.Format("2006-01-02"))
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

type GetOrdersRequest struct {
	Status    string    `json:"status"`
	Limit     int       `json:"limit"`
	After     time.Time `json:"after"`
	Until     time.Time `json:"until"`
	Direction string    `json:"direction"`
	Nested    bool      `json:"nested"`
	Side      string    `json:"side"`
	Symbols   []string  `json:"symbols"`
}

// GetOrders returns the list of orders for an account.
func (c *Client) GetOrders(req GetOrdersRequest) ([]Order, error) {
	urlString := fmt.Sprintf("%s/%s/orders", c.opts.BaseURL, apiVersion)

	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if req.Status != "" {
		q.Set("status", req.Status)
	}
	if req.Limit != 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	if !req.After.IsZero() {
		q.Set("after", req.After.Format(time.RFC3339))
	}
	if !req.Until.IsZero() {
		q.Set("until", req.Until.Format(time.RFC3339))
	}
	if req.Direction != "" {
		q.Set("direction", req.Direction)
	}
	if req.Side != "" {
		q.Set("side", req.Side)
	}
	q.Set("nested", strconv.FormatBool(req.Nested))
	if len(req.Symbols) > 0 {
		q.Set("symbols", strings.Join(req.Symbols, ","))
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

type PlaceOrderRequest struct {
	Symbol        string           `json:"symbol"`
	Qty           *decimal.Decimal `json:"qty"`
	Notional      *decimal.Decimal `json:"notional"`
	Side          Side             `json:"side"`
	Type          OrderType        `json:"type"`
	TimeInForce   TimeInForce      `json:"time_in_force"`
	LimitPrice    *decimal.Decimal `json:"limit_price"`
	ExtendedHours bool             `json:"extended_hours"`
	StopPrice     *decimal.Decimal `json:"stop_price"`
	ClientOrderID string           `json:"client_order_id"`
	OrderClass    OrderClass       `json:"order_class"`
	TakeProfit    *TakeProfit      `json:"take_profit"`
	StopLoss      *StopLoss        `json:"stop_loss"`
	TrailPrice    *decimal.Decimal `json:"trail_price"`
	TrailPercent  *decimal.Decimal `json:"trail_percent"`
}

type TakeProfit struct {
	LimitPrice *decimal.Decimal `json:"limit_price"`
}

type StopLoss struct {
	LimitPrice *decimal.Decimal `json:"limit_price"`
	StopPrice  *decimal.Decimal `json:"stop_price"`
}

// PlaceOrder submits an order request to buy or sell an asset.
func (c *Client) PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.post(u, req)
	if err != nil {
		return nil, err
	}

	var order Order
	if err = unmarshal(resp, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// GetOrder submits a request to get an order by the order ID.
func (c *Client) GetOrder(orderID string) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", c.opts.BaseURL, apiVersion, orderID))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var order Order
	if err = unmarshal(resp, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// GetOrderByClientOrderID submits a request to get an order by the client order ID.
func (c *Client) GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
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

	var order Order
	if err = unmarshal(resp, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

type ReplaceOrderRequest struct {
	Qty           *decimal.Decimal `json:"qty"`
	LimitPrice    *decimal.Decimal `json:"limit_price"`
	StopPrice     *decimal.Decimal `json:"stop_price"`
	Trail         *decimal.Decimal `json:"trail"`
	TimeInForce   TimeInForce      `json:"time_in_force"`
	ClientOrderID string           `json:"client_order_id"`
}

// ReplaceOrder submits a request to replace an order by id
func (c *Client) ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders/%s", c.opts.BaseURL, apiVersion, orderID))
	if err != nil {
		return nil, err
	}

	resp, err := c.patch(u, req)
	if err != nil {
		return nil, err
	}

	var order Order
	if err = unmarshal(resp, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// CancelOrder submits a request to cancel an open order.
func (c *Client) CancelOrder(orderID string) error {
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

// CancelAllOrders submits a request to cancel all orders.
func (c *Client) CancelAllOrders() error {
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

type GetAssetsRequest struct {
	Status     string
	AssetClass string
	Exchange   string
}

// GetAssets returns the list of assets.
func (c *Client) GetAssets(req GetAssetsRequest) ([]Asset, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if req.Status != "" {
		q.Set("status", req.Status)
	}
	if req.AssetClass != "" {
		q.Set("asset_class", req.AssetClass)
	}
	if req.Exchange != "" {
		q.Set("exchange", req.Exchange)
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
	u, err := url.Parse(fmt.Sprintf("%s/%s/assets/%v", c.opts.BaseURL, apiVersion, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var asset Asset
	if err = unmarshal(resp, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

type GetAnnouncementsRequest struct {
	CATypes  []string  `json:"ca_types"`
	Since    time.Time `json:"since"`
	Until    time.Time `json:"until"`
	Symbol   string    `json:"symbol"`
	Cusip    string    `json:"cusip"`
	DateType DateType  `json:"date_type"`
}

func (c *Client) GetAnnouncements(req GetAnnouncementsRequest) ([]Announcement, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/corporate_actions/announcements", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if len(req.CATypes) != 0 {
		q.Set("ca_types", strings.Join(req.CATypes, ","))
	}
	if !req.Since.IsZero() {
		q.Set("since", req.Since.Format("2006-01-02"))
	}
	if !req.Until.IsZero() {
		q.Set("until", req.Until.Format("2006-01-02"))
	}
	if req.Symbol != "" {
		q.Set("symbol", req.Symbol)
	}
	if req.Cusip != "" {
		q.Set("cusip", req.Cusip)
	}
	if req.DateType != "" {
		q.Set("date_type", string(req.DateType))
	}
	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var announcements []Announcement

	if err = unmarshal(resp, &announcements); err != nil {
		return nil, err
	}

	return announcements, nil
}

func (c *Client) GetAnnouncement(announcementID string) (*Announcement, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/corporate_actions/announcements/%s",
		c.opts.BaseURL, apiVersion, announcementID))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var announcement Announcement
	if err = unmarshal(resp, &announcement); err != nil {
		return nil, err
	}
	return &announcement, nil
}

// GetAccount returns the user's account information.
func GetAccount() (*Account, error) {
	return DefaultClient.GetAccount()
}

// GetConfigs returns the current account configurations
func GetAccountConfigurations() (*AccountConfigurations, error) {
	return DefaultClient.GetAccountConfigurations()
}

// EditConfigs patches the account configs
func UpdateAccountConfigurations(req UpdateAccountConfigurationsRequest) (*AccountConfigurations, error) {
	return DefaultClient.UpdateAccountConfigurations(req)
}

func GetAccountActivities(req GetAccountActivitiesRequest) ([]AccountActivity, error) {
	return DefaultClient.GetAccountActivities(req)
}

func GetPortfolioHistory(req GetPortfolioHistoryRequest) (*PortfolioHistory, error) {
	return DefaultClient.GetPortfolioHistory(req)
}

// GetPositions lists the account's open positions.
func GetPositions() ([]Position, error) {
	return DefaultClient.GetPositions()
}

// GetPosition returns the account's position for the provided symbol.
func GetPosition(symbol string) (*Position, error) {
	return DefaultClient.GetPosition(symbol)
}

// CloseAllPositions liquidates all open positions at market price.
func CloseAllPositions(req CloseAllPositionsRequest) error {
	return DefaultClient.CloseAllPositions(req)
}

// ClosePosition liquidates the position for the given symbol at market price.
func ClosePosition(symbol string, req ClosePositionRequest) error {
	return DefaultClient.ClosePosition(symbol, req)
}

// GetClock returns the current market clock.
func GetClock() (*Clock, error) {
	return DefaultClient.GetClock()
}

// GetCalendar returns the market calendar.
func GetCalendar(req GetCalendarRequest) ([]CalendarDay, error) {
	return DefaultClient.GetCalendar(req)
}

// GetOrders returns the list of orders for an account.
func GetOrders(req GetOrdersRequest) ([]Order, error) {
	return DefaultClient.GetOrders(req)
}

// PlaceOrder submits an order request to buy or sell an asset.
func PlaceOrder(req PlaceOrderRequest) (*Order, error) {
	return DefaultClient.PlaceOrder(req)
}

// GetOrder submits a request to get an order by the order ID.
func GetOrder(orderID string) (*Order, error) {
	return DefaultClient.GetOrder(orderID)
}

// GetOrderByClientOrderID submits a request to get an order by the client order ID.
func GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
	return DefaultClient.GetOrderByClientOrderID(clientOrderID)
}

// ReplaceOrder submits a request to replace an order by id
func ReplaceOrder(orderID string, req ReplaceOrderRequest) (*Order, error) {
	return DefaultClient.ReplaceOrder(orderID, req)
}

// CancelOrder submits a request to cancel an open order.
func CancelOrder(orderID string) error {
	return DefaultClient.CancelOrder(orderID)
}

// CancelAllOrders submits a request to cancel all orders.
func CancelAllOrders() error {
	return DefaultClient.CancelAllOrders()
}

// GetAssets returns the list of assets.
func GetAssets(req GetAssetsRequest) ([]Asset, error) {
	return DefaultClient.GetAssets(req)
}

// GetAsset returns an asset for the given symbol.
func GetAsset(symbol string) (*Asset, error) {
	return DefaultClient.GetAsset(symbol)
}

// GetAnnouncements returns a list of announcements
// with the default Alpaca client.
func GetAnnouncements(req GetAnnouncementsRequest) ([]Announcement, error) {
	return DefaultClient.GetAnnouncements(req)
}

// GetAnnouncement returns a single announcement
// with the default Alpaca client.
func GetAnnouncement(announcementID string) (*Announcement, error) {
	return DefaultClient.GetAnnouncement(announcementID)
}

func (c *Client) get(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	return c.do(c, req)
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

	return c.do(c, req)
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

	return c.do(c, req)
}

func (c *Client) delete(u *url.URL) (*http.Response, error) {
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

		body, err := io.ReadAll(resp.Body)
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
	defer func() {
		// The underlying TCP connection can not be reused if the body is not fully read
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	return json.NewDecoder(resp.Body).Decode(data)
}
