package new

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

var initializeTimeout = 3 * time.Second
var authRetryDelayMultiplier = 1
var authRetryCount = 15

// initialize performs the initial flow:
// 1. wait to be welcomed
// 2. authenticates (and waits for the response)
// 3. subscribes (and waits for the response)
//
// If it runs into retriable issues during the flow it retries for a while
func (c *client) initialize(ctx context.Context) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, initializeTimeout)
	defer cancel()

	if err := c.readConnected(ctxWithTimeout); err != nil {
		return err
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
			c.logger.Infof("datav2stream: retring auth in %s, attempt %d/%d", sleepDuration, i+1, authRetryCount+1)
			time.Sleep(sleepDuration)
		}
		ctxWithTimeout, cancel := context.WithTimeout(ctx, initializeTimeout)
		defer cancel()
		if err := c.writeAuth(ctxWithTimeout); err != nil {
			return err
		}

		ctxWithTimeoutResp, cancelResp := context.WithTimeout(ctx, initializeTimeout)
		defer cancelResp()
		retryErr = c.readAuthResponse(ctxWithTimeoutResp)
		if retryErr == nil {
			break
		}
		if !isErrorRetriable(retryErr) {
			return retryErr
		}
	}

	if retryErr != nil {
		return retryErr
	}

	if noSubscribeCallNecessary(c.trades, c.quotes, c.bars) {
		return nil
	}

	ctxWithTimeoutWriteSub, cancelWriteSub := context.WithTimeout(ctx, initializeTimeout)
	defer cancelWriteSub()
	if err := c.writeSub(ctxWithTimeoutWriteSub); err != nil {
		return err
	}

	ctxWithTimeoutReadSub, cancelReadSub := context.WithTimeout(ctx, initializeTimeout)
	defer cancelReadSub()
	if err := c.readSubResponse(ctxWithTimeoutReadSub); err != nil {
		return err
	}

	return nil
}

// ErrNoConnected is returned when the client did not receive the welcome
// message from the server
var ErrNoConnected = errors.New("did not receive connected message")

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

//ErrBadAuthResponse is returned when the client could not successfully authenticate
var ErrBadAuthResponse = errors.New("did not receive authenticated message")

// ErrConnectionLimitExceeded is returned when the client has exceeded their connection limit
var ErrConnectionLimitExceeded = errors.New("connection limit exceeded")

// ErrInvalidCredentials is returned when invalid credentials have been sent by the user
var ErrInvalidCredentials = errors.New("invalid credentials")

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

	// A previous connection may be "stuck" on the server so we may run into
	// `[{"T":"error","code":406,"msg":"connection limit exceeded"}]`
	if resps[0].T == "error" {
		err := fmt.Errorf("auth: error from server: %s", resps[0].Msg)
		if resps[0].Code == 406 {
			return ErrConnectionLimitExceeded
		}
		// invalid credentials
		// [{"T":"error","code":402,"msg":"auth failed"}]
		if resps[0].Code == 402 {
			return ErrInvalidCredentials
		}
		return err
	}
	if resps[0].T != "success" || resps[0].Msg != "authenticated" {
		return ErrBadAuthResponse
	}

	return nil
}

func noSubscribeCallNecessary(trades, quotes, bars []string) bool {
	return len(trades) == 0 && len(quotes) == 0 && len(bars) == 0
}

func (c *client) writeSub(ctx context.Context) error {
	msg, err := getSubChangeMessage(true, c.trades, c.quotes, c.bars)
	if err != nil {
		return err
	}

	return c.conn.writeMessage(ctx, msg)
}

// ErrSubResponse is returned when the client's subscription request was not
// acknowledged
var ErrSubResponse = errors.New("did not receive subscribed message")

func (c *client) readSubResponse(ctx context.Context) error {
	b, err := c.conn.readMessage(ctx)
	if err != nil {
		return err
	}
	var resps []struct {
		T      string   `msgpack:"T"`
		Msg    string   `msgpack:"msg"`
		Code   int      `msgpack:"code"`
		Trades []string `msgpack:"trades"`
		Quotes []string `msgpack:"quotes"`
		Bars   []string `msgpack:"bars"`
	}
	if err := msgpack.Unmarshal(b, &resps); err != nil {
		return err
	}
	if len(resps) != 1 {
		return ErrSubResponse
	}

	if resps[0].T == "error" {
		return fmt.Errorf("sub: error from server: %s", resps[0].Msg)
	}
	if resps[0].T != "subscription" {
		return ErrSubResponse
	}

	c.trades = resps[0].Trades
	c.quotes = resps[0].Quotes
	c.bars = resps[0].Bars
	return nil
}
