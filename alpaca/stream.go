package alpaca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func (c *Client) StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	transport := http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		},
	}
	client := http.Client{
		Transport: &transport,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.opts.BaseURL+"/events/trades", nil)
	if err != nil {
		return err
	}
	if c.opts.OAuth != "" {
		req.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	} else {
		req.Header.Set("APCA-API-KEY-ID", c.opts.APIKey)
		req.Header.Set("APCA-API-SECRET-KEY", c.opts.APISecret)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("trade events returned HTTP %s, body: %s", resp.Status, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		msg, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		const dataPrefix = "data: "
		if !bytes.HasPrefix(msg, []byte(dataPrefix)) {
			continue
		}
		msg = msg[(len(dataPrefix)):]
		var tu TradeUpdate
		if err := json.Unmarshal(msg, &tu); err != nil {
			return err
		}
		handler(tu)
	}
}

// StreamTradeUpdatesInBackground streams the trade updates of the account.
// It runs in the background and keeps calling the handler function for each trade update
// until the context is cancelled. If an error happens it logs it and retries immediately.
func (c *Client) StreamTradeUpdatesInBackground(ctx context.Context, handler func(TradeUpdate)) {
	go func() {
		for {
			err := c.StreamTradeUpdates(ctx, handler)
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("alpaca stream trade updates error: %v", err)
		}
	}()
}

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	return DefaultClient.StreamTradeUpdates(ctx, handler)
}

// StreamTradeUpdatesInBackground streams the trade updates of the account.
// It runs in the background and keeps calling the handler function for each trade update
// until the context is cancelled. If an error happens it logs it and retries immediately.
func StreamTradeUpdatesInBackground(ctx context.Context, handler func(TradeUpdate)) {
	DefaultClient.StreamTradeUpdatesInBackground(ctx, handler)
}
