package marketdata

// GetTradesAsync returns the trades for the given symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetTradesAsync(symbol string, req GetTradesPaginatedRequest, callback func(trades []Trade, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetMultiTradesPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetQuotesAsync returns quotes for the given symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetQuotesAsync(symbol string, req GetQuotesPaginatedRequest, callback func(quotes []Quote, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetMultiQuotesPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetBarsAsync returns bars for the given symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetBarsAsync(symbol string, req GetBarsPaginatedRequest, callback func(bars []Bar, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetMultiBarsPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetAuctionsAsync returns auctions for the given symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetAuctionsAsync(symbol string, req GetAuctionsPaginatedRequest, callback func(auctions []DailyAuctions, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetMultiAuctionsPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetCryptoTradesAsync returns trades for the given crypto symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetCryptoTradesAsync(symbol string, req GetCryptoTradesPaginatedRequest, callback func(trades []CryptoTrade, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetCryptoMultiTradesPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetCryptoBarsAsync returns bars for the given crypto symbol asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetCryptoBarsAsync(symbol string, req GetCryptoBarsPaginatedRequest, callback func(bars []CryptoBar, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetCryptoMultiBarsPaginated([]string{symbol}, req)
		keepGoing := callback(resp[symbol], err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}

// GetNewsAsync returns the news articles based on the given req asynchronously, triggering the callback function on each received batch.
// The callback receives the batch, and an error (if there was one). It can return a boolean to decide whether to continue streaming the data or not.
func (c *Client) GetNewsAsync(req GetNewsPaginatedRequest, callback func(news []News, err error) (keepGoing bool)) error {
	if req.TotalLimit == 0 && req.PageLimit > 0 {
		req.TotalLimit = req.PageLimit
	}
	if req.TotalLimit == 0 {
		req.TotalLimit = v2MaxLimit
	}

	for {
		resp, nextPageToken, err := c.GetNewsPaginated(req)
		keepGoing := callback(resp, err)
		req.PageToken = nextPageToken
		if keepGoing && nextPageToken != "" {
			continue
		}
		if err != nil {
			return err
		}
		if nextPageToken == "" {
			return nil
		}
	}
}
