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

type handlerKind int

const (
	noHandler handlerKind = -1 + iota
	tradeHandler
	quoteHandler
	barHandler
	dailyBarHandler
)

type subChangeRequest struct {
	msg             []byte
	result          chan error
	handlerToChange handlerKind
	tradeHandler    *func(trade Trade)
	quoteHandler    *func(quote Quote)
	barHandler      *func(bar Bar)
	dailyBarHandler *func(bar Bar)
}

func (c *client) SubscribeToTrades(handler func(Trade), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: tradeHandler,
		tradeHandler:    &handler,
	}
	return c.handleSubChange(true, symbols, []string{}, []string{}, []string{}, req)
}

func (c *client) SubscribeToQuotes(handler func(Quote), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: quoteHandler,
		quoteHandler:    &handler,
	}
	return c.handleSubChange(true, []string{}, symbols, []string{}, []string{}, req)
}

func (c *client) SubscribeToBars(handler func(Bar), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: barHandler,
		barHandler:      &handler,
	}
	return c.handleSubChange(true, []string{}, []string{}, symbols, []string{}, req)
}

func (c *client) SubscribeToDailyBars(handler func(Bar), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: dailyBarHandler,
		dailyBarHandler: &handler,
	}
	return c.handleSubChange(true, []string{}, []string{}, []string{}, symbols, req)
}

func (c *client) UnsubscribeFromTrades(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, symbols, []string{}, []string{}, []string{}, req)
}

func (c *client) UnsubscribeFromQuotes(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, []string{}, symbols, []string{}, []string{}, req)
}

func (c *client) UnsubscribeFromBars(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, []string{}, []string{}, symbols, []string{}, req)
}

func (c *client) UnsubscribeFromDailyBars(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, []string{}, []string{}, []string{}, symbols, req)
}

func (c *client) handleSubChange(
	subscribe bool, trades, quotes, bars, dailyBars []string, request subChangeRequest,
) error {
	if !c.connectCalled {
		return ErrSubscriptionChangeBeforeConnect
	}

	// Special case: if no symbols are changed we update the handler
	if len(trades) == 0 && len(quotes) == 0 && len(bars) == 0 && len(dailyBars) == 0 {
		if subscribe {
			c.handlerMutex.Lock()
			defer c.handlerMutex.Unlock()
			switch request.handlerToChange {
			case tradeHandler:
				c.tradeHandler = *request.tradeHandler
			case quoteHandler:
				c.quoteHandler = *request.quoteHandler
			case barHandler:
				c.barHandler = *request.barHandler
			case dailyBarHandler:
				c.dailyBarHandler = *request.dailyBarHandler
			}
		}
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
