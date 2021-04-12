package new

import (
	"errors"

	"github.com/vmihailenco/msgpack/v5"
)

var ErrSubChangeBeforeConnect = errors.New("subscription change attempted before connecting")
var ErrSubChangeAfterTerminated = errors.New("subscription change after client termination")
var ErrSubChangeAlreadyInProgress = errors.New("subscription change already in progress")
var ErrSubChangeInterrupted = errors.New("sub change interrupted by client termination")

type handlerKind int

const (
	noHandler handlerKind = -1 + iota
	tradeHandler
	quoteHandler
	barHandler
)

type subChangeRequest struct {
	msg             []byte
	result          chan error
	handlerToChange handlerKind
	tradeHandler    *func(trade Trade)
	quoteHandler    *func(quote Quote)
	barHandler      *func(bar Bar)
}

func (c *client) SubscribeToTrades(handler func(Trade), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: tradeHandler,
		tradeHandler:    &handler,
	}
	return c.handleSubChange(true, symbols, []string{}, []string{}, req)
}

func (c *client) SubscribeToQuotes(handler func(Quote), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: quoteHandler,
		quoteHandler:    &handler,
	}
	return c.handleSubChange(true, []string{}, symbols, []string{}, req)
}

func (c *client) SubscribeToBars(handler func(Bar), symbols ...string) error {
	req := subChangeRequest{
		result:          make(chan error),
		handlerToChange: barHandler,
		barHandler:      &handler,
	}
	return c.handleSubChange(true, []string{}, []string{}, symbols, req)
}

func (c *client) UnsubscribeFromTrades(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, symbols, []string{}, []string{}, req)
}

func (c *client) UnsubscribeFromQuotes(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, []string{}, symbols, []string{}, req)
}

func (c *client) UnsubscribeFromBars(symbols ...string) error {
	req := subChangeRequest{
		handlerToChange: noHandler,
		result:          make(chan error),
	}
	return c.handleSubChange(false, []string{}, []string{}, symbols, req)
}

func (c *client) handleSubChange(subscribe bool, trades, quotes, bars []string, request subChangeRequest) error {
	if !c.connectCalled {
		return ErrSubChangeBeforeConnect
	}

	// Special case: if no symbols are changed we update the handler
	if len(trades) == 0 && len(quotes) == 0 && len(bars) == 0 {
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
			}
		}
		return nil
	}
	msg, err := getSubChangeMessage(subscribe, trades, quotes, bars)
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
		return ErrSubChangeAfterTerminated
	}
	if c.pendingSubChange != nil {
		return ErrSubChangeAlreadyInProgress
	}
	c.pendingSubChange = request
	c.subChanges <- request.msg
	return nil
}

func getSubChangeMessage(subscribe bool, trades, quotes, bars []string) ([]byte, error) {
	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}
	return msgpack.Marshal(map[string]interface{}{
		"action": action,
		"trades": trades,
		"quotes": quotes,
		"bars":   bars,
	})
}
