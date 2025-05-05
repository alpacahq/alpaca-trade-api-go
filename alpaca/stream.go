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
	"sync"
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
	return c.StreamTradeUpdatesComplex(ctx, nil, handler, req)
}

// StreamTradeUpdatesComplex like StreamTradeUpdates, but has ready callback
func (c *Client) StreamTradeUpdatesComplex(
	ctx context.Context, ready func(), handler func(TradeUpdate), req StreamTradeUpdatesRequest,
) error {
	transport := http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, addr)
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

	if ready != nil {
		ready()
	}

	return c.processTradeUpdates(resp.Body, handler)
}

// processTradeUpdates processes the trade updates from the response body
func (c *Client) processTradeUpdates(body io.Reader, handler func(TradeUpdate)) error {
	reader := bufio.NewReader(body)
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
// It runs in the background and keeps calling the handler function for each trade update.
// The provided ctx is only used to control the initial connection of the stream.
// Once the stream is successfully started, it will continue running independently of the ctx.
// If an error happens it logs it and retries immediately.
// Returns a terminate function that can be called to stop streaming.
func (c *Client) StreamTradeUpdatesInBackground(
	ctx context.Context, handler func(TradeUpdate),
) (func(), error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	streamCtx, streamCtxCancel := context.WithCancel(context.Background())
	var readyOnce sync.Once
	readyC := make(chan struct{})
	doneC := make(chan struct{})
	go func() {
		defer close(doneC)

		var lastMessage time.Time
		for {
			req := StreamTradeUpdatesRequest{}
			if !lastMessage.IsZero() {
				req.Since = lastMessage.Add(time.Nanosecond)
			}
			err := c.StreamTradeUpdatesComplex(
				streamCtx,
				func() {
					readyOnce.Do(func() {
						close(readyC)
					})
				},
				func(tu TradeUpdate) {
					lastMessage = tu.At
					handler(tu)
				},
				req)
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("alpaca stream trade updates error: %v", err)
		}
	}()

	terminate := func() {
		streamCtxCancel()
		<-doneC
	}
	select {
	case <-ctx.Done():
		terminate()
		return nil, ctx.Err()
	case <-readyC:
		return terminate, nil
	}
}

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate), req StreamTradeUpdatesRequest) error {
	return DefaultClient.StreamTradeUpdates(ctx, handler, req)
}

// StreamTradeUpdatesInBackground streams the trade updates of the account.
// It runs in the background and keeps calling the handler function for each trade update.
// The provided ctx is only used to control the initial connection of the stream.
// Once the stream is successfully started, it will continue running independently of the ctx.
// If an error happens it logs it and retries immediately.
// Returns a terminate function that can be called to stop streaming.
func StreamTradeUpdatesInBackground(
	ctx context.Context, handler func(TradeUpdate),
) (func(), error) {
	return DefaultClient.StreamTradeUpdatesInBackground(ctx, handler)
}
