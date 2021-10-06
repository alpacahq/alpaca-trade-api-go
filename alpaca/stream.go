package alpaca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func (c *client) StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
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
	req.Header.Add("APCA-API-KEY-ID", c.opts.ApiKey)
	req.Header.Add("APCA-API-SECRET-KEY", c.opts.ApiSecret)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
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
func (c *client) StreamTradeUpdatesInBackground(ctx context.Context, handler func(TradeUpdate)) {
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
