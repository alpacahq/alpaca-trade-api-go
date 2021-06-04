package stream

import (
	"errors"

	"github.com/vmihailenco/msgpack/v5"
)

// ErrSubscriptionChangeBeforeConnect is returned when the client attempts to change subscriptions before
// calling Connect
var ErrSubscriptionChangeBeforeConnect = errors.New("subscription change attempted before calling Connect")

// ErrSubscriptionChangeAfterTerminated is returned when client attempts to change subscriptions after
// the client has been terminated
var ErrSubscriptionChangeAfterTerminated = errors.New("subscription change after client termination")

// ErrSubscriptionChangeAlreadyInProgress is returned when a subscription change is called concurrently
// with another
var ErrSubscriptionChangeAlreadyInProgress = errors.New("subscription change already in progress")

// ErrSubscriptionChangeInterrupted is returned when a subscription change was in progress when the client
// has terminated
var ErrSubscriptionChangeInterrupted = errors.New("subscription change interrupted by client termination")

type subChangeRequest struct {
	msg    []byte
	result chan error
}

func newSubChangeRequest() subChangeRequest {
	return subChangeRequest{
		result: make(chan error),
	}
}

func (sc *stocksClient) SubscribeToTrades(handler func(Trade), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.tradeHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, newSubChangeRequest(),
		symbols, []string{}, []string{}, []string{})
}

func (sc *stocksClient) SubscribeToQuotes(handler func(Quote), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.quoteHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, symbols, []string{}, []string{})
}

func (sc *stocksClient) SubscribeToBars(handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.barHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, []string{}, symbols, []string{})
}

func (sc *stocksClient) SubscribeToDailyBars(handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.dailyBarHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, []string{}, []string{}, symbols)
}

func (sc *stocksClient) UnsubscribeFromTrades(symbols ...string) error {
	return sc.handleSubChange(false, newSubChangeRequest(),
		symbols, []string{}, []string{}, []string{})
}

func (sc *stocksClient) UnsubscribeFromQuotes(symbols ...string) error {
	return sc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, symbols, []string{}, []string{})
}

func (sc *stocksClient) UnsubscribeFromBars(symbols ...string) error {
	return sc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, []string{}, symbols, []string{})
}

func (sc *stocksClient) UnsubscribeFromDailyBars(symbols ...string) error {
	return sc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, []string{}, []string{}, symbols)
}

func (cc *cryptoClient) SubscribeToTrades(handler func(CryptoTrade), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.tradeHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, newSubChangeRequest(),
		symbols, []string{}, []string{}, []string{})
}

func (cc *cryptoClient) SubscribeToQuotes(handler func(CryptoQuote), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.quoteHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, symbols, []string{}, []string{})
}

func (cc *cryptoClient) SubscribeToBars(handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.barHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, []string{}, symbols, []string{})
}

func (cc *cryptoClient) SubscribeToDailyBars(handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.dailyBarHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, newSubChangeRequest(),
		[]string{}, []string{}, []string{}, symbols)
}

func (cc *cryptoClient) UnsubscribeFromTrades(symbols ...string) error {
	return cc.handleSubChange(false, newSubChangeRequest(),
		symbols, []string{}, []string{}, []string{})
}

func (cc *cryptoClient) UnsubscribeFromQuotes(symbols ...string) error {
	return cc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, symbols, []string{}, []string{})
}

func (cc *cryptoClient) UnsubscribeFromBars(symbols ...string) error {
	return cc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, []string{}, symbols, []string{})
}

func (cc *cryptoClient) UnsubscribeFromDailyBars(symbols ...string) error {
	return cc.handleSubChange(false, newSubChangeRequest(),
		[]string{}, []string{}, []string{}, symbols)
}

func (c *client) handleSubChange(
	subscribe bool, request subChangeRequest,
	trades, quotes, bars, dailyBars []string,
) error {
	if !c.connectCalled {
		return ErrSubscriptionChangeBeforeConnect
	}

	if len(trades) == 0 && len(quotes) == 0 && len(bars) == 0 && len(dailyBars) == 0 {
		return nil
	}
	msg, err := getSubChangeMessage(subscribe, trades, quotes, bars, dailyBars)
	if err != nil {
		return err
	}
	request.msg = msg

	if err := c.setSubChangeRequest(&request); err != nil {
		return err
	}

	return <-request.result
}

func (c *client) setSubChangeRequest(request *subChangeRequest) error {
	c.pendingSubChangeMutex.Lock()
	defer c.pendingSubChangeMutex.Unlock()
	if c.hasTerminated {
		return ErrSubscriptionChangeAfterTerminated
	}
	if c.pendingSubChange != nil {
		return ErrSubscriptionChangeAlreadyInProgress
	}
	c.pendingSubChange = request
	c.subChanges <- request.msg
	return nil
}

func getSubChangeMessage(subscribe bool, trades, quotes, bars, dailyBars []string) ([]byte, error) {
	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}
	return msgpack.Marshal(map[string]interface{}{
		"action":    action,
		"trades":    trades,
		"quotes":    quotes,
		"bars":      bars,
		"dailyBars": dailyBars,
	})
}
