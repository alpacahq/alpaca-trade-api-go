package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

var (
	initializeTimeout        = 3 * time.Second
	authRetryDelayMultiplier = 1
	authRetryCount           = 15
)

// initialize performs the initial flow:
// 1. wait to be welcomed
// 2. authenticates (and waits for the response)
// 3. subscribes (and waits for the response)
//
// If it runs into retriable issues during the flow it retries for a while
func (c *client) initialize(ctx context.Context) error {
	readConnectedCtx, cancelReadConnected := context.WithTimeout(ctx, initializeTimeout)
	defer cancelReadConnected()

	if err := c.readConnected(readConnectedCtx); err != nil {
		return fmt.Errorf("failed to read connected: %w", err)
	}

	var retryErr error
	// If we run into a retriable error during auth we should try
	// again after a delay
	for i := 0; i < authRetryCount; i++ {
		if i > 0 {
			n := i
			if n > 10 {
				n = 10
			}
			sleepDuration := 500 * time.Millisecond * time.Duration(authRetryDelayMultiplier*n)
			c.logger.Infof("datav2stream: retrying auth in %s, attempt %d/%d", sleepDuration, i+1, authRetryCount+1)
			time.Sleep(sleepDuration)
		}
		writeAuthCtx, cancelWriteAuth := context.WithTimeout(ctx, initializeTimeout)
		defer cancelWriteAuth()
		if err := c.writeAuth(writeAuthCtx); err != nil {
			return fmt.Errorf("failed to write auth: %w", err)
		}

		readAuthRespCtx, cancelReadAuthResp := context.WithTimeout(ctx, initializeTimeout)
		defer cancelReadAuthResp()
		retryErr = c.readAuthResponse(readAuthRespCtx)
		if retryErr == nil {
			break
		}
		if !isErrorRetriable(retryErr) {
			return retryErr
		}
		c.logger.Infof("datav2stream: auth error: %s", retryErr)
	}

	if retryErr != nil {
		return retryErr
	}

	if c.sub.noSubscribeCallNecessary() {
		return nil
	}

	writeSubCtx, cancelWriteSub := context.WithTimeout(ctx, initializeTimeout)
	defer cancelWriteSub()
	if err := c.writeSub(writeSubCtx); err != nil {
		return fmt.Errorf("failed to write subscribe: %w", err)
	}

	readRespCtx, cancelReadSub := context.WithTimeout(ctx, initializeTimeout)
	defer cancelReadSub()
	if err := c.readSubResponse(readRespCtx); err != nil {
		return fmt.Errorf("failed to read subscribe response: %w", err)
	}

	return nil
}

func (c *client) readConnected(ctx context.Context) error {
	b, err := c.conn.readMessage(ctx)
	if err != nil {
		return err
	}
	var resps []struct {
		T   string `msgpack:"T"`
		Msg string `msgpack:"msg"`
	}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) != 1 {
		return ErrNoConnected
	}
	if resps[0].T != "success" || resps[0].Msg != "connected" {
		return ErrNoConnected
	}
	return nil
}

func (c *client) writeAuth(ctx context.Context) error {
	msg, err := msgpack.Marshal(map[string]string{
		"action": "auth",
		"key":    c.key,
		"secret": c.secret,
	})
	if err != nil {
		return err
	}

	return c.conn.writeMessage(ctx, msg)
}

// isErrorRetriable returns whether the error is considered retriable during the initialization flow
func isErrorRetriable(err error) bool {
	return errors.Is(err, ErrConnectionLimitExceeded)
}

func (c *client) readAuthResponse(ctx context.Context) error {
	b, err := c.conn.readMessage(ctx)
	if err != nil {
		return err
	}
	var resps []struct {
		T    string `msgpack:"T"`
		Msg  string `msgpack:"msg"`
		Code int    `msgpack:"code"`
	}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) != 1 {
		return ErrBadAuthResponse
	}

	resp := resps[0]

	if resp.T == msgTypeError {
		return errorMessage{
			msg:  resp.Msg,
			code: resp.Code,
		}
	}
	if resp.T != "success" || resp.Msg != "authenticated" {
		return ErrBadAuthResponse
	}

	return nil
}

func (c *client) writeSub(ctx context.Context) error {
	msg, err := getSubChangeMessage(true, c.sub)
	if err != nil {
		return err
	}

	return c.conn.writeMessage(ctx, msg)
}

func (c *client) readSubResponse(ctx context.Context) error {
	b, err := c.conn.readMessage(ctx)
	if err != nil {
		return err
	}
	var resps []struct {
		T            string   `msgpack:"T"`
		Msg          string   `msgpack:"msg"`
		Code         int      `msgpack:"code"`
		Trades       []string `msgpack:"trades"`
		Quotes       []string `msgpack:"quotes"`
		Bars         []string `msgpack:"bars"`
		UpdatedBars  []string `msgpack:"updatedBars"`
		DailyBars    []string `msgpack:"dailyBars"`
		Statuses     []string `msgpack:"statuses"`
		Imbalances   []string `msgpack:"imbalances"`
		LULDs        []string `msgpack:"lulds"`
		CancelErrors []string `msgpack:"cancelErrors"`
		Corrections  []string `msgpack:"corrections"`
		Orderbooks   []string `msgpack:"orderbooks"`
	}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) != 1 {
		return ErrSubResponse
	}
	resp := resps[0]

	if resp.T == msgTypeError {
		return errorMessage{
			msg:  resp.Msg,
			code: resp.Code,
		}
	}
	if resp.T != "subscription" {
		return ErrSubResponse
	}

	c.sub.trades = resp.Trades
	c.sub.quotes = resp.Quotes
	c.sub.bars = resp.Bars
	c.sub.updatedBars = resp.UpdatedBars
	c.sub.dailyBars = resp.DailyBars
	c.sub.statuses = resp.Statuses
	c.sub.imbalances = resp.Imbalances
	c.sub.lulds = resp.LULDs
	c.sub.cancelErrors = resp.CancelErrors
	c.sub.corrections = resp.Corrections
	c.sub.orderbooks = resp.Orderbooks
	return nil
}
