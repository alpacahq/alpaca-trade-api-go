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
	"net/url"
	"time"
)

type StreamTradeUpdatesRequest struct {
	Since   time.Time
	Until   time.Time
	SinceID string
	UntilID string
}

// StreamTradeUpdates streams the trade updates of the account.
func (c *Client) StreamTradeUpdates(
	ctx context.Context, handler func(TradeUpdate), req StreamTradeUpdatesRequest,
) error {
	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		},
	}
	client := http.Client{
		Transport: &transport,
	}
	u, err := url.Parse(c.opts.BaseURL + "/v2/events/trades")
	if err != nil {
		return err
	}
	q := u.Query()
	if !req.Since.IsZero() {
		q.Set("since", req.Since.Format(time.RFC3339Nano))
	}
	if !req.Until.IsZero() {
		q.Set("until", req.Until.Format(time.RFC3339Nano))
	}

	if req.SinceID != "" {
		q.Set("since_id", req.SinceID)
	}
	if req.UntilID != "" {
		q.Set("until_id", req.UntilID)
	}

	u.RawQuery = q.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	if c.opts.OAuth != "" {
		request.Header.Set("Authorization", "Bearer "+c.opts.OAuth)
	} else {
		request.Header.Set("APCA-API-KEY-ID", c.opts.APIKey)
		request.Header.Set("APCA-API-SECRET-KEY", c.opts.APISecret)
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s (HTTP %d)", body, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		msg, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
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
		var lastMessage time.Time
		for {
			req := StreamTradeUpdatesRequest{}
			if !lastMessage.IsZero() {
				req.Since = lastMessage.Add(time.Nanosecond)
			}
			err := c.StreamTradeUpdates(ctx, func(tu TradeUpdate) {
				lastMessage = tu.At
				handler(tu)
			}, req)
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("alpaca stream trade updates error: %v", err)
		}
	}()
}

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate), req StreamTradeUpdatesRequest) error {
	return DefaultClient.StreamTradeUpdates(ctx, handler, req)
}

// StreamTradeUpdatesInBackground streams the trade updates of the account.
// It runs in the background and keeps calling the handler function for each trade update
// until the context is cancelled. If an error happens it logs it and retries immediately.
func StreamTradeUpdatesInBackground(ctx context.Context, handler func(TradeUpdate)) {
	DefaultClient.StreamTradeUpdatesInBackground(ctx, handler)
}
