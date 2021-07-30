package alpaca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v2/common"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
)

const (
	rateLimitRetryCount = 3
	rateLimitRetryDelay = time.Second
)

var (
	// DefaultClient is the default Alpaca client using the
	// environment variable set credentials
	DefaultClient = NewClient(common.Credentials())
	base          = "https://api.alpaca.markets"
	dataURL       = "https://data.alpaca.markets"
	apiVersion    = "v2"
	clientTimeout = 10 * time.Second
	do            = defaultDo
)

func defaultDo(c *Client, req *http.Request) (*http.Response, error) {
	if c.credentials.OAuth != "" {
		req.Header.Set("Authorization", "Bearer "+c.credentials.OAuth)
	} else {
		req.Header.Set("APCA-API-KEY-ID", c.credentials.ID)
		req.Header.Set("APCA-API-SECRET-KEY", c.credentials.Secret)
	}

	client := &http.Client{
		Timeout: clientTimeout,
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
		if i >= rateLimitRetryCount {
			break
		}
		time.Sleep(rateLimitRetryDelay)
	}

	if err = verify(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

// TODO: Move to the marketdata package
const (
	v2DefaultLimit = 1000
	v2MaxLimit     = 10000
)

func init() {
	if s := os.Getenv("APCA_API_BASE_URL"); s != "" {
		base = s
	} else if s := os.Getenv("ALPACA_BASE_URL"); s != "" {
		// legacy compatibility...
		base = s
	}
	if s := os.Getenv("APCA_DATA_URL"); s != "" {
		dataURL = s
	}
	// also allow APCA_API_DATA_URL to be consistent with the python SDK
	if s := os.Getenv("APCA_API_DATA_URL"); s != "" {
		dataURL = s
	}
	if s := os.Getenv("APCA_API_VERSION"); s != "" {
		apiVersion = s
	}
	if s := os.Getenv("APCA_API_CLIENT_TIMEOUT"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			log.Fatal("invalid APCA_API_CLIENT_TIMEOUT: " + err.Error())
		}
		clientTimeout = d
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

func (c *Client) GetAccountActivities(activityType *string, opts *AccountActivitiesRequest) ([]AccountActivity, error) {
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

// GetAggregates returns the bars for the given symbol, timespan and date-range.
//
// Deprecated: all v1 endpoints will be removed!
func (c *Client) GetAggregates(symbol, timespan, from, to string) (*Aggregates, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/aggs/ticker/%s/range/1/%s/%s/%s",
		dataURL, symbol, timespan, from, to))
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

// GetLastQuote returns the last quote for the given symbol.
//
// Deprecated: all v1 endpoints will be removed!
func (c *Client) GetLastQuote(symbol string) (*LastQuoteResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/last_quote/stocks/%s", dataURL, symbol))
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

// GetLastTrade returns the last trade for the given symbol.
//
// Deprecated: all v1 endpoints will be removed!
func (c *Client) GetLastTrade(symbol string) (*LastTradeResponse, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v1/last/stocks/%s", dataURL, symbol))
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

func setBaseQuery(q url.Values, start, end *time.Time, feed string) {
	if start != nil {
		q.Set("start", start.Format(time.RFC3339))
	}
	if end != nil {
		q.Set("end", end.Format(time.RFC3339))
	}
	if feed != "" {
		q.Set("feed", feed)
	}
}

func setQueryLimit(q url.Values, totalLimit *int, pageLimit *int, received int) {
	limit := v2DefaultLimit
	if pageLimit != nil {
		limit = *pageLimit
	}
	if limit > v2MaxLimit {
		limit = v2MaxLimit
	}
	if totalLimit != nil {
		remaining := *totalLimit - received
		if remaining <= 0 {
			return
		}
		if limit > remaining {
			limit = remaining
		}
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
}

// GetTradesParams contains optional parameters for getting trades.
type GetTradesParams struct {
	// Start is the inclusive beginning of the interval
	Start *time.Time
	// End is the inclusive end of the interval
	End *time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit *int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit *int
	// Feed is the source of the data: sip or iex.
	Feed string
}

// GetTrades returns the trades for the given symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetTradesAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetTrades(symbol string, params GetTradesParams) ([]marketdata.Trade, error) {
	trades := make([]marketdata.Trade, 0)
	for item := range c.GetTradesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		trades = append(trades, item.Trade)
	}
	return trades, nil
}

// GetTradesAsync returns a channel that will be populated with the trades for the given symbol.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetTradesAsync(symbol string, params GetTradesParams) <-chan marketdata.TradeItem {
	ch := make(chan marketdata.TradeItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/trades", dataURL, symbol))
		if err != nil {
			ch <- marketdata.TradeItem{Error: err}
			return
		}

		q := u.Query()
		setBaseQuery(q, params.Start, params.End, params.Feed)

		received := 0
		for {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- marketdata.TradeItem{Error: err}
				return
			}

			var tradeResp tradeResponse
			if err = unmarshal(resp, &tradeResp); err != nil {
				ch <- marketdata.TradeItem{Error: err}
				return
			}

			for _, trade := range tradeResp.Trades {
				ch <- marketdata.TradeItem{Trade: trade}
			}
			if tradeResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *tradeResp.NextPageToken)
			received += len(tradeResp.Trades)
		}
	}()

	return ch
}

// GetQuotesParams contains optional parameters for getting quotes
type GetQuotesParams struct {
	// Start is the inclusive beginning of the interval
	Start *time.Time
	// End is the inclusive end of the interval
	End *time.Time
	// TotalLimit is the limit of the total number of the returned quotes.
	// If missing, all quotes between start end end will be returned.
	TotalLimit *int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit *int
	// Feed is the source of the data: sip or iex.
	Feed string
}

// GetQuotes returns the quotes for the given symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetQuotesAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetQuotes(symbol string, params GetQuotesParams) ([]marketdata.Quote, error) {
	quotes := make([]marketdata.Quote, 0)
	for item := range c.GetQuotesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		quotes = append(quotes, item.Quote)
	}
	return quotes, nil
}

// GetQuotesAsync returns a channel that will be populated with the quotes for the given symbol.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetQuotesAsync(symbol string, params GetQuotesParams) <-chan marketdata.QuoteItem {
	// NOTE: this method is very similar to GetTrades.
	// With generics it would be almost trivial to refactor them to use a common base method,
	// but without them it doesn't seem to be worth it
	ch := make(chan marketdata.QuoteItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/quotes", dataURL, symbol))
		if err != nil {
			ch <- marketdata.QuoteItem{Error: err}
			return
		}

		q := u.Query()
		setBaseQuery(q, params.Start, params.End, params.Feed)

		received := 0
		for {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- marketdata.QuoteItem{Error: err}
				return
			}

			var quoteResp quoteResponse
			if err = unmarshal(resp, &quoteResp); err != nil {
				ch <- marketdata.QuoteItem{Error: err}
				return
			}

			for _, quote := range quoteResp.Quotes {
				ch <- marketdata.QuoteItem{Quote: quote}
			}
			if quoteResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *quoteResp.NextPageToken)
			received += len(quoteResp.Quotes)
		}
	}()

	return ch
}

// GetBarsParams contains optional parameters for getting bars
type GetBarsParams struct {
	// TimeFrame is the aggregation size of the bars
	TimeFrame marketdata.TimeFrame
	// Adjustment tells if the bars should be adjusted for corporate actions
	Adjustment marketdata.Adjustment
	// Start is the inclusive beginning of the interval
	Start *time.Time
	// End is the inclusive end of the interval
	End *time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit *int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit *int
	// Feed is the source of the data: sip or iex.
	Feed string
}

func setQueryBarParams(q url.Values, params GetBarsParams) {
	setBaseQuery(q, params.Start, params.End, params.Feed)
	// TODO: Replace with All once it's supported
	adjustment := marketdata.Raw
	if params.Adjustment != "" {
		adjustment = params.Adjustment
	}
	q.Set("adjustment", string(adjustment))
	timeframe := marketdata.Day
	if params.TimeFrame != "" {
		timeframe = params.TimeFrame
	}
	q.Set("timeframe", string(timeframe))
}

// GetBars returns a slice of bars for the given symbol.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetBars(symbol string, params GetBarsParams) ([]marketdata.Bar, error) {
	bars := make([]marketdata.Bar, 0)
	for item := range c.GetBarsAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars = append(bars, item.Bar)
	}
	return bars, nil
}

// GetBarsAsync returns a channel that will be populated with the bars for the given symbol.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetBarsAsync(symbol string, params GetBarsParams) <-chan marketdata.BarItem {
	ch := make(chan marketdata.BarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/bars", dataURL, symbol))
		if err != nil {
			ch <- marketdata.BarItem{Error: err}
			return
		}

		q := u.Query()
		setQueryBarParams(q, params)

		received := 0
		for {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- marketdata.BarItem{Error: err}
				return
			}

			var barResp barResponse
			if err = unmarshal(resp, &barResp); err != nil {
				ch <- marketdata.BarItem{Error: err}
				return
			}

			for _, bar := range barResp.Bars {
				ch <- marketdata.BarItem{Bar: bar}
			}
			if barResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *barResp.NextPageToken)
			received += len(barResp.Bars)
		}
	}()

	return ch
}

// GetMultiBars returns a slice of bars for the given symbols.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetMultiBars(
	symbols []string, params GetBarsParams,
) (map[string][]marketdata.Bar, error) {
	bars := make(map[string][]marketdata.Bar, len(symbols))
	for item := range c.GetMultiBarsAsync(symbols, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars[item.Symbol] = append(bars[item.Symbol], item.Bar)
	}
	return bars, nil
}

// GetBars returns a channel that will be populated with the bars for the requested symbols.
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetMultiBarsAsync(symbols []string, params GetBarsParams) <-chan marketdata.MultiBarItem {
	ch := make(chan marketdata.MultiBarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/bars", dataURL))
		if err != nil {
			ch <- marketdata.MultiBarItem{Error: err}
			return
		}

		q := u.Query()
		q.Set("symbols", strings.Join(symbols, ","))
		setQueryBarParams(q, params)

		received := 0
		for {
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- marketdata.MultiBarItem{Error: err}
				return
			}

			var barResp multiBarResponse
			if err = unmarshal(resp, &barResp); err != nil {
				ch <- marketdata.MultiBarItem{Error: err}
				return
			}

			sortedSymbols := make([]string, 0, len(barResp.Bars))
			for symbol := range barResp.Bars {
				sortedSymbols = append(sortedSymbols, symbol)
			}
			sort.Strings(sortedSymbols)

			for _, symbol := range sortedSymbols {
				bars := barResp.Bars[symbol]
				for _, bar := range bars {
					ch <- marketdata.MultiBarItem{Symbol: symbol, Bar: bar}
				}
				received += len(bars)
			}
			if barResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *barResp.NextPageToken)
		}
	}()

	return ch
}

// GetLatestTrade returns the latest trade for a given symbol
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetLatestTrade(symbol string) (*marketdata.Trade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/trades/latest", dataURL, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestTradeResp latestTradeResponse

	if err = unmarshal(resp, &latestTradeResp); err != nil {
		return nil, err
	}

	return &latestTradeResp.Trade, nil
}

// GetLatestQuote returns the latest quote for a given symbol
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetLatestQuote(symbol string) (*marketdata.Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/quotes/latest", dataURL, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestQuoteResp latestQuoteResponse

	if err = unmarshal(resp, &latestQuoteResp); err != nil {
		return nil, err
	}

	return &latestQuoteResp.Quote, nil
}

// GetSnapshot returns the snapshot for a given symbol
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetSnapshot(symbol string) (*marketdata.Snapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/snapshot", dataURL, symbol))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshot marketdata.Snapshot

	if err = unmarshal(resp, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetSnapshots returns the snapshots for multiple symbol
//
// Deprecated: will be moved to the marketdata package!
func (c *Client) GetSnapshots(symbols []string) (map[string]*marketdata.Snapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/snapshots?symbols=%s",
		dataURL, strings.Join(symbols, ",")))
	if err != nil {
		return nil, err
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshots map[string]*marketdata.Snapshot

	if err = unmarshal(resp, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
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

// GetOrderByClientOrderID submits a request to get an order by the client order ID.
func (c *Client) GetOrderByClientOrderID(clientOrderID string) (*Order, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/orders:by_client_order_id", base, apiVersion))
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
//
// Deprecated: all v1 endpoints will be removed!
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

	u, err := url.Parse(fmt.Sprintf("%s/v1/bars/%s?%v", dataURL, opts.Timeframe, vals.Encode()))
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
// data for one symbol.
//
// Deprecated: all v1 endpoints will be removed!
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

// GetAggregates returns the bars for the given symbol, timespan and date-range.
//
// Deprecated: all v1 endpoints will be removed!
func GetAggregates(symbol, timespan, from, to string) (*Aggregates, error) {
	return DefaultClient.GetAggregates(symbol, timespan, from, to)
}

// GetLastQuote returns the last quote for the given symbol.
//
// Deprecated: all v1 endpoints will be removed!
func GetLastQuote(symbol string) (*LastQuoteResponse, error) {
	return DefaultClient.GetLastQuote(symbol)
}

// GetLastTrade returns the last trade for the given symbol.
//
// Deprecated: all v1 endpoints will be removed!
func GetLastTrade(symbol string) (*LastTradeResponse, error) {
	return DefaultClient.GetLastTrade(symbol)
}

// GetTrades returns the trades for the given symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetTradesAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func GetTrades(symbol string, params GetTradesParams) ([]marketdata.Trade, error) {
	return DefaultClient.GetTrades(symbol, params)
}

// GetTradesAsync returns a channel that will be populated with the trades for the given symbol
// that happened between the given start and end times, limited to the given limit.
//
// Deprecated: will be moved to the marketdata package!
func GetTradesAsync(symbol string, params GetTradesParams) <-chan marketdata.TradeItem {
	return DefaultClient.GetTradesAsync(symbol, params)
}

// GetQuotes returns the quotes for the given symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetQuotesAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func GetQuotes(symbol string, params GetQuotesParams) ([]marketdata.Quote, error) {
	return DefaultClient.GetQuotes(symbol, params)
}

// GetQuotesAsync returns a channel that will be populated with the quotes for the given symbol
// that happened between the given start and end times, limited to the given limit.
//
// Deprecated: will be moved to the marketdata package!
func GetQuotesAsync(symbol string, params GetQuotesParams) <-chan marketdata.QuoteItem {
	return DefaultClient.GetQuotesAsync(symbol, params)
}

// GetBars returns the bars for the given symbol. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetBarsAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func GetBars(symbol string, params GetBarsParams) ([]marketdata.Bar, error) {
	return DefaultClient.GetBars(symbol, params)
}

// GetBarsAsync returns a channel that will be populated with the bars for the given symbol.
//
// Deprecated: will be moved to the marketdata package!
func GetBarsAsync(symbol string, params GetBarsParams) <-chan marketdata.BarItem {
	return DefaultClient.GetBarsAsync(symbol, params)
}

// GetMultiBars returns the bars for the given symbols. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetMultiBarsAsync instead!
//
// Deprecated: will be moved to the marketdata package!
func GetMultiBars(symbols []string, params GetBarsParams) (map[string][]marketdata.Bar, error) {
	return DefaultClient.GetMultiBars(symbols, params)
}

// GetMultiBarsAsync returns a channel that will be populated with the bars for the given symbols.
//
// Deprecated: will be moved to the marketdata package!
func GetMultiBarsAsync(symbols []string, params GetBarsParams) <-chan marketdata.MultiBarItem {
	return DefaultClient.GetMultiBarsAsync(symbols, params)
}

// GetLatestTrade returns the latest trade for a given symbol.
//
// Deprecated: will be moved to the marketdata package!
func GetLatestTrade(symbol string) (*marketdata.Trade, error) {
	return DefaultClient.GetLatestTrade(symbol)
}

// GetLatestTrade returns the latest quote for a given symbol.
//
// Deprecated: will be moved to the marketdata package!
func GetLatestQuote(symbol string) (*marketdata.Quote, error) {
	return DefaultClient.GetLatestQuote(symbol)
}

// GetSnapshot returns the snapshot for a given symbol
//
// Deprecated: will be moved to the marketdata package!
func GetSnapshot(symbol string) (*marketdata.Snapshot, error) {
	return DefaultClient.GetSnapshot(symbol)
}

// GetSnapshots returns the snapshots for a multiple symbols
//
// Deprecated: will be moved to the marketdata package!
func GetSnapshots(symbols []string) (map[string]*marketdata.Snapshot, error) {
	return DefaultClient.GetSnapshots(symbols)
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

// ListBars returns a map of bar lists corresponding to the provided
// symbol list that is filtered by the provided parameters with the default
// Alpaca client.
//
// Deprecated: all v1 endpoints will be removed!
func ListBars(symbols []string, opts ListBarParams) (map[string][]Bar, error) {
	return DefaultClient.ListBars(symbols, opts)
}

// GetSymbolBars returns a list of bars corresponding to the provided
// symbol that is filtered by the provided parameters with the default
// Alpaca client.
//
// Deprecated: all v1 endpoints will be removed!
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
		if err != nil {
			return fmt.Errorf("json unmarshal error: %s", err.Error())
		}
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
