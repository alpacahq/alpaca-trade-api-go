package marketdata

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"cloud.google.com/go/civil"
)

const optionPrefix = "v1beta1/options"

// GetOptionTradesRequest contains optional parameters for getting option trades.
type GetOptionTradesRequest struct {
	// Start is the inclusive beginning of the interval
	Start time.Time
	// End is the inclusive end of the interval
	End time.Time
	// TotalLimit is the limit of the total number of the returned trades.
	// If missing, all trades between start end end will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Sort is the sort direction of the data
	Sort Sort
}

// GetOptionTrades returns the option trades for the given symbol.
func (c *Client) GetOptionTrades(symbol string, req GetOptionTradesRequest) ([]OptionTrade, error) {
	resp, err := c.GetOptionMultiTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetOptionMultiTrades returns option trades for the given symbols.
func (c *Client) GetOptionMultiTrades(symbols []string, req GetOptionTradesRequest) (map[string][]OptionTrade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/trades", c.opts.BaseURL, optionPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols: symbols,
		Start:   req.Start,
		End:     req.End,
		Sort:    req.Sort,
	})

	trades := make(map[string][]OptionTrade, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var tradeResp multiOptionTradeResponse
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

// GetOptionBarsRequest contains optional parameters for getting bars
type GetOptionBarsRequest struct {
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
	// Sort is the sort direction of the data
	Sort Sort
}

// GetOptionBars returns a slice of bars for the given symbol.
func (c *Client) GetOptionBars(symbol string, req GetOptionBarsRequest) ([]OptionBar, error) {
	resp, err := c.GetMultiOptionBars([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	return resp[symbol], nil
}

// GetMultiOptionBars returns bars for the given symbols.
func (c *Client) GetMultiOptionBars(symbols []string, req GetOptionBarsRequest) (map[string][]OptionBar, error) {
	bars := make(map[string][]OptionBar, len(symbols))

	u, err := url.Parse(fmt.Sprintf("%s/%s/bars", c.opts.BaseURL, optionPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols: symbols,
		Start:   req.Start,
		End:     req.End,
		Sort:    req.Sort,
	})
	timeframe := OneDay
	if req.TimeFrame.N != 0 {
		timeframe = req.TimeFrame
	}
	q.Set("timeframe", timeframe.String())

	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var barResp multiOptionBarResponse
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

type GetLatestOptionTradeRequest struct {
	Feed OptionFeed
}

// GetLatestOptionTrade returns the latest option trade for a given symbol
func (c *Client) GetLatestOptionTrade(symbol string, req GetLatestOptionTradeRequest) (*OptionTrade, error) {
	resp, err := c.GetLatestOptionTrades([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	trade, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &trade, nil
}

// GetLatestOptionTrades returns the latest option trades for the given symbols
func (c *Client) GetLatestOptionTrades(
	symbols []string, req GetLatestOptionTradeRequest,
) (map[string]OptionTrade, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/trades/latest", c.opts.BaseURL, optionPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols: symbols,
		Feed:    req.Feed,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}
	defer closeResp(resp)

	var latestTradesResp latestOptionTradesResponse
	if err = unmarshal(resp, &latestTradesResp); err != nil {
		return nil, err
	}
	return latestTradesResp.Trades, nil
}

type GetLatestOptionQuoteRequest struct {
	Feed OptionFeed
}

// GetLatestOptionQuote returns the latest option quote for a given symbol
func (c *Client) GetLatestOptionQuote(symbol string, req GetLatestOptionQuoteRequest) (*OptionQuote, error) {
	resp, err := c.GetLatestOptionQuotes([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	quote, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &quote, nil
}

// GetLatestOptionQuotes returns the latest option quotes for the given symbols
func (c *Client) GetLatestOptionQuotes(
	symbols []string, req GetLatestOptionQuoteRequest,
) (map[string]OptionQuote, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/quotes/latest", c.opts.BaseURL, optionPrefix))
	if err != nil {
		return nil, err
	}
	c.setLatestQueryRequest(u, baseLatestRequest{
		Symbols: symbols,
		Feed:    req.Feed,
	})

	resp, err := c.get(u)
	if err != nil {
		return nil, err
	}
	defer closeResp(resp)

	var latestQuotesResp latestOptionQuotesResponse
	if err = unmarshal(resp, &latestQuotesResp); err != nil {
		return nil, err
	}
	return latestQuotesResp.Quotes, nil
}

type GetOptionSnapshotRequest struct {
	// Feed is the source of the data: opra or indicative.
	Feed OptionFeed
	// TotalLimit is the limit of the total number of the returned snapshots.
	// If missing, all snapshots will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
}

// GetOptionSnapshot returns the snapshot for a given symbol
func (c *Client) GetOptionSnapshot(symbol string, req GetOptionSnapshotRequest) (*OptionSnapshot, error) {
	resp, err := c.GetOptionSnapshots([]string{symbol}, req)
	if err != nil {
		return nil, err
	}
	snapshot, ok := resp[symbol]
	if !ok {
		return nil, nil
	}
	return &snapshot, nil
}

// GetOptionSnapshots returns the snapshots for multiple symbols
func (c *Client) GetOptionSnapshots(symbols []string, req GetOptionSnapshotRequest) (map[string]OptionSnapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/snapshots", c.opts.BaseURL, optionPrefix))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Symbols: symbols,
		Feed:    req.Feed,
	})

	snapshots := make(map[string]OptionSnapshot, len(symbols))
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var snapshotsResp optionSnapshotsResponse
		if err := unmarshal(resp, &snapshotsResp); err != nil {
			return nil, err
		}

		for symbol, s := range snapshotsResp.Snapshots {
			snapshots[symbol] = s
			received++
		}
		if snapshotsResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *snapshotsResp.NextPageToken)
	}
	return snapshots, nil
}

type GetOptionChainRequest struct {
	// Feed is the source of the data: opra or indicative.
	Feed OptionFeed
	// TotalLimit is the limit of the total number of the returned snapshots.
	// If missing, all snapshots will be returned.
	TotalLimit int
	// PageLimit is the pagination size. If empty, the default page size will be used.
	PageLimit int
	// Type filters contracts by the type (call or put).
	Type OptionType
	// StrikePriceGte filters contracts with strike price greater than or equal to the specified value.
	StrikePriceGte float64
	// StrikePriceLte filters contracts with strike price less than or equal to the specified value.
	StrikePriceLte float64
	// ExpirationDate filters contracts by the exact expiration date
	ExpirationDate civil.Date
	// ExpirationDateGte filters contracts with expiration date greater than or equal to the specified date.
	ExpirationDateGte civil.Date
	// ExpirationDateLte filters contracts with expiration date less than or equal to the specified date.
	ExpirationDateLte civil.Date
	// RootSymbol filters contracts by the root symbol (e.g. AAPL1)
	RootSymbol string
}

// GetOptionChain returns the snapshot chain for an underlying symbol (e.g. AAPL)
func (c *Client) GetOptionChain(underlyingSymbol string, req GetOptionChainRequest) (map[string]OptionSnapshot, error) {
	u, err := url.Parse(fmt.Sprintf("%s/%s/snapshots/%s", c.opts.BaseURL, optionPrefix, underlyingSymbol))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	c.setBaseQuery(q, baseRequest{
		Feed: req.Feed,
	})
	if req.Type != "" {
		q.Set("type", req.Type)
	}
	if req.StrikePriceGte != 0 {
		q.Set("strike_price_gte", strconv.FormatFloat(req.StrikePriceGte, 'f', -1, 64))
	}
	if req.StrikePriceLte != 0 {
		q.Set("strike_price_lte", strconv.FormatFloat(req.StrikePriceLte, 'f', -1, 64))
	}
	if !req.ExpirationDate.IsZero() {
		q.Set("expiration_date", req.ExpirationDate.String())
	}
	if !req.ExpirationDateLte.IsZero() {
		q.Set("expiration_date_lte", req.ExpirationDateLte.String())
	}
	if !req.ExpirationDateGte.IsZero() {
		q.Set("expiration_date_gte", req.ExpirationDateGte.String())
	}
	if req.RootSymbol != "" {
		q.Set("root_symbol", req.RootSymbol)
	}

	snapshots := make(map[string]OptionSnapshot)
	received := 0
	for req.TotalLimit == 0 || received < req.TotalLimit {
		setQueryLimit(q, req.TotalLimit, req.PageLimit, received, v2MaxLimit)
		u.RawQuery = q.Encode()

		resp, err := c.get(u)
		if err != nil {
			return nil, err
		}

		var snapshotsResp optionSnapshotsResponse
		if err := unmarshal(resp, &snapshotsResp); err != nil {
			return nil, err
		}

		for symbol, s := range snapshotsResp.Snapshots {
			snapshots[symbol] = s
			received++
		}
		if snapshotsResp.NextPageToken == nil {
			break
		}
		q.Set("page_token", *snapshotsResp.NextPageToken)
	}
	return snapshots, nil
}

// GetOptionTrades returns the option trades for the given symbol.
func GetOptionTrades(symbol string, req GetOptionTradesRequest) ([]OptionTrade, error) {
	return DefaultClient.GetOptionTrades(symbol, req)
}

// GetOptionMultiTrades returns option trades for the given symbols.
func GetOptionMultiTrades(symbols []string, req GetOptionTradesRequest) (map[string][]OptionTrade, error) {
	return DefaultClient.GetOptionMultiTrades(symbols, req)
}

// GetOptionBars returns a slice of bars for the given symbol.
func GetOptionBars(symbol string, req GetOptionBarsRequest) ([]OptionBar, error) {
	return DefaultClient.GetOptionBars(symbol, req)
}

// GetMultiOptionBars returns bars for the given symbols.
func GetMultiOptionBars(symbols []string, req GetOptionBarsRequest) (map[string][]OptionBar, error) {
	return DefaultClient.GetMultiOptionBars(symbols, req)
}

// GetLatestOptionTrade returns the latest option trade for a given symbol
func GetLatestOptionTrade(symbol string, req GetLatestOptionTradeRequest) (*OptionTrade, error) {
	return DefaultClient.GetLatestOptionTrade(symbol, req)
}

// GetLatestOptionTrades returns the latest option trades for the given symbols
func GetLatestOptionTrades(symbols []string, req GetLatestOptionTradeRequest) (map[string]OptionTrade, error) {
	return DefaultClient.GetLatestOptionTrades(symbols, req)
}

// GetLatestOptionQuote returns the latest option quote for a given symbol
func GetLatestOptionQuote(symbol string, req GetLatestOptionQuoteRequest) (*OptionQuote, error) {
	return DefaultClient.GetLatestOptionQuote(symbol, req)
}

// GetLatestOptionQuotes returns the latest option quotes for the given symbols
func GetLatestOptionQuotes(symbols []string, req GetLatestOptionQuoteRequest) (map[string]OptionQuote, error) {
	return DefaultClient.GetLatestOptionQuotes(symbols, req)
}

// GetOptionSnapshot returns the snapshot for a given symbol
func GetOptionSnapshot(symbol string, req GetOptionSnapshotRequest) (*OptionSnapshot, error) {
	return DefaultClient.GetOptionSnapshot(symbol, req)
}

// GetOptionSnapshots returns the snapshots for multiple symbols
func GetOptionSnapshots(symbols []string, req GetOptionSnapshotRequest) (map[string]OptionSnapshot, error) {
	return DefaultClient.GetOptionSnapshots(symbols, req)
}

// GetOptionChain returns the snapshot chain for an underlying symbol (e.g. AAPL)
func GetOptionChain(underlyingSymbol string, req GetOptionChainRequest) (map[string]OptionSnapshot, error) {
	return DefaultClient.GetOptionChain(underlyingSymbol, req)
}
