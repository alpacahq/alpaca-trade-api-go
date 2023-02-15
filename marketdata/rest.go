package marketdata

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mailru/easyjson"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
)

// ClientOpts contains options for the alpaca marketdata client.
//
// Currently it contains the exact same options as the trading alpaca client,
// but there is no guarantee that this will remain the case.
type ClientOpts struct {
	APIKey     string
	APISecret  string
	OAuth      string
	BaseURL    string
	RetryLimit int
	RetryDelay time.Duration
	// Feed is the default feed to be used by all requests. Can be overridden per request.
	Feed Feed
	// CryptoFeed is the default crypto feed to be used by all requests. Can be overridden per request.
	CryptoFeed CryptoFeed
	// Currency is the default currency to be used by all requests. Can be overridden per request.
	// For the latest endpoints this is the only way to set this parameter.
	Currency string
	// HTTPClient to be used for each http request.
	HTTPClient *http.Client
}

// Client is the alpaca marketdata Client.
type Client struct {
	opts       ClientOpts
	httpClient *http.Client

	do func(c *Client, req *http.Request) (*http.Response, error)
}

// NewClient creates a new marketdata client using the given opts.
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

func defaultDo(c *Client, req *http.Request) (*http.Response, error) {
	if c.opts.OAuth != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	} else {
		req.Header.Set("APCA-API-KEY-ID", c.opts.APIKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.opts.APISecret)
	}

	var resp *http.Response
	var err error

RetryLoop:
	for i := 0; ; i++ {
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusInternalServerError:
		default:
			break RetryLoop
		}
		if i >= c.opts.RetryLimit {
			break
		}
		time.Sleep(c.opts.RetryDelay)
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, alpaca.APIErrorFromResponse(resp)
	}

	return resp, nil
}

type baseRequest struct {
	Symbols  []string
	Start    time.Time
	End      time.Time
	Feed     Feed
	AsOf     string
	Currency string
}

func (c *Client) setBaseQuery(q url.Values, req baseRequest) {
	q.Set("symbols", strings.Join(req.Symbols, ","))
	if !req.Start.IsZero() {
		q.Set("start", req.Start.Format(time.RFC3339Nano))
	}
	if !req.End.IsZero() {
		q.Set("end", req.End.Format(time.RFC3339Nano))
	}
	if req.Feed != "" {
		q.Set("feed", req.Feed)
	} else if c.opts.Feed != "" {
		q.Set("feed", c.opts.Feed)
	}
	if req.AsOf != "" {
		q.Set("asof", req.AsOf)
	}
	if req.Currency != "" {
		q.Set("currency", req.Currency)
	} else if c.opts.Currency != "" {
		q.Set("currency", c.opts.Currency)
	}
}

const (
	v2MaxLimit   = 10000
	newsMaxLimit = 50
	stockPrefix  = "v2/stocks"
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
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
}

// GetTradesRequest contains optional parameters for getting trades.
type GetTradesRequest struct {
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
	Feed Feed
	// AsOf defines the date when the symbols are mapped. "-" means no mapping.
	AsOf string
	// Currency is the currency of the displayed prices
	Currency string
}

// GetTrades returns the trades for the given symbol.
func (c *Client) GetTrades(symbol string, req GetTradesRequest) ([]Trade, error) {
	resp, err := c.GetMultiTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiTrades returns trades for the given symbols.
func (c *Client) GetMultiTrades(symbols []string, req GetTradesRequest) (map[string][]Trade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/trades", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols:  symbols,
		Start:    req.Start,
		End:      req.End,
		Feed:     req.Feed,
		AsOf:     req.AsOf,
		Currency: req.Currency,
	})

	trades := make(map[string][]Trade, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var tradeResp multiTradeResponse
		if err = unmarshal(resp, &tradeResp); err != nil {
			return nil, err
		}

		for symbol, t := range tradeResp.Trades {
			trades[symbol] = append(trades[symbol], t...)
			received += len(t)
		}
		if tradeResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *tradeResp.NextPageToken)
	}
	return trades, nil
}

// GetQuotesRequest contains optional parameters for getting quotes
type GetQuotesRequest struct {
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
	Feed Feed
	// AsOf defines the date when the symbols are mapped. "-" means no mapping.
	AsOf string
	// Currency is the currency of the displayed prices
	Currency string
}

// GetQuotes returns the quotes for the given symbol.
func (c *Client) GetQuotes(symbol string, req GetQuotesRequest) ([]Quote, error) {
	resp, err := c.GetMultiQuotes([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiQuotes returns quotes for the given symbols.
func (c *Client) GetMultiQuotes(symbols []string, req GetQuotesRequest) (map[string][]Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/quotes", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols:  symbols,
		Start:    req.Start,
		End:      req.End,
		Feed:     req.Feed,
		AsOf:     req.AsOf,
		Currency: req.Currency,
	})

	quotes := make(map[string][]Quote, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var quoteResp multiQuoteResponse
		if err = unmarshal(resp, &quoteResp); err != nil {
			return nil, err
		}

		for symbol, q := range quoteResp.Quotes {
			quotes[symbol] = append(quotes[symbol], q...)
			received += len(q)
		}
		if quoteResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *quoteResp.NextPageToken)
	}
	return quotes, nil
}

// GetBarsRequest contains optional parameters for getting bars
type GetBarsRequest struct {
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
	Feed Feed
	// AsOf defines the date when the symbols are mapped. "-" means no mapping.
	AsOf string
	// Currency is the currency of the displayed prices
	Currency string
}

func (c *Client) setQueryBarRequest(q url.Values, symbols []string, req GetBarsRequest) {
	c.setBaseQuery(q, baseRequest{
		Symbols:  symbols,
		Start:    req.Start,
		End:      req.End,
		Feed:     req.Feed,
		AsOf:     req.AsOf,
		Currency: req.Currency,
	})
	adjustment := Raw
	if req.Adjustment != "" {
		adjustment = req.Adjustment
	}
	q.Set("adjustment", string(adjustment))
	timeframe := OneDay
	if req.TimeFrame.N != 0 {
		timeframe = req.TimeFrame
	}
	q.Set("timeframe", timeframe.String())
}

// GetBars returns a slice of bars for the given symbol.
func (c *Client) GetBars(symbol string, req GetBarsRequest) ([]Bar, error) {
	resp, err := c.GetMultiBars([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiBars returns bars for the given symbols.
func (c *Client) GetMultiBars(symbols []string, req GetBarsRequest) (map[string][]Bar, error) {
	bars := make(map[string][]Bar, len(symbols))

	u, err := url.Parse(fmt.Sprintf("%s/%s/bars", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setQueryBarRequest(q, symbols, req)

	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var barResp multiBarResponse
		if err = unmarshal(resp, &barResp); err != nil {
			return nil, err
		}

		for symbol, b := range barResp.Bars {
			bars[symbol] = append(bars[symbol], b...)
			received += len(b)
		}
		if barResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *barResp.NextPageToken)
	}
	return bars, nil
}

// GetAuctionsRequest contains optional parameters for getting auctions
type GetAuctionsRequest struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned auctions.
	// If missing, all auctions between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// AsOf defines the date when the symbols are mapped. "-" means no mapping.
	AsOf string
	// Currency is the currency of the displayed prices
	Currency string
}

// GetAuctions returns the auctions for the given symbol.
func (c *Client) GetAuctions(symbol string, req GetAuctionsRequest) ([]DailyAuctions, error) {
	resp, err := c.GetMultiAuctions([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiAuctions returns auctions for the given symbols.
func (c *Client) GetMultiAuctions(
	symbols []string, req GetAuctionsRequest,
) (map[string][]DailyAuctions, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/auctions", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols:  symbols,
		Start:    req.Start,
		End:      req.End,
		Feed:     "sip",
		AsOf:     req.AsOf,
		Currency: req.Currency,
	})

	auctions := make(map[string][]DailyAuctions, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var auctionsResp multiAuctionsResponse
		if err = unmarshal(resp, &auctionsResp); err != nil {
			return nil, err
		}

		for symbol, a := range auctionsResp.Auctions {
			auctions[symbol] = append(auctions[symbol], a...)
			received += len(a)
		}
		if auctionsResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *auctionsResp.NextPageToken)
	}
	return auctions, nil
}

type baseLatestRequest struct {
	Symbols  []string
	Feed     Feed
	Currency string
}

func (c *Client) setLatestQueryRequest(u *url.URL, req baseLatestRequest) {
	q := u.Query()
	if len(req.Symbols) > 0 {
		q.Set("symbols", strings.Join(req.Symbols, ","))
	}
	if req.Feed != "" {
		// The request's feed has precedent over the client's feed
		q.Set("feed", req.Feed)
	} else if c.opts.Feed != "" {
		q.Set("feed", c.opts.Feed)
	}
	if req.Currency != "" {
		q.Set("currency", req.Currency)
	} else if c.opts.Currency != "" {
		q.Set("currency", c.opts.Currency)
	}
	u.RawQuery = q.Encode()
}

type GetLatestBarRequest struct {
	Feed     Feed
	Currency string
}

// GetLatestBar returns the latest minute bar for a given symbol
func (c *Client) GetLatestBar(symbol string, req GetLatestBarRequest) (*Bar, error) {
	resp, err := c.GetLatestBars([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	bar, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &bar, nil
}

// GetLatestBars returns the latest minute bars for the given symbols
func (c *Client) GetLatestBars(symbols []string, req GetLatestBarRequest) (map[string]Bar, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/bars/latest", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols:  symbols,
		Feed:     req.Feed,
		Currency: req.Currency,
	})

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

type GetLatestTradeRequest struct {
	Feed     Feed
	Currency string
}

// GetLatestTrade returns the latest trade for a given symbol
func (c *Client) GetLatestTrade(symbol string, req GetLatestTradeRequest) (*Trade, error) {
	resp, err := c.GetLatestTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	trade, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &trade, nil
}

// GetLatestTrades returns the latest trades for the given symbols
func (c *Client) GetLatestTrades(symbols []string, req GetLatestTradeRequest) (map[string]Trade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/trades/latest", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols:  symbols,
		Feed:     req.Feed,
		Currency: req.Currency,
	})

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

type GetLatestQuoteRequest struct {
	Feed     Feed
	Currency string
}

// GetLatestQuote returns the latest quote for a given symbol
func (c *Client) GetLatestQuote(symbol string, req GetLatestQuoteRequest) (*Quote, error) {
	resp, err := c.GetLatestQuotes([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	quote, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &quote, nil
}

// GetLatestQuotes returns the latest quotes for the given symbols
func (c *Client) GetLatestQuotes(symbols []string, req GetLatestQuoteRequest) (map[string]Quote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/quotes/latest", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols:  symbols,
		Feed:     req.Feed,
		Currency: req.Currency,
	})

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

type GetSnapshotRequest struct {
	Feed     Feed
	Currency string
}

// GetSnapshot returns the snapshot for a given symbol
func (c *Client) GetSnapshot(symbol string, req GetSnapshotRequest) (*Snapshot, error) {
	resp, err := c.GetSnapshots([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetSnapshots returns the snapshots for multiple symbol
func (c *Client) GetSnapshots(symbols []string, req GetSnapshotRequest) (map[string]*Snapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/snapshots", c.opts.BaseURL, stockPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols:  symbols,
		Feed:     req.Feed,
		Currency: req.Currency,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshots snapshotsResponse
	if err = unmarshal(resp, &snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

const cryptoPrefix = "v1beta3/crypto"

func setCryptoBaseQuery(q url.Values, symbols []string, start, end time.Time) {
	q.Set("symbols", strings.Join(symbols, ","))
	if !start.IsZero() {
		q.Set("start", start.Format(time.RFC3339Nano))
	}
	if !end.IsZero() {
		q.Set("end", end.Format(time.RFC3339Nano))
	}
}

// GetCryptoTradesRequest contains optional parameters for getting crypto trades
type GetCryptoTradesRequest struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// CryptoFeed is the crypto feed. Default is "us".
	CryptoFeed CryptoFeed
}

// GetCryptoTrades returns the trades for the given crypto symbol.
func (c *Client) GetCryptoTrades(symbol string, req GetCryptoTradesRequest) ([]CryptoTrade, error) {
	resp, err := c.GetCryptoMultiTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiTrades returns trades for the given crypto symbols.
func (c *Client) GetCryptoMultiTrades(symbols []string, req GetCryptoTradesRequest) (map[string][]CryptoTrade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/trades", c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	setCryptoBaseQuery(q, symbols, req.Start, req.End)

	trades := make(map[string][]CryptoTrade, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var tradeResp cryptoMultiTradeResponse
		if err = unmarshal(resp, &tradeResp); err != nil {
			return nil, err
		}

		for symbol, t := range tradeResp.Trades {
			trades[symbol] = append(trades[symbol], t...)
			received += len(t)
		}
		if tradeResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *tradeResp.NextPageToken)
	}
	return trades, nil
}

// GetCryptoBarsRequest contains optional parameters for getting crypto bars
type GetCryptoBarsRequest struct {
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
	// CryptoFeed is the crypto feed. Default is "us".
	CryptoFeed CryptoFeed
}

func setQueryCryptoBarRequest(q url.Values, symbols []string, req GetCryptoBarsRequest) {
	setCryptoBaseQuery(q, symbols, req.Start, req.End)
	timeframe := OneDay
	if req.TimeFrame.N != 0 {
		timeframe = req.TimeFrame
	}
	q.Set("timeframe", timeframe.String())
}

// GetCryptoBars returns a slice of bars for the given crypto symbol.
func (c *Client) GetCryptoBars(symbol string, req GetCryptoBarsRequest) ([]CryptoBar, error) {
	resp, err := c.GetCryptoMultiBars([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetCryptoMultiBars returns bars for the given crypto symbols.
func (c *Client) GetCryptoMultiBars(symbols []string, req GetCryptoBarsRequest) (map[string][]CryptoBar, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/bars",
		c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	setQueryCryptoBarRequest(q, symbols, req)

	bars := make(map[string][]CryptoBar, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var barResp cryptoMultiBarResponse
		if err = unmarshal(resp, &barResp); err != nil {
			return nil, err
		}

		for symbol, b := range barResp.Bars {
			bars[symbol] = append(bars[symbol], b...)
			received += len(b)
		}
		if barResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *barResp.NextPageToken)
	}
	return bars, nil
}

type cryptoBaseLatestRequest struct {
	Symbols []string
}

func (c *Client) setLatestCryptoQueryRequest(u *url.URL, req cryptoBaseLatestRequest) {
	q := u.Query()
	q.Set("symbols", strings.Join(req.Symbols, ","))
	u.RawQuery = q.Encode()
}

type GetLatestCryptoBarRequest struct {
	CryptoFeed CryptoFeed
}

func (c *Client) cryptoFeed(fromReq string) string {
	if fromReq != "" {
		return fromReq
	}
	if c.opts.CryptoFeed != "" {
		return c.opts.CryptoFeed
	}
	return "us"
}

// GetLatestCryptoBar returns the latest bar for a given crypto symbol
func (c *Client) GetLatestCryptoBar(symbol string, req GetLatestCryptoBarRequest) (*CryptoBar, error) {
	resp, err := c.GetLatestCryptoBars([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	bar, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &bar, nil
}

// GetLatestCryptoBars returns the latest bars for the given crypto symbols
func (c *Client) GetLatestCryptoBars(symbols []string, req GetLatestCryptoBarRequest) (map[string]CryptoBar, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/latest/bars",
		c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}
	c.setLatestCryptoQueryRequest(u, cryptoBaseLatestRequest{
		Symbols: symbols,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestBarsResp latestCryptoBarsResponse
	if err = unmarshal(resp, &latestBarsResp); err != nil {
		return nil, err
	}
	return latestBarsResp.Bars, nil
}

type GetLatestCryptoTradeRequest struct {
	CryptoFeed CryptoFeed
}

// GetLatestCryptoTrade returns the latest trade for a given crypto symbol
func (c *Client) GetLatestCryptoTrade(symbol string, req GetLatestCryptoTradeRequest) (*CryptoTrade, error) {
	resp, err := c.GetLatestCryptoTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	trade, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &trade, nil
}

// GetLatestCryptoTrades returns the latest trades for the given crypto symbols
func (c *Client) GetLatestCryptoTrades(symbols []string, req GetLatestCryptoTradeRequest) (map[string]CryptoTrade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/latest/trades",
		c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}
	c.setLatestCryptoQueryRequest(u, cryptoBaseLatestRequest{
		Symbols: symbols,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestTradesResp latestCryptoTradesResponse
	if err = unmarshal(resp, &latestTradesResp); err != nil {
		return nil, err
	}
	return latestTradesResp.Trades, nil
}

type GetLatestCryptoQuoteRequest struct {
	CryptoFeed CryptoFeed
}

// GetLatestCryptoQuote returns the latest quote for a given crypto symbol
func (c *Client) GetLatestCryptoQuote(symbol string, req GetLatestCryptoQuoteRequest) (*CryptoQuote, error) {
	resp, err := c.GetLatestCryptoQuotes([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	quote, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &quote, nil
}

// GetLatestCryptoQuotes returns the latest quotes for the given crypto symbols
func (c *Client) GetLatestCryptoQuotes(symbols []string, req GetLatestCryptoQuoteRequest) (map[string]CryptoQuote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/latest/quotes",
		c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}
	c.setLatestCryptoQueryRequest(u, cryptoBaseLatestRequest{
		Symbols: symbols,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var latestQuotesResp latestCryptoQuotesResponse
	if err = unmarshal(resp, &latestQuotesResp); err != nil {
		return nil, err
	}
	return latestQuotesResp.Quotes, nil
}

type GetCryptoSnapshotRequest struct {
	CryptoFeed CryptoFeed
}

// GetCryptoSnapshot returns the snapshot for a given crypto symbol
func (c *Client) GetCryptoSnapshot(symbol string, req GetCryptoSnapshotRequest) (*CryptoSnapshot, error) {
	resp, err := c.GetCryptoSnapshots([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	snapshot, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &snapshot, nil
}

// GetCryptoSnapshots returns the snapshots for the given crypto symbols
func (c *Client) GetCryptoSnapshots(symbols []string, req GetCryptoSnapshotRequest) (map[string]CryptoSnapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/%s/snapshots",
		c.opts.BaseURL, cryptoPrefix, c.cryptoFeed(req.CryptoFeed)))
	if err != nil {
		return nil, err
	}
	c.setLatestCryptoQueryRequest(u, cryptoBaseLatestRequest{
		Symbols: symbols,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}

	var snapshots CryptoSnapshots
	if err = unmarshal(resp, &snapshots); err != nil {
		return nil, err
	}
	return snapshots.Snapshots, nil
}

// Sort represents the sort order of the results
type Sort string

// List of sort values
var (
	// SortDesc means the results will be sorted in a descending order
	SortDesc Sort = "desc"
	// SortAsc means the results will be sorted in an ascending order
	SortAsc Sort = "asc"
)

// GetNewsRequest contains optional parameters for getting news articles.
type GetNewsRequest struct {
	// Symbols filters the news to the related symbols.
	// If empty or nil, all articles will be returned.
	Symbols []string
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// Sort sets the sort order of the results. Sorting will be done by the UpdatedAt field.
	Sort Sort
	// IncludeContent tells the server to include the article content in the response.
	IncludeContent bool
	// ExcludeContentless tells the server to exclude articles that have no content.
	ExcludeContentless bool
	// TotalLimit is the limit of the total number of the returned news.
	//
	// If it's non-zero, NoTotalLimit must be false, otherwise an error well be returned.
	// If it's zero then the NoTotalLimit parameter is considered: if NoTotalLimit is true,
	// then all the articles in the given start-end interval are returned.
	// If NoTotalLimit is false, then 50 articles will be returned.
	//
	// The reason for this complication is that the default (empty GetNewsRequest) would
	// not return all the news articles.
	TotalLimit int
	// NoTotalLimit means all news articles will be returned from the given start-end interval.
	//
	// TotalLimit must be set to 0 if NoTotalLimit is true, otherwise an error well be returned.
	NoTotalLimit bool
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
}

func (c *Client) setNewsQuery(q url.Values, p GetNewsRequest) {
	if len(p.Symbols) > 0 {
		q.Set("symbols", strings.Join(p.Symbols, ","))
	}
	if !p.Start.IsZero() {
		q.Set("start", p.Start.Format(time.RFC3339))
	}
	if !p.End.IsZero() {
		q.Set("end", p.End.Format(time.RFC3339))
	}
	if p.Sort != "" {
		q.Set("sort", string(p.Sort))
	}
	if p.IncludeContent {
		q.Set("include_content", strconv.FormatBool(p.IncludeContent))
	}
	if p.ExcludeContentless {
		q.Set("exclude_contentless", strconv.FormatBool(p.ExcludeContentless))
	}
}

// GetNews returns the news articles based on the given req.
func (c *Client) GetNews(req GetNewsRequest) ([]News, error) {
	if req.TotalLimit < 0 {
		return nil, fmt.Errorf("negative total limit")
	}
	if req.PageLimit < 0 {
		return nil, fmt.Errorf("negative page limit")
	}
	if req.NoTotalLimit && req.TotalLimit != 0 {
		return nil, fmt.Errorf("both NoTotalLimit and non-zero TotalLimit specified")
	}
	u, err := url.Parse(fmt.Sprintf("%s/v1beta1/news", c.opts.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("invalid news url: %w", err)
	}

	q := u.Query()
	c.setNewsQuery(q, req)
	received := 0
	totalLimit := req.TotalLimit
	if req.TotalLimit == 0 && !req.NoTotalLimit {
		totalLimit = newsMaxLimit
	}

	news := make([]News, 0, totalLimit)
	for totalLimit == 0 || received < totalLimit {
		setQueryLimit(q, totalLimit, req.PageLimit, received, newsMaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, fmt.Errorf("failed to get news: %w", err)
		}

		var newsResp newsResponse
		if err = unmarshal(resp, &newsResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal news: %w", err)
		}

		news = append(news, newsResp.News...)
		if newsResp.NextPageToken == nil {
			return news, nil
		}
		q.Set("page_token", *newsResp.NextPageToken)
		received += len(newsResp.News)
	}
	return news, nil
}

// GetTrades returns the trades for the given symbol.
func GetTrades(symbol string, req GetTradesRequest) ([]Trade, error) {
	return DefaultClient.GetTrades(symbol, req)
}

// GetMultiTrades returns the trades for the given symbols.
func GetMultiTrades(symbols []string, req GetTradesRequest) (map[string][]Trade, error) {
	return DefaultClient.GetMultiTrades(symbols, req)
}

// GetQuotes returns the quotes for the given symbol.
func GetQuotes(symbol string, req GetQuotesRequest) ([]Quote, error) {
	return DefaultClient.GetQuotes(symbol, req)
}

// GetMultiQuotes returns the quotes for the given symbols.
func GetMultiQuotes(symbols []string, req GetQuotesRequest) (map[string][]Quote, error) {
	return DefaultClient.GetMultiQuotes(symbols, req)
}

// GetBars returns the bars for the given symbol.
func GetBars(symbol string, req GetBarsRequest) ([]Bar, error) {
	return DefaultClient.GetBars(symbol, req)
}

// GetMultiBars returns the bars for the given symbols.
func GetMultiBars(symbols []string, req GetBarsRequest) (map[string][]Bar, error) {
	return DefaultClient.GetMultiBars(symbols, req)
}

// GetAuctions returns the auctions for the given symbol.
func GetAuctions(symbol string, req GetAuctionsRequest) ([]DailyAuctions, error) {
	return DefaultClient.GetAuctions(symbol, req)
}

// GetMultiAuctions returns the auctions for the given symbols.
func GetMultiAuctions(symbols []string, req GetAuctionsRequest) (map[string][]DailyAuctions, error) {
	return DefaultClient.GetMultiAuctions(symbols, req)
}

// GetLatestBar returns the latest minute bar for a given symbol.
func GetLatestBar(symbol string, req GetLatestBarRequest) (*Bar, error) {
	return DefaultClient.GetLatestBar(symbol, req)
}

// GetLatestBars returns the latest minute bars for the given symbols.
func GetLatestBars(symbols []string, req GetLatestBarRequest) (map[string]Bar, error) {
	return DefaultClient.GetLatestBars(symbols, req)
}

// GetLatestTrade returns the latest trade for a given symbol.
func GetLatestTrade(symbol string, req GetLatestTradeRequest) (*Trade, error) {
	return DefaultClient.GetLatestTrade(symbol, req)
}

// GetLatestTrades returns the latest trades for the given symbols.
func GetLatestTrades(symbols []string, req GetLatestTradeRequest) (map[string]Trade, error) {
	return DefaultClient.GetLatestTrades(symbols, req)
}

// GetLatestQuote returns the latest quote for a given symbol.
func GetLatestQuote(symbol string, req GetLatestQuoteRequest) (*Quote, error) {
	return DefaultClient.GetLatestQuote(symbol, req)
}

// GetLatestQuotes returns the latest quotes for the given symbols.
func GetLatestQuotes(symbols []string, req GetLatestQuoteRequest) (map[string]Quote, error) {
	return DefaultClient.GetLatestQuotes(symbols, req)
}

// GetSnapshot returns the snapshot for a given symbol
func GetSnapshot(symbol string, req GetSnapshotRequest) (*Snapshot, error) {
	return DefaultClient.GetSnapshot(symbol, req)
}

// GetSnapshots returns the snapshots for a multiple symbols
func GetSnapshots(symbols []string, req GetSnapshotRequest) (map[string]*Snapshot, error) {
	return DefaultClient.GetSnapshots(symbols, req)
}

// GetCryptoTrades returns the trades for the given crypto symbol.
func GetCryptoTrades(symbol string, req GetCryptoTradesRequest) ([]CryptoTrade, error) {
	return DefaultClient.GetCryptoTrades(symbol, req)
}

// GetCryptoMultiTrades returns trades for the given crypto symbols.
func GetCryptoMultiTrades(symbols []string, req GetCryptoTradesRequest) (map[string][]CryptoTrade, error) {
	return DefaultClient.GetCryptoMultiTrades(symbols, req)
}

// GetCryptoBars returns the bars for the given crypto symbol.
func GetCryptoBars(symbol string, req GetCryptoBarsRequest) ([]CryptoBar, error) {
	return DefaultClient.GetCryptoBars(symbol, req)
}

// GetCryptoMultiBars returns the bars for the given crypto symbols.
func GetCryptoMultiBars(symbols []string, req GetCryptoBarsRequest) (map[string][]CryptoBar, error) {
	return DefaultClient.GetCryptoMultiBars(symbols, req)
}

// GetLatestCryptoBar returns the latest bar for a given crypto symbol
func GetLatestCryptoBar(symbol string, req GetLatestCryptoBarRequest) (*CryptoBar, error) {
	return DefaultClient.GetLatestCryptoBar(symbol, req)
}

// GetLatestCryptoBars returns the latest bars for the given crypto symbols
func GetLatestCryptoBars(symbols []string, req GetLatestCryptoBarRequest) (map[string]CryptoBar, error) {
	return DefaultClient.GetLatestCryptoBars(symbols, req)
}

// GetLatestCryptoTrade returns the latest trade for a given crypto symbol
func GetLatestCryptoTrade(symbol string, req GetLatestCryptoTradeRequest) (*CryptoTrade, error) {
	return DefaultClient.GetLatestCryptoTrade(symbol, req)
}

// GetLatestCryptoTrades returns the latest trades for the given crypto symbols
func GetLatestCryptoTrades(symbols []string, req GetLatestCryptoTradeRequest) (map[string]CryptoTrade, error) {
	return DefaultClient.GetLatestCryptoTrades(symbols, req)
}

// GetLatestCryptoQuote returns the latest quote for a given crypto symbol
func GetLatestCryptoQuote(symbol string, req GetLatestCryptoQuoteRequest) (*CryptoQuote, error) {
	return DefaultClient.GetLatestCryptoQuote(symbol, req)
}

// GetLatestCryptoQuotes returns the latest quotes for the given crypto symbols
func GetLatestCryptoQuotes(symbols []string, req GetLatestCryptoQuoteRequest) (map[string]CryptoQuote, error) {
	return DefaultClient.GetLatestCryptoQuotes(symbols, req)
}

// GetCryptoSnapshot returns the snapshot for a given crypto symbol
func GetCryptoSnapshot(symbol string, req GetCryptoSnapshotRequest) (*CryptoSnapshot, error) {
	return DefaultClient.GetCryptoSnapshot(symbol, req)
}

// GetCryptoSnapshots returns the snapshots for the given crypto symbols
func GetCryptoSnapshots(symbols []string, req GetCryptoSnapshotRequest) (map[string]CryptoSnapshot, error) {
	return DefaultClient.GetCryptoSnapshots(symbols, req)
}

// GetNews returns the news articles based on the given req.
func GetNews(req GetNewsRequest) ([]News, error) {
	return DefaultClient.GetNews(req)
}

func (c *Client) get(u *url.URL) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")

	return c.do(c, req)
}

func unmarshal(resp *http.Response, v easyjson.Unmarshaler) error {
	defer func() {
		// The underlying TCP connection can not be reused if the body is not fully read
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
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
	return easyjson.UnmarshalFromReader(reader, v)
}
