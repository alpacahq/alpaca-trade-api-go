package alpaca

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/civil"
	"github.com/mailru/easyjson"
	"github.com/shopspring/decimal"
)

// ClientOpts contains options for the alpaca client
type ClientOpts struct {
	APIKey       string
	APISecret    string
	BrokerKey    string
	BrokerSecret string
	OAuth        string
	BaseURL      string
	RetryLimit   int
	RetryDelay   time.Duration
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
	req.Header.Set("User-Agent", Version())

	switch {
	case c.opts.OAuth != "":
		req.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	case c.opts.BrokerKey != "":
		req.SetBasicAuth(c.opts.BrokerKey, c.opts.BrokerSecret)
	default:
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
		CloseResp(resp) // Close body before retrying
		time.Sleep(c.opts.RetryDelay)
	}

	if err = Verify(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// Helper function to build a URL with optional query parameters
func (c *Client) buildURL(endpoint string, queryParams map[string]string) (*url.URL, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s", c.opts.BaseURL, apiVersion, endpoint))
	if err != nil {
		return nil, err
	}
	if queryParams != nil {
		q := u.Query()
		for key, value := range queryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}
	return u, nil
}

// Helper function to make a GET request and unmarshal the response
func (c *Client) fetchAndUnmarshal(endpoint string, queryParams map[string]string, result easyjson.Unmarshaler) error {
	u, err := c.buildURL(endpoint, queryParams)
	if err != nil {
		return err
	}

	resp, err := c.get(u) //nolint:bodyclose // unmarshal closes the body
	if err != nil {
		return err
	}

	return unmarshal(resp, result)
}

// GetAccount returns the user's account information.
func (c *Client) GetAccount() (*Account, error) {
	var account Account
	err := c.fetchAndUnmarshal("account", nil, &account)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// GetAccountConfigurations returns the current account configurations
func (c *Client) GetAccountConfigurations() (*AccountConfigurations, error) {
	var configs AccountConfigurations
	err := c.fetchAndUnmarshal("account/configurations", nil, &configs)
	if err != nil {
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

// UpdateAccountConfigurations updates the account configs.
func (c *Client) UpdateAccountConfigurations(req UpdateAccountConfigurationsRequest) (*AccountConfigurations, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/account/configurations", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.patch(u, req) //nolint:bodyclose // Linter Error
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
	PageToken     string    `json:"page_token"`
	Category      string    `json:"category"`
}

// GetAccountActivities returns the account activities.
func (c *Client) GetAccountActivities(req GetAccountActivitiesRequest) ([]AccountActivity, error) {
	queryParams := map[string]string{}

	if len(req.ActivityTypes) > 0 {
		queryParams["activity_types"] = strings.Join(req.ActivityTypes, ",")
	}
	if !req.Date.IsZero() {
		queryParams["date"] = req.Date.UTC().Format(time.RFC3339Nano)
	}
	if !req.Until.IsZero() {
		queryParams["until"] = req.Until.UTC().Format(time.RFC3339Nano)
	}
	if !req.After.IsZero() {
		queryParams["after"] = req.After.UTC().Format(time.RFC3339Nano)
	}
	if req.Direction != "" {
		queryParams["direction"] = req.Direction
	}
	if req.PageSize != 0 {
		queryParams["page_size"] = strconv.Itoa(req.PageSize)
	}
	if req.PageToken != "" {
		queryParams["page_token"] = req.PageToken
	}
	if req.Category != "" {
		queryParams["category"] = req.Category
	}

	var activities accountSlice
	err := c.fetchAndUnmarshal("account/activities", queryParams, &activities)
	if err != nil {
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

// GetPortfolioHistory returns the portfolio history.
func (c *Client) GetPortfolioHistory(req GetPortfolioHistoryRequest) (*PortfolioHistory, error) {
	queryParams := map[string]string{}

	if req.Period != "" {
		queryParams["period"] = req.Period
	}
	if req.TimeFrame != "" {
		queryParams["timeframe"] = string(req.TimeFrame)
	}
	if !req.DateEnd.IsZero() {
		queryParams["date_end"] = req.DateEnd.Format("2006-01-02")
	}
	queryParams["extended_hours"] = strconv.FormatBool(req.ExtendedHours)

	var history PortfolioHistory
	err := c.fetchAndUnmarshal("account/portfolio/history", queryParams, &history)
	if err != nil {
		return nil, err
	}
	return &history, nil
}

// GetPositions returns the account's open positions.
func (c *Client) GetPositions() ([]Position, error) {
	var positions positionSlice
	err := c.fetchAndUnmarshal("positions", nil, &positions)
	if err != nil {
		return nil, err
	}
	return positions, nil
}

// GetPosition returns the account's position for the provided symbol.
func (c *Client) GetPosition(symbol string) (*Position, error) {
	var position Position
	err := c.fetchAndUnmarshal(fmt.Sprintf("positions/%s", symbol), map[string]string{"symbol": symbol}, &position)
	if err != nil {
		return nil, err
	}
	return &position, nil
}

type CloseAllPositionsRequest struct {
	CancelOrders bool
}

// CloseAllPositions liquidates all open positions at market price.
// It returns the list of orders that were created to close the positions.
// If errors occur while closing some of the positions, the errors will also be returned (possibly among orders)
func (c *Client) CloseAllPositions(req CloseAllPositionsRequest) ([]Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("cancel_orders", strconv.FormatBool(req.CancelOrders))
	u.RawQuery = q.Encode()

	resp, err := c.delete(u) //nolint:bodyclose // Linter Error
	if err != nil {
		return nil, err
	}

	var closeAllPositions closeAllPositionsSlice
	if err = unmarshal(resp, &closeAllPositions); err != nil {
		return nil, err
	}

	var (
		orders = make([]Order, 0, len(closeAllPositions))
		errs   = make([]error, 0, len(closeAllPositions))
	)
	for _, capr := range closeAllPositions {
		if capr.Status == http.StatusOK {
			var order Order
			if err := easyjson.Unmarshal(capr.Body, &order); err != nil {
				return nil, err
			}
			orders = append(orders, order)
			continue
		}
		var apiErr APIError
		if err := easyjson.Unmarshal(capr.Body, &apiErr); err != nil {
			return nil, err
		}
		apiErr.StatusCode = capr.Status
		errs = append(errs, &apiErr)
	}

	return orders, errors.Join(errs...)
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
func (c *Client) ClosePosition(symbol string, req ClosePositionRequest) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/positions/%s", c.opts.BaseURL, apiVersion, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	if !req.Qty.IsZero() {
		q.Set("qty", req.Qty.String())
	}
	if !req.Percentage.IsZero() {
		q.Set("percentage", req.Percentage.String())
	}
	u.RawQuery = q.Encode()

	resp, err := c.delete(u) //nolint:bodyclose // Linter Error
	if err != nil {
		return nil, err
	}

	var order Order
	if err = unmarshal(resp, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

// GetClock returns the current market clock.
func (c *Client) GetClock() (*Clock, error) {
	var clock Clock
	err := c.fetchAndUnmarshal("clock", nil, &clock)
	if err != nil {
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
	queryParams := map[string]string{}
	if !req.Start.IsZero() {
		queryParams["start"] = req.Start.Format("2006-01-02")
	}
	if !req.End.IsZero() {
		queryParams["end"] = req.End.Format("2006-01-02")
	}

	var calendar calendarDaySlice
	err := c.fetchAndUnmarshal("calendar", queryParams, &calendar)
	if err != nil {
		return nil, err
	}
	return calendar, nil
}

type GetOrdersRequest struct {
	// Status to be queried. Possible values: open, closed, all. Defaults to open.
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
	queryParams := map[string]string{}

	if req.Status != "" {
		queryParams["status"] = req.Status
	}
	if req.Limit != 0 {
		queryParams["limit"] = strconv.Itoa(req.Limit)
	}
	if !req.After.IsZero() {
		queryParams["after"] = req.After.Format(time.RFC3339)
	}
	if !req.Until.IsZero() {
		queryParams["until"] = req.Until.Format(time.RFC3339)
	}
	if req.Direction != "" {
		queryParams["direction"] = req.Direction
	}
	if req.Side != "" {
		queryParams["side"] = req.Side
	}
	queryParams["nested"] = strconv.FormatBool(req.Nested)

	if len(req.Symbols) > 0 {
		queryParams["symbols"] = strings.Join(req.Symbols, ",")
	}

	var orders orderSlice
	err := c.fetchAndUnmarshal("orders", queryParams, &orders)
	if err != nil {
		return nil, err
	}
	return orders, nil
}

type Leg struct {
	Side           Side            `json:"side"`
	PositionIntent PositionIntent  `json:"position_intent"`
	Symbol         string          `json:"symbol"`
	RatioQty       decimal.Decimal `json:"ratio_qty"`
}

type PlaceOrderRequest struct {
	Symbol         string           `json:"symbol"`
	Qty            *decimal.Decimal `json:"qty"`
	Notional       *decimal.Decimal `json:"notional"`
	Side           Side             `json:"side"`
	Type           OrderType        `json:"type"`
	TimeInForce    TimeInForce      `json:"time_in_force"`
	LimitPrice     *decimal.Decimal `json:"limit_price"`
	ExtendedHours  bool             `json:"extended_hours"`
	StopPrice      *decimal.Decimal `json:"stop_price"`
	ClientOrderID  string           `json:"client_order_id"`
	OrderClass     OrderClass       `json:"order_class"`
	TakeProfit     *TakeProfit      `json:"take_profit"`
	StopLoss       *StopLoss        `json:"stop_loss"`
	TrailPrice     *decimal.Decimal `json:"trail_price"`
	TrailPercent   *decimal.Decimal `json:"trail_percent"`
	PositionIntent PositionIntent   `json:"position_intent,omitempty"`
	Legs           []Leg            `json:"legs"` // mleg order legs
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

	resp, err := c.post(u, req) //nolint:bodyclose // Linter Error
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
	var order Order
	err := c.fetchAndUnmarshal(fmt.Sprintf("orders/%s", orderID), nil, &order)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetOrderByClientOrderID submits a request to get an order by the client order ID.
func (c *Client) GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
	queryParams := map[string]string{
		"client_order_id": clientOrderID,
	}

	var order Order
	err := c.fetchAndUnmarshal("orders:by_client_order_id", queryParams, &order)
	if err != nil {
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

	resp, err := c.patch(u, req) //nolint:bodyclose // Linter Error
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

	// Verify the response and close the body, if error happens, verify will return the error and close the body
	responseVal := Verify(resp)
	if responseVal == nil {
		CloseResp(resp)
	}
	return responseVal
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
	responseVal := Verify(resp)
	if responseVal == nil {
		CloseResp(resp)
	}
	return responseVal
}

type GetAssetsRequest struct {
	Status     string
	AssetClass string
	Exchange   string
}

// GetAssets returns the list of assets.
func (c *Client) GetAssets(req GetAssetsRequest) ([]Asset, error) {
	queryParams := map[string]string{}

	if req.Status != "" {
		queryParams["status"] = req.Status
	}
	if req.AssetClass != "" {
		queryParams["asset_class"] = req.AssetClass
	}
	if req.Exchange != "" {
		queryParams["exchange"] = req.Exchange
	}

	var assets assetSlice
	err := c.fetchAndUnmarshal("assets", queryParams, &assets)
	if err != nil {
		return nil, err
	}
	return assets, nil
}

// GetAsset returns an asset for the given symbol.
func (c *Client) GetAsset(symbol string) (*Asset, error) {
	var asset Asset
	err := c.fetchAndUnmarshal(fmt.Sprintf("assets/%v", symbol), nil, &asset)
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

const (
	optionContractsRequestsMaxLimit = 10000
)

func setQueryLimit(q url.Values, totalLimit, pageLimit, received, maxLimit int) {
	limit := 0 // use server side default if unset
	if pageLimit != 0 {
		limit = pageLimit
	}
	if totalLimit != 0 {
		remaining := totalLimit - received
		if remaining <= 0 { // this should never happen
			return
		}
		if (limit == 0 || limit > remaining) && remaining <= maxLimit {
			limit = remaining
		}
	}

	if limit != 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
}

type GetOptionContractsRequest struct {
	UnderlyingSymbols     string
	ShowDeliverable       bool
	Status                OptionStatus
	ExpirationDate        civil.Date
	ExpirationDateGTE     civil.Date
	ExpirationDateLTE     civil.Date
	RootSymbol            string
	Type                  OptionType
	Style                 OptionStyle
	StrikePriceGTE        decimal.Decimal
	StrikePriceLTE        decimal.Decimal
	PennyProgramIndicator bool
	PageLimit             int
	TotalLimit            int
}

// GetOptionContracts returns the list of Option Contracts.
func (c *Client) GetOptionContracts(req GetOptionContractsRequest) ([]OptionContract, error) {
	queryParams := buildOptionContractsQueryParams(req)
	optionContracts := make([]OptionContract, 0)

	for req.TotalLimit == 0 || len(optionContracts) < req.TotalLimit {
		resp, nextPageToken, err := c.fetchOptionContracts(queryParams, req.TotalLimit,
			req.PageLimit, len(optionContracts))
		if err != nil {
			return nil, err
		}

		optionContracts = append(optionContracts, resp.OptionContracts...)

		if nextPageToken == "" {
			break
		}

		queryParams["page_token"] = nextPageToken
	}

	return optionContracts, nil
}

// buildOptionContractsQueryParams constructs query parameters from request
func buildOptionContractsQueryParams(req GetOptionContractsRequest) map[string]string {
	qp := map[string]string{
		"show_deliverables": strconv.FormatBool(req.ShowDeliverable),
	}

	if req.UnderlyingSymbols != "" {
		qp["underlying_symbols"] = req.UnderlyingSymbols
	}
	if req.Status != "" {
		qp["status"] = string(req.Status)
	}
	if req.RootSymbol != "" {
		qp["root_symbol"] = req.RootSymbol
	}
	if req.Type != "" {
		qp["type"] = string(req.Type)
	}
	if req.Style != "" {
		qp["style"] = string(req.Style)
	}
	if req.PennyProgramIndicator {
		qp["ppind"] = "true"
	}

	// Date filters
	addDateParam(qp, "expiration_date", req.ExpirationDate)
	addDateParam(qp, "expiration_date_gte", req.ExpirationDateGTE)
	addDateParam(qp, "expiration_date_lte", req.ExpirationDateLTE)

	// Price filters
	addDecimalParam(qp, "strike_price_lte", req.StrikePriceLTE)
	addDecimalParam(qp, "strike_price_gte", req.StrikePriceGTE)

	return qp
}

// addDateParam adds a date parameter if it's not zero
func addDateParam(qp map[string]string, key string, value civil.Date) {
	qp[key] = value.String()
}

// addDecimalParam adds a decimal parameter if it's not zero
func addDecimalParam(qp map[string]string, key string, value decimal.Decimal) {
	if !value.IsZero() {
		qp[key] = value.String()
	}
}

// fetchOptionContracts makes an API request and returns response + pagination token
func (c *Client) fetchOptionContracts(queryParams map[string]string, totalLimit, pageLimit,
	fetched int) (optionContractsResponse, string, error) {
	u, err := c.buildURL("options/contracts", queryParams)
	if err != nil {
		return optionContractsResponse{}, "", err
	}

	// Set pagination limit
	q := u.Query()
	setQueryLimit(q, totalLimit, pageLimit, fetched, optionContractsRequestsMaxLimit)
	u.RawQuery = q.Encode()

	resp, err := c.get(u) //nolint:bodyclose // unmarshal closes the body
	if err != nil {
		return optionContractsResponse{}, "", err
	}

	var response optionContractsResponse
	if err = unmarshal(resp, &response); err != nil {
		return optionContractsResponse{}, "", err
	}

	nextPageToken := ""
	if response.NextPageToken != nil {
		nextPageToken = *response.NextPageToken
	}

	return response, nextPageToken, nil
}

// GetOptionContract returns an option contract by symbol or contract ID.
func (c *Client) GetOptionContract(symbolOrID string) (*OptionContract, error) {
	var optionContract OptionContract
	err := c.fetchAndUnmarshal(fmt.Sprintf("options/contracts/%v", symbolOrID), nil, &optionContract)
	if err != nil {
		return nil, err
	}
	return &optionContract, nil
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
	queryParams := make(map[string]string)

	if len(req.CATypes) != 0 {
		queryParams["ca_types"] = strings.Join(req.CATypes, ",")
	}
	if !req.Since.IsZero() {
		queryParams["since"] = req.Since.Format("2006-01-02")
	}
	if !req.Until.IsZero() {
		queryParams["until"] = req.Until.Format("2006-01-02")
	}
	if req.Symbol != "" {
		queryParams["symbol"] = req.Symbol
	}
	if req.Cusip != "" {
		queryParams["cusip"] = req.Cusip
	}
	if req.DateType != "" {
		queryParams["date_type"] = string(req.DateType)
	}

	var announcements announcementSlice
	if err := c.fetchAndUnmarshal("corporate_actions/announcements", queryParams, &announcements); err != nil {
		return nil, err
	}

	return announcements, nil
}

func (c *Client) GetAnnouncement(announcementID string) (*Announcement, error) {
	var announcement Announcement
	if err := c.fetchAndUnmarshal(fmt.Sprintf("corporate_actions/announcements/%s", announcementID),
		nil, &announcement); err != nil {
		return nil, err
	}
	return &announcement, nil
}

// GetAccount returns the user's account information.
func (c *Client) GetWatchlists() ([]Watchlist, error) {
	var watchlists watchlistSlice
	if err := c.fetchAndUnmarshal("watchlists", nil, &watchlists); err != nil {
		return nil, err
	}
	return watchlists, nil
}

func (c *Client) CreateWatchlist(req CreateWatchlistRequest) (*Watchlist, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/watchlists", c.opts.BaseURL, apiVersion))
	if err != nil {
		return nil, err
	}

	resp, err := c.post(u, req) //nolint:bodyclose // Linter Error
	if err != nil {
		return nil, err
	}

	watchlist := &Watchlist{}
	if err = unmarshal(resp, watchlist); err != nil {
		return nil, err
	}
	return watchlist, nil
}

func (c *Client) GetWatchlist(watchlistID string) (*Watchlist, error) {
	var watchlist Watchlist
	if err := c.fetchAndUnmarshal(fmt.Sprintf("watchlists/%s", watchlistID),
		nil, &watchlist); err != nil {
		return nil, err
	}
	return &watchlist, nil
}

func (c *Client) UpdateWatchlist(watchlistID string, req UpdateWatchlistRequest) (*Watchlist, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/watchlists/%s", c.opts.BaseURL, apiVersion, watchlistID))
	if err != nil {
		return nil, err
	}

	resp, err := c.put(u, req) //nolint:bodyclose // Linter Error
	if err != nil {
		return nil, err
	}

	watchlist := &Watchlist{}
	if err = unmarshal(resp, watchlist); err != nil {
		return nil, err
	}
	return watchlist, nil
}

var ErrSymbolMissing = errors.New("symbol missing from request")

func (c *Client) AddSymbolToWatchlist(watchlistID string, req AddSymbolToWatchlistRequest) (*Watchlist, error) {
	if req.Symbol == "" {
		return nil, ErrSymbolMissing
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/watchlists/%s", c.opts.BaseURL, apiVersion, watchlistID))
	if err != nil {
		return nil, err
	}

	resp, err := c.post(u, req) //nolint:bodyclose // Linter Error
	if err != nil {
		return nil, err
	}

	watchlist := &Watchlist{}
	if err = unmarshal(resp, watchlist); err != nil {
		return nil, err
	}
	return watchlist, nil
}

func (c *Client) RemoveSymbolFromWatchlist(watchlistID string, req RemoveSymbolFromWatchlistRequest) error {
	if req.Symbol == "" {
		return ErrSymbolMissing
	}

	u, err := url.Parse(fmt.Sprintf("%s/%s/watchlists/%s/%s", c.opts.BaseURL,
		apiVersion, watchlistID, req.Symbol))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}
	CloseResp(resp)
	return nil
}

func (c *Client) DeleteWatchlist(watchlistID string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s/watchlists/%s", c.opts.BaseURL, apiVersion, watchlistID))
	if err != nil {
		return err
	}

	resp, err := c.delete(u)
	if err != nil {
		return err
	}
	CloseResp(resp)
	return nil
}

// GetAccount returns the user's account information
// using the default Alpaca client.
func GetAccount() (*Account, error) {
	return DefaultClient.GetAccount()
}

// GetAccountConfigurations returns the current account configurations
func GetAccountConfigurations() (*AccountConfigurations, error) {
	return DefaultClient.GetAccountConfigurations()
}

// UpdateAccountConfigurations updates the account configs.
func UpdateAccountConfigurations(req UpdateAccountConfigurationsRequest) (*AccountConfigurations, error) {
	return DefaultClient.UpdateAccountConfigurations(req)
}

// GetAccountActivities returns the account activities.
func GetAccountActivities(req GetAccountActivitiesRequest) ([]AccountActivity, error) {
	return DefaultClient.GetAccountActivities(req)
}

// GetPortfolioHistory returns the portfolio history.
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
func CloseAllPositions(req CloseAllPositionsRequest) ([]Order, error) {
	return DefaultClient.CloseAllPositions(req)
}

// ClosePosition liquidates the position for the given symbol at market price.
func ClosePosition(symbol string, req ClosePositionRequest) (*Order, error) {
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

// GetOptionContracts returns the list of Option Contracts.
func GetOptionContracts(req GetOptionContractsRequest) ([]OptionContract, error) {
	return DefaultClient.GetOptionContracts(req)
}

// GetOptionContract returns an option contract by symbol or contract ID.
func GetOptionContract(symbolOrID string) (*OptionContract, error) {
	return DefaultClient.GetOptionContract(symbolOrID)
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

// GetWatchlists returns a list of watchlists
// with the default Alpaca client.
func GetWatchlists() ([]Watchlist, error) {
	return DefaultClient.GetWatchlists()
}

// CreateWatchlist creates a new watchlist
// with the default Alpaca client.
func CreateWatchlist(req CreateWatchlistRequest) (*Watchlist, error) {
	return DefaultClient.CreateWatchlist(req)
}

// GetWatchlist returns a single watchlist by getting the watchlist id
// with the default Alpaca client.
func GetWatchlist(watchlistID string) (*Watchlist, error) {
	return DefaultClient.GetWatchlist(watchlistID)
}

// UpdateWatchlist updates a watchlist by getting the watchlist id
// with the default Alpaca client.
func UpdateWatchlist(watchlistID string, req UpdateWatchlistRequest) (*Watchlist, error) {
	return DefaultClient.UpdateWatchlist(watchlistID, req)
}

// DeleteWatchlist deletes a watchlist by getting the watchlist id
// with the default Alpaca client.
func DeleteWatchlist(watchlistID string) error {
	return DefaultClient.DeleteWatchlist(watchlistID)
}

// AddSymbolToWatchlist adds an asset to a watchlist by getting the watchlist id
// with the default Alpaca client.
func AddSymbolToWatchlist(watchlistID string, req AddSymbolToWatchlistRequest) (*Watchlist, error) {
	return DefaultClient.AddSymbolToWatchlist(watchlistID, req)
}

// RemoveSymbolFromWatchlist removes an asset from a watchlist by getting the watchlist id
// with the default Alpaca client.
func RemoveSymbolFromWatchlist(watchlistID string, req RemoveSymbolFromWatchlistRequest) error {
	return DefaultClient.RemoveSymbolFromWatchlist(watchlistID, req)
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

func (c *Client) put(u *url.URL, data interface{}) (*http.Response, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewReader(buf))
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

func Verify(resp *http.Response) error {
	if resp.StatusCode >= http.StatusMultipleChoices {
		return APIErrorFromResponse(resp)
	}
	return nil
}

func unmarshal(resp *http.Response, v easyjson.Unmarshaler) error {
	defer CloseResp(resp)
	return easyjson.UnmarshalFromReader(resp.Body, v)
}

func CloseResp(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	// The underlying TCP connection cannot be reused if the body is not fully read
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
