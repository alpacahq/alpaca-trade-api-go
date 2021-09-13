package alpaca

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func (c *client) StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
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

// StreamTradeUpdates streams the trade updates of the account. It blocks and keeps calling the handler
// function for each trade update until the context is cancelled.
func StreamTradeUpdates(ctx context.Context, handler func(TradeUpdate)) error {
	return DefaultClient.StreamTradeUpdates(ctx, handler)
}
