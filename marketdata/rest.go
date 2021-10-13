package marketdata

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// Client is the alpaca marketdata client.
type Client interface {
	GetTrades(symbol string, params GetTradesParams) ([]Trade, error)
	GetTradesAsync(symbol string, params GetTradesParams) <-chan TradeItem
	GetMultiTrades(symbols []string, params GetTradesParams) (map[string][]Trade, error)
	GetMultiTradesAsync(symbols []string, params GetTradesParams) <-chan MultiTradeItem
	GetQuotes(symbol string, params GetQuotesParams) ([]Quote, error)
	GetQuotesAsync(symbol string, params GetQuotesParams) <-chan QuoteItem
	GetMultiQuotes(symbols []string, params GetQuotesParams) (map[string][]Quote, error)
	GetMultiQuotesAsync(symbols []string, params GetQuotesParams) <-chan MultiQuoteItem
	GetBars(symbol string, params GetBarsParams) ([]Bar, error)
	GetBarsAsync(symbol string, params GetBarsParams) <-chan BarItem
	GetMultiBars(symbols []string, params GetBarsParams) (map[string][]Bar, error)
	GetMultiBarsAsync(symbols []string, params GetBarsParams) <-chan MultiBarItem
	GetLatestBar(symbol string) (*Bar, error)
	GetLatestBars(symbols []string) (map[string]Bar, error)
	GetLatestTrade(symbol string) (*Trade, error)
	GetLatestTrades(symbols []string) (map[string]Trade, error)
	GetLatestQuote(symbol string) (*Quote, error)
	GetLatestQuotes(symbols []string) (map[string]Quote, error)
	GetSnapshot(symbol string) (*Snapshot, error)
	GetSnapshots(symbols []string) (map[string]*Snapshot, error)
	GetCryptoTrades(symbol string, params GetCryptoTradesParams) ([]CryptoTrade, error)
	GetCryptoTradesAsync(symbol string, params GetCryptoTradesParams) <-chan CryptoTradeItem
	GetCryptoQuotes(symbol string, params GetCryptoQuotesParams) ([]CryptoQuote, error)
	GetCryptoQuotesAsync(symbol string, params GetCryptoQuotesParams) <-chan CryptoQuoteItem
	GetCryptoBars(symbol string, params GetCryptoBarsParams) ([]CryptoBar, error)
	GetCryptoBarsAsync(symbol string, params GetCryptoBarsParams) <-chan CryptoBarItem
	GetCryptoMultiBars(symbols []string, params GetCryptoBarsParams) (map[string][]CryptoBar, error)
	GetCryptoMultiBarsAsync(symbols []string, params GetCryptoBarsParams) <-chan CryptoMultiBarItem
	GetLatestCryptoTrade(symbol, exchange string) (*CryptoTrade, error)
	GetLatestCryptoQuote(symbol, exchange string) (*CryptoQuote, error)
	GetLatestCryptoXBBO(symbol string, exchanges []string) (*CryptoXBBO, error)
}

// ClientOpts contains options for the alpaca marketdata client.
//
// Currently it contains the exact same options as the trading alpaca client,
// but there is no guarantee that this will remain the case.
type ClientOpts struct {
	ApiKey     string
	ApiSecret  string
	OAuth      string
	BaseURL    string
	Timeout    time.Duration
	RetryLimit int
	RetryDelay time.Duration
	Feed       string
}

type client struct {
	opts ClientOpts

	do func(c *client, req *http.Request) (*http.Response, error)
}

// NewClient creates a new marketdata client using the given opts.
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
		if s := os.Getenv("APCA_API_DATA_URL"); s != "" {
			opts.BaseURL = s
		} else {
			opts.BaseURL = "https://data.alpaca.markets"
		}
	}
	if opts.RetryLimit == 0 {
		opts.RetryLimit = 10
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

func setBaseQuery(q url.Values, start, end time.Time, feed, defaultFeed string) {
	if !start.IsZero() {
		q.Set("start", start.Format(time.RFC3339))
	}
	if !end.IsZero() {
		q.Set("end", end.Format(time.RFC3339))
	}
	if feed != "" {
		q.Set("feed", feed)
	} else {
		if defaultFeed != "" {
			q.Set("feed", feed)
		}
	}
}

func setCryptoBaseQuery(q url.Values, start, end time.Time, exchanges []string) {
	if !start.IsZero() {
		q.Set("start", start.Format(time.RFC3339))
	}
	if !end.IsZero() {
		q.Set("end", end.Format(time.RFC3339))
	}
	if len(exchanges) > 0 {
		q.Set("exchanges", strings.Join(exchanges, ","))
	}
}

const v2MaxLimit = 10000

func setQueryLimit(q url.Values, totalLimit int, pageLimit int, received int) {
	limit := 0 // use server side default if unset
	if pageLimit != 0 {
		limit = pageLimit
	}
	if totalLimit != 0 {
		remaining := totalLimit - received
		if remaining <= 0 { // this should never happen
			return
		}
		if (limit == 0 || limit > remaining) && remaining <= v2MaxLimit {
			limit = remaining
		}
	}

	if limit != 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
}

// GetTradesParams contains optional parameters for getting trades.
type GetTradesParams struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Feed is the source of the data: sip or iex.
	Feed string
}

// GetTrades returns the trades for the given symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetTradesAsync instead!
func (c *client) GetTrades(symbol string, params GetTradesParams) ([]Trade, error) {
	trades := make([]Trade, 0)
	for item := range c.GetTradesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		trades = append(trades, item.Trade)
	}
	return trades, nil
}

// GetTradesAsync returns a channel that will be populated with the historical trades for the given symbol.
func (c *client) GetTradesAsync(symbol string, params GetTradesParams) <-chan TradeItem {
	ch := make(chan TradeItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/trades", c.opts.BaseURL, symbol))
		if err != nil {
			ch <- TradeItem{Error: err}
			return
		}

		q := u.Query()
		setBaseQuery(q, params.Start, params.End, params.Feed, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- TradeItem{Error: err}
				return
			}

			var tradeResp tradeResponse
			if err = unmarshal(resp, &tradeResp); err != nil {
				ch <- TradeItem{Error: err}
				return
			}

			for _, trade := range tradeResp.Trades {
				ch <- TradeItem{Trade: trade}
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

// GetMultiTrades returns trades for the given symbols.
func (c *client) GetMultiTrades(
	symbols []string, params GetTradesParams,
) (map[string][]Trade, error) {
	trades := make(map[string][]Trade, len(symbols))
	for item := range c.GetMultiTradesAsync(symbols, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		trades[item.Symbol] = append(trades[item.Symbol], item.Trade)
	}
	return trades, nil
}

// GetTrades returns a channel that will be populated with the trades for the requested symbols.
func (c *client) GetMultiTradesAsync(symbols []string, params GetTradesParams) <-chan MultiTradeItem {
	ch := make(chan MultiTradeItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/trades", c.opts.BaseURL))
		if err != nil {
			ch <- MultiTradeItem{Error: err}
			return
		}

		q := u.Query()
		q.Set("symbols", strings.Join(symbols, ","))
		setBaseQuery(q, params.Start, params.End, params.Feed, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- MultiTradeItem{Error: err}
				return
			}

			var tradeResp multiTradeResponse
			if err = unmarshal(resp, &tradeResp); err != nil {
				ch <- MultiTradeItem{Error: err}
				return
			}

			sortedSymbols := make([]string, 0, len(tradeResp.Trades))
			for symbol := range tradeResp.Trades {
				sortedSymbols = append(sortedSymbols, symbol)
			}
			sort.Strings(sortedSymbols)

			for _, symbol := range sortedSymbols {
				trades := tradeResp.Trades[symbol]
				for _, trade := range trades {
					ch <- MultiTradeItem{Symbol: symbol, Trade: trade}
				}
				received += len(trades)
			}
			if tradeResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *tradeResp.NextPageToken)
		}
	}()

	return ch
}

// GetQuotesParams contains optional parameters for getting quotes
type GetQuotesParams struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned quotes.
	// If missing, all quotes between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Feed is the source of the data: sip or iex.
	Feed string
}

// GetQuotes returns the quotes for the given symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetQuotesAsync instead!
func (c *client) GetQuotes(symbol string, params GetQuotesParams) ([]Quote, error) {
	quotes := make([]Quote, 0)
	for item := range c.GetQuotesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		quotes = append(quotes, item.Quote)
	}
	return quotes, nil
}

// GetQuotesAsync returns a channel that will be populated with the quotes for the given symbol.
func (c *client) GetQuotesAsync(symbol string, params GetQuotesParams) <-chan QuoteItem {
	// NOTE: this method is very similar to GetTrades.
	// With generics it would be almost trivial to refactor them to use a common c.opts.BaseURL method,
	// but without them it doesn't seem to be worth it
	ch := make(chan QuoteItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/quotes", c.opts.BaseURL, symbol))
		if err != nil {
			ch <- QuoteItem{Error: err}
			return
		}

		q := u.Query()
		setBaseQuery(q, params.Start, params.End, params.Feed, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- QuoteItem{Error: err}
				return
			}

			var quoteResp quoteResponse
			if err = unmarshal(resp, &quoteResp); err != nil {
				ch <- QuoteItem{Error: err}
				return
			}

			for _, quote := range quoteResp.Quotes {
				ch <- QuoteItem{Quote: quote}
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

// GetMultiQuotes returns quotes for the given symbols.
func (c *client) GetMultiQuotes(
	symbols []string, params GetQuotesParams,
) (map[string][]Quote, error) {
	quotes := make(map[string][]Quote, len(symbols))
	for item := range c.GetMultiQuotesAsync(symbols, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		quotes[item.Symbol] = append(quotes[item.Symbol], item.Quote)
	}
	return quotes, nil
}

// GetQuotes returns a channel that will be populated with the quotes for the requested symbols.
func (c *client) GetMultiQuotesAsync(symbols []string, params GetQuotesParams) <-chan MultiQuoteItem {
	ch := make(chan MultiQuoteItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/quotes", c.opts.BaseURL))
		if err != nil {
			ch <- MultiQuoteItem{Error: err}
			return
		}

		q := u.Query()
		q.Set("symbols", strings.Join(symbols, ","))
		setBaseQuery(q, params.Start, params.End, params.Feed, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- MultiQuoteItem{Error: err}
				return
			}

			var quoteResp multiQuoteResponse
			if err = unmarshal(resp, &quoteResp); err != nil {
				ch <- MultiQuoteItem{Error: err}
				return
			}

			sortedSymbols := make([]string, 0, len(quoteResp.Quotes))
			for symbol := range quoteResp.Quotes {
				sortedSymbols = append(sortedSymbols, symbol)
			}
			sort.Strings(sortedSymbols)

			for _, symbol := range sortedSymbols {
				quotes := quoteResp.Quotes[symbol]
				for _, quote := range quotes {
					ch <- MultiQuoteItem{Symbol: symbol, Quote: quote}
				}
				received += len(quotes)
			}
			if quoteResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *quoteResp.NextPageToken)
		}
	}()

	return ch
}

// GetBarsParams contains optional parameters for getting bars
type GetBarsParams struct {
	// TimeFrame is the aggregation size of the bars
	TimeFrame TimeFrame
	// Adjustment tells if the bars should be adjusted for corporate actions
	Adjustment Adjustment
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned bars.
	// If missing, all bars between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Feed is the source of the data: sip or iex.
	// If provided, it overrides the client's Feed option.
	Feed string
}

func setQueryBarParams(q url.Values, params GetBarsParams, feed string) {
	setBaseQuery(q, params.Start, params.End, params.Feed, feed)
	adjustment := Raw
	if params.Adjustment != "" {
		adjustment = params.Adjustment
	}
	q.Set("adjustment", string(adjustment))
	timeframe := OneDay
	if params.TimeFrame.N != 0 {
		timeframe = params.TimeFrame
	}
	q.Set("timeframe", timeframe.String())
}

// GetBars returns a slice of bars for the given symbol.
func (c *client) GetBars(symbol string, params GetBarsParams) ([]Bar, error) {
	bars := make([]Bar, 0)
	for item := range c.GetBarsAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars = append(bars, item.Bar)
	}
	return bars, nil
}

// GetBarsAsync returns a channel that will be populated with the bars for the given symbol.
func (c *client) GetBarsAsync(symbol string, params GetBarsParams) <-chan BarItem {
	ch := make(chan BarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/bars", c.opts.BaseURL, symbol))
		if err != nil {
			ch <- BarItem{Error: err}
			return
		}

		q := u.Query()
		setQueryBarParams(q, params, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- BarItem{Error: err}
				return
			}

			var barResp barResponse
			if err = unmarshal(resp, &barResp); err != nil {
				ch <- BarItem{Error: err}
				return
			}

			for _, bar := range barResp.Bars {
				ch <- BarItem{Bar: bar}
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

// GetMultiBars returns bars for the given symbols.
func (c *client) GetMultiBars(
	symbols []string, params GetBarsParams,
) (map[string][]Bar, error) {
	bars := make(map[string][]Bar, len(symbols))
	for item := range c.GetMultiBarsAsync(symbols, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars[item.Symbol] = append(bars[item.Symbol], item.Bar)
	}
	return bars, nil
}

// GetMultiBarsAsync returns a channel that will be populated with the bars for the requested symbols.
func (c *client) GetMultiBarsAsync(symbols []string, params GetBarsParams) <-chan MultiBarItem {
	ch := make(chan MultiBarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/bars", c.opts.BaseURL))
		if err != nil {
			ch <- MultiBarItem{Error: err}
			return
		}

		q := u.Query()
		q.Set("symbols", strings.Join(symbols, ","))
		setQueryBarParams(q, params, c.opts.Feed)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- MultiBarItem{Error: err}
				return
			}

			var barResp multiBarResponse
			if err = unmarshal(resp, &barResp); err != nil {
				ch <- MultiBarItem{Error: err}
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
					ch <- MultiBarItem{Symbol: symbol, Bar: bar}
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

func setLatestQueryParams(u *url.URL, feed string, symbols []string) {
	q := u.Query()
	if len(symbols) > 0 {
		q.Set("symbols", strings.Join(symbols, ","))
	}
	if feed != "" {
		q.Set("feed", feed)
	}
	u.RawQuery = q.Encode()
}

// GetLatestBar returns the latest minute bar for a given symbol
func (c *client) GetLatestBar(symbol string) (*Bar, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/bars/latest", c.opts.BaseURL, symbol))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, nil)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestBarResp latestBarResponse
	if err = unmarshal(resp, &latestBarResp); err != nil {
		return nil, err
	}
	return &latestBarResp.Bar, nil
}

// GetLatestBars returns the latest minute bars for the given symbols
func (c *client) GetLatestBars(symbols []string) (map[string]Bar, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/bars/latest", c.opts.BaseURL))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, symbols)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestBarsResp latestBarsResponse
	if err = unmarshal(resp, &latestBarsResp); err != nil {
		return nil, err
	}
	return latestBarsResp.Bars, nil
}

// GetLatestTrade returns the latest trade for a given symbol
func (c *client) GetLatestTrade(symbol string) (*Trade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/trades/latest", c.opts.BaseURL, symbol))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, nil)

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

// GetLatestTrades returns the latest trades for the given symbols
func (c *client) GetLatestTrades(symbols []string) (map[string]Trade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/trades/latest", c.opts.BaseURL))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, symbols)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestTradesResp latestTradesResponse
	if err = unmarshal(resp, &latestTradesResp); err != nil {
		return nil, err
	}
	return latestTradesResp.Trades, nil
}

// GetLatestQuote returns the latest quote for a given symbol
func (c *client) GetLatestQuote(symbol string) (*Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/quotes/latest", c.opts.BaseURL, symbol))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, nil)

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

// GetLatestQuotes returns the latest quotes for the given symbols
func (c *client) GetLatestQuotes(symbols []string) (map[string]Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/quotes/latest", c.opts.BaseURL))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, symbols)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestQuotesResp latestQuotesResponse
	if err = unmarshal(resp, &latestQuotesResp); err != nil {
		return nil, err
	}
	return latestQuotesResp.Quotes, nil
}

// GetSnapshot returns the snapshot for a given symbol
func (c *client) GetSnapshot(symbol string) (*Snapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/%s/snapshot", c.opts.BaseURL, symbol))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, nil)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshot Snapshot

	if err = unmarshal(resp, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetSnapshots returns the snapshots for multiple symbol
func (c *client) GetSnapshots(symbols []string) (map[string]*Snapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/v2/stocks/snapshots", c.opts.BaseURL))
	if err != nil {
		return nil, err
	}
	setLatestQueryParams(u, c.opts.Feed, symbols)

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshots map[string]*Snapshot

	if err = unmarshal(resp, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

const cryptoPrefix = "v1beta1/crypto"

// GetCryptoTradesParams contains optional parameters for getting crypto trades
type GetCryptoTradesParams struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Exchanges is the list of exchanges to query. If empty, the default exchanges will be used.
	Exchanges []string
}

// GetCryptoTrades returns the trades for the given crypto symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetCryptoTradesAsync instead!
func (c *client) GetCryptoTrades(symbol string, params GetCryptoTradesParams) ([]CryptoTrade, error) {
	trades := make([]CryptoTrade, 0)
	for item := range c.GetCryptoTradesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		trades = append(trades, item.Trade)
	}
	return trades, nil
}

// GetCryptoTradesAsync returns a channel that will be populated with the trades for the given crypto symbol.
func (c *client) GetCryptoTradesAsync(symbol string, params GetCryptoTradesParams) <-chan CryptoTradeItem {
	ch := make(chan CryptoTradeItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/%s/%s/trades", c.opts.BaseURL, cryptoPrefix, symbol))
		if err != nil {
			ch <- CryptoTradeItem{Error: err}
			return
		}

		q := u.Query()
		setCryptoBaseQuery(q, params.Start, params.End, params.Exchanges)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- CryptoTradeItem{Error: err}
				return
			}

			var tradeResp cryptoTradeResponse
			if err = unmarshal(resp, &tradeResp); err != nil {
				ch <- CryptoTradeItem{Error: err}
				return
			}

			for _, trade := range tradeResp.Trades {
				ch <- CryptoTradeItem{Trade: trade}
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

// GetCryptoQuotesParams contains optional parameters for getting crypto quotes
type GetCryptoQuotesParams struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned quotes.
	// If missing, all quotes between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Exchanges is the list of exchanges to query. If empty, the default exchanges will be used.
	Exchanges []string
}

// GetCryptoQuotes returns the quotes for the given crypto symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetCryptoQuotesAsync instead!
func (c *client) GetCryptoQuotes(symbol string, params GetCryptoQuotesParams) ([]CryptoQuote, error) {
	quotes := make([]CryptoQuote, 0)
	for item := range c.GetCryptoQuotesAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		quotes = append(quotes, item.Quote)
	}
	return quotes, nil
}

// GetCryptoQuotesAsync returns a channel that will be populated with the quotes for the given crypto symbol.
func (c *client) GetCryptoQuotesAsync(symbol string, params GetCryptoQuotesParams) <-chan CryptoQuoteItem {
	ch := make(chan CryptoQuoteItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/%s/%s/quotes", c.opts.BaseURL, cryptoPrefix, symbol))
		if err != nil {
			ch <- CryptoQuoteItem{Error: err}
			return
		}

		q := u.Query()
		setCryptoBaseQuery(q, params.Start, params.End, params.Exchanges)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- CryptoQuoteItem{Error: err}
				return
			}

			var quoteResp cryptoQuoteResponse
			if err = unmarshal(resp, &quoteResp); err != nil {
				ch <- CryptoQuoteItem{Error: err}
				return
			}

			for _, quote := range quoteResp.Quotes {
				ch <- CryptoQuoteItem{Quote: quote}
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

// GetCryptoBarsParams contains optional parameters for getting crypto bars
type GetCryptoBarsParams struct {
	// TimeFrame is the aggregation size of the bars
	TimeFrame TimeFrame
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned bars.
	// If missing, all bars between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Exchanges is the list of exchanges to query. If empty, the default exchanges will be used.
	Exchanges []string
}

func setQueryCryptoBarParams(q url.Values, params GetCryptoBarsParams) {
	setCryptoBaseQuery(q, params.Start, params.End, params.Exchanges)
	timeframe := OneDay
	if params.TimeFrame.N != 0 {
		timeframe = params.TimeFrame
	}
	q.Set("timeframe", timeframe.String())
}

// GetCryptoBars returns a slice of bars for the given crypto symbol.
func (c *client) GetCryptoBars(symbol string, params GetCryptoBarsParams) ([]CryptoBar, error) {
	bars := make([]CryptoBar, 0)
	for item := range c.GetCryptoBarsAsync(symbol, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars = append(bars, item.Bar)
	}
	return bars, nil
}

// GetCryptoBarsAsync returns a channel that will be populated with the bars for the given crypto symbol.
func (c *client) GetCryptoBarsAsync(symbol string, params GetCryptoBarsParams) <-chan CryptoBarItem {
	ch := make(chan CryptoBarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/%s/%s/bars", c.opts.BaseURL, cryptoPrefix, symbol))
		if err != nil {
			ch <- CryptoBarItem{Error: err}
			return
		}

		q := u.Query()
		setQueryCryptoBarParams(q, params)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- CryptoBarItem{Error: err}
				return
			}

			var cryptoBarResp cryptoBarResponse
			if err = unmarshal(resp, &cryptoBarResp); err != nil {
				ch <- CryptoBarItem{Error: err}
				return
			}

			for _, bar := range cryptoBarResp.Bars {
				ch <- CryptoBarItem{Bar: bar}
			}
			if cryptoBarResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *cryptoBarResp.NextPageToken)
			received += len(cryptoBarResp.Bars)
		}
	}()

	return ch
}

// GetCryptoMultiBars returns bars for the given crypto symbols.
func (c *client) GetCryptoMultiBars(
	symbols []string, params GetCryptoBarsParams,
) (map[string][]CryptoBar, error) {
	bars := make(map[string][]CryptoBar, len(symbols))
	for item := range c.GetCryptoMultiBarsAsync(symbols, params) {
		if err := item.Error; err != nil {
			return nil, err
		}
		bars[item.Symbol] = append(bars[item.Symbol], item.Bar)
	}
	return bars, nil
}

// GetCryptoMultiBarsAsync returns a channel that will be populated with the bars for the requested crypto symbols.
func (c *client) GetCryptoMultiBarsAsync(symbols []string, params GetCryptoBarsParams) <-chan CryptoMultiBarItem {
	ch := make(chan CryptoMultiBarItem)

	go func() {
		defer close(ch)

		u, err := url.Parse(fmt.Sprintf("%s/%s/bars", c.opts.BaseURL, cryptoPrefix))
		if err != nil {
			ch <- CryptoMultiBarItem{Error: err}
			return
		}

		q := u.Query()
		q.Set("symbols", strings.Join(symbols, ","))
		setQueryCryptoBarParams(q, params)

		received := 0
		for params.TotalLimit == 0 || received < params.TotalLimit {
			setQueryLimit(q, params.TotalLimit, params.PageLimit, received)
			u.RawQuery = q.Encode()

			resp, err := c.get(u)
			if err != nil {
				ch <- CryptoMultiBarItem{Error: err}
				return
			}

			var multiBarResp cryptoMultiBarResponse
			if err = unmarshal(resp, &multiBarResp); err != nil {
				ch <- CryptoMultiBarItem{Error: err}
				return
			}

			sortedSymbols := make([]string, 0, len(multiBarResp.Bars))
			for symbol := range multiBarResp.Bars {
				sortedSymbols = append(sortedSymbols, symbol)
			}
			sort.Strings(sortedSymbols)

			for _, symbol := range sortedSymbols {
				bars := multiBarResp.Bars[symbol]
				for _, bar := range bars {
					ch <- CryptoMultiBarItem{Symbol: symbol, Bar: bar}
				}
				received += len(bars)
			}
			if multiBarResp.NextPageToken == nil {
				return
			}
			q.Set("page_token", *multiBarResp.NextPageToken)
		}
	}()

	return ch
}

// GetLatestCryptoTrade returns the latest trade for a given crypto symbol on the given exchange
func (c *client) GetLatestCryptoTrade(symbol, exchange string) (*CryptoTrade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/trades/latest", c.opts.BaseURL, cryptoPrefix, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("exchange", exchange)
	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestTradeResp latestCryptoTradeResponse

	if err = unmarshal(resp, &latestTradeResp); err != nil {
		return nil, err
	}

	return &latestTradeResp.Trade, nil
}

// GetLatestCryptoQuote returns the latest quote for a given crypto symbol on the given exhange
func (c *client) GetLatestCryptoQuote(symbol, exchange string) (*CryptoQuote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/quotes/latest", c.opts.BaseURL, cryptoPrefix, symbol))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("exchange", exchange)
	u.RawQuery = q.Encode()

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestQuoteResp latestCryptoQuoteResponse

	if err = unmarshal(resp, &latestQuoteResp); err != nil {
		return nil, err
	}

	return &latestQuoteResp.Quote, nil
}

// GetLatestCryptoXBBO returns the latest cross exchange BBO for a given crypto symbol on the given exhange
func (c *client) GetLatestCryptoXBBO(symbol string, exchanges []string) (*CryptoXBBO, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/xbbo/latest", c.opts.BaseURL, cryptoPrefix, symbol))
	if err != nil {
		return nil, err
	}

	if len(exchanges) > 0 {
		q := u.Query()
		q.Set("exchanges", strings.Join(exchanges, ","))
		u.RawQuery = q.Encode()
	}

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestXBBOResp latestCryptoXBBOResponse

	if err = unmarshal(resp, &latestXBBOResp); err != nil {
		return nil, err
	}

	return &latestXBBOResp.XBBO, nil
}

// GetTrades returns the trades for the given symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetTradesAsync instead!
func GetTrades(symbol string, params GetTradesParams) ([]Trade, error) {
	return DefaultClient.GetTrades(symbol, params)
}

// GetTradesAsync returns a channel that will be populated with the trades for the given symbol
// that happened between the given start and end times, limited to the given limit.
func GetTradesAsync(symbol string, params GetTradesParams) <-chan TradeItem {
	return DefaultClient.GetTradesAsync(symbol, params)
}

// GetMultiTrades returns the trades for the given symbols. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetMultiTradesAsync instead!
func GetMultiTrades(symbols []string, params GetTradesParams) (map[string][]Trade, error) {
	return DefaultClient.GetMultiTrades(symbols, params)
}

// GetMultiTradesAsync returns a channel that will be populated with the trades for the given symbols.
func GetMultiTradesAsync(symbols []string, params GetTradesParams) <-chan MultiTradeItem {
	return DefaultClient.GetMultiTradesAsync(symbols, params)
}

// GetQuotes returns the quotes for the given symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetQuotesAsync instead!
func GetQuotes(symbol string, params GetQuotesParams) ([]Quote, error) {
	return DefaultClient.GetQuotes(symbol, params)
}

// GetQuotesAsync returns a channel that will be populated with the quotes for the given symbol
// that happened between the given start and end times, limited to the given limit.
func GetQuotesAsync(symbol string, params GetQuotesParams) <-chan QuoteItem {
	return DefaultClient.GetQuotesAsync(symbol, params)
}

// GetMultiQuotes returns the quotes for the given symbols. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetMultiQuotesAsync instead!
func GetMultiQuotes(symbols []string, params GetQuotesParams) (map[string][]Quote, error) {
	return DefaultClient.GetMultiQuotes(symbols, params)
}

// GetMultiQuotesAsync returns a channel that will be populated with the quotes for the given symbols.
func GetMultiQuotesAsync(symbols []string, params GetQuotesParams) <-chan MultiQuoteItem {
	return DefaultClient.GetMultiQuotesAsync(symbols, params)
}

// GetBars returns the bars for the given symbol. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetBarsAsync instead!
func GetBars(symbol string, params GetBarsParams) ([]Bar, error) {
	return DefaultClient.GetBars(symbol, params)
}

// GetBarsAsync returns a channel that will be populated with the bars for the given symbol.
func GetBarsAsync(symbol string, params GetBarsParams) <-chan BarItem {
	return DefaultClient.GetBarsAsync(symbol, params)
}

// GetMultiBars returns the bars for the given symbols. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetMultiBarsAsync instead!
func GetMultiBars(symbols []string, params GetBarsParams) (map[string][]Bar, error) {
	return DefaultClient.GetMultiBars(symbols, params)
}

// GetMultiBarsAsync returns a channel that will be populated with the bars for the given symbols.
func GetMultiBarsAsync(symbols []string, params GetBarsParams) <-chan MultiBarItem {
	return DefaultClient.GetMultiBarsAsync(symbols, params)
}

// GetLatestBar returns the latest minute bar for a given symbol.
func GetLatestBar(symbol string) (*Bar, error) {
	return DefaultClient.GetLatestBar(symbol)
}

// GetLatestBars returns the latest minute bars for the given symbols.
func GetLatestBars(symbols []string) (map[string]Bar, error) {
	return DefaultClient.GetLatestBars(symbols)
}

// GetLatestTrade returns the latest trade for a given symbol.
func GetLatestTrade(symbol string) (*Trade, error) {
	return DefaultClient.GetLatestTrade(symbol)
}

// GetLatestTrades returns the latest trades for the given symbols.
func GetLatestTrades(symbols []string) (map[string]Trade, error) {
	return DefaultClient.GetLatestTrades(symbols)
}

// GetLatestQuote returns the latest quote for a given symbol.
func GetLatestQuote(symbol string) (*Quote, error) {
	return DefaultClient.GetLatestQuote(symbol)
}

// GetLatestQuotes returns the latest quotes for the given symbols.
func GetLatestQuotes(symbols []string) (map[string]Quote, error) {
	return DefaultClient.GetLatestQuotes(symbols)
}

// GetSnapshot returns the snapshot for a given symbol
func GetSnapshot(symbol string) (*Snapshot, error) {
	return DefaultClient.GetSnapshot(symbol)
}

// GetSnapshots returns the snapshots for a multiple symbols
func GetSnapshots(symbols []string) (map[string]*Snapshot, error) {
	return DefaultClient.GetSnapshots(symbols)
}

// GetCryptoTrades returns the trades for the given crypto symbol. It blocks until all the trades are collected.
// If you want to process the incoming trades instantly, use GetCryptoTradesAsync instead!
func GetCryptoTrades(symbol string, params GetCryptoTradesParams) ([]CryptoTrade, error) {
	return DefaultClient.GetCryptoTrades(symbol, params)
}

// GetCryptoQuotesAsync returns a channel that will be populated with the trades for the given crypto symbol
// that happened between the given start and end times, limited to the given limit.
func GetCryptoTradesAsync(symbol string, params GetCryptoTradesParams) <-chan CryptoTradeItem {
	return DefaultClient.GetCryptoTradesAsync(symbol, params)
}

// GetCryptoQuotes returns the quotes for the given crypto symbol. It blocks until all the quotes are collected.
// If you want to process the incoming quotes instantly, use GetCryptoQuotesAsync instead!
func GetCryptoQuotes(symbol string, params GetCryptoQuotesParams) ([]CryptoQuote, error) {
	return DefaultClient.GetCryptoQuotes(symbol, params)
}

// GetCryptoQuotesAsync returns a channel that will be populated with the quotes for the given crypto symbol
// that happened between the given start and end times, limited to the given limit.
func GetCryptoQuotesAsync(symbol string, params GetCryptoQuotesParams) <-chan CryptoQuoteItem {
	return DefaultClient.GetCryptoQuotesAsync(symbol, params)
}

// GetCryptoBars returns the bars for the given crypto symbol. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetCryptoBarsAsync instead!
func GetCryptoBars(symbol string, params GetCryptoBarsParams) ([]CryptoBar, error) {
	return DefaultClient.GetCryptoBars(symbol, params)
}

// GetCryptoBarsAsync returns a channel that will be populated with the bars for the given crypto symbol.
func GetCryptoBarsAsync(symbol string, params GetCryptoBarsParams) <-chan CryptoBarItem {
	return DefaultClient.GetCryptoBarsAsync(symbol, params)
}

// GetCryptoMultiBars returns the bars for the given crypto symbols. It blocks until all the bars are collected.
// If you want to process the incoming bars instantly, use GetCryptoMultiBarsAsync instead!
func GetCryptoMultiBars(symbols []string, params GetCryptoBarsParams) (map[string][]CryptoBar, error) {
	return DefaultClient.GetCryptoMultiBars(symbols, params)
}

// GetCryptoMultiBarsAsync returns a channel that will be populated with the bars for the given crypto symbols.
func GetCryptoMultiBarsAsync(symbols []string, params GetCryptoBarsParams) <-chan CryptoMultiBarItem {
	return DefaultClient.GetCryptoMultiBarsAsync(symbols, params)
}

// GetLatestCryptoTrade returns the latest trade for a given crypto symbol on the given exchange
func GetLatestCryptoTrade(symbol, exchange string) (*CryptoTrade, error) {
	return DefaultClient.GetLatestCryptoTrade(symbol, exchange)
}

// GetLatestCryptoQuote returns the latest quote for a given crypto symbol on the given exhange
func GetLatestCryptoQuote(symbol, exchange string) (*CryptoQuote, error) {
	return DefaultClient.GetLatestCryptoQuote(symbol, exchange)
}

// GetLatestCryptoXBBO returns the latest quote for a given crypto symbol on the given exhange
func GetLatestCryptoXBBO(symbol string, exchanges []string) (*CryptoXBBO, error) {
	return DefaultClient.GetLatestCryptoXBBO(symbol, exchanges)
}

func (c *client) get(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")

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
	var (
		reader io.ReadCloser
		err    error
	)
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}
	return json.NewDecoder(reader).Decode(data)
}
