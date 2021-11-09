package stream

import (
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

type subChangeRequest struct {
	msg    []byte
	result chan error
}

func (sc *stocksClient) SubscribeToTrades(handler func(Trade), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.tradeHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{trades: symbols})
}

func (sc *stocksClient) SubscribeToQuotes(handler func(Quote), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.quoteHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{quotes: symbols})
}

func (sc *stocksClient) SubscribeToBars(handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.barHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{bars: symbols})
}

func (sc *stocksClient) SubscribeToDailyBars(handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.dailyBarHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{dailyBars: symbols})
}

func (sc *stocksClient) SubscribeToStatuses(handler func(TradingStatus), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.tradingStatusHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{statuses: symbols})
}

func (sc *stocksClient) SubscribeToLULDs(handler func(LULD), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.luldHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{lulds: symbols})
}

func (sc *stocksClient) UnsubscribeFromTrades(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{trades: symbols})
}

func (sc *stocksClient) UnsubscribeFromQuotes(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{quotes: symbols})
}

func (sc *stocksClient) UnsubscribeFromBars(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{bars: symbols})
}

func (sc *stocksClient) UnsubscribeFromDailyBars(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{dailyBars: symbols})
}

func (sc *stocksClient) UnsubscribeFromStatuses(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{statuses: symbols})
}

func (sc *stocksClient) UnsubscribeFromLULDs(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{lulds: symbols})
}

func (cc *cryptoClient) SubscribeToTrades(handler func(CryptoTrade), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.tradeHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, subscriptions{trades: symbols})
}

func (cc *cryptoClient) SubscribeToQuotes(handler func(CryptoQuote), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.quoteHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, subscriptions{quotes: symbols})
}

func (cc *cryptoClient) SubscribeToBars(handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.barHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, subscriptions{bars: symbols})
}

func (cc *cryptoClient) SubscribeToDailyBars(handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.dailyBarHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, subscriptions{dailyBars: symbols})
}

func (cc *cryptoClient) UnsubscribeFromTrades(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{trades: symbols})
}

func (cc *cryptoClient) UnsubscribeFromQuotes(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{quotes: symbols})
}

func (cc *cryptoClient) UnsubscribeFromBars(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{bars: symbols})
}

func (cc *cryptoClient) UnsubscribeFromDailyBars(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{dailyBars: symbols})
}

type subscriptions struct {
	trades    []string
	quotes    []string
	bars      []string
	dailyBars []string
	statuses  []string
	lulds     []string
}

func (s subscriptions) noSubscribeCallNecessary() bool {
	return len(s.trades) == 0 && len(s.quotes) == 0 && len(s.bars) == 0 &&
		len(s.dailyBars) == 0 && len(s.statuses) == 0 && len(s.lulds) == 0
}

var timeAfter = time.After

func (c *client) handleSubChange(subscribe bool, changes subscriptions) error {
	if !c.connectCalled {
		return ErrSubscriptionChangeBeforeConnect
	}

	if changes.noSubscribeCallNecessary() {
		return nil
	}
	msg, err := getSubChangeMessage(subscribe, changes)
	if err != nil {
		return err
	}

	request := subChangeRequest{
		result: make(chan error),
		msg:    msg,
	}

	if err := c.setSubChangeRequest(&request); err != nil {
		return err
	}

	select {
	case err := <-request.result:
		return err
	case <-timeAfter(3 * time.Second):
		c.pendingSubChangeMutex.Lock()
		defer c.pendingSubChangeMutex.Unlock()
		c.pendingSubChange = nil
	}

	return ErrSubscriptionChangeTimeout
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

func getSubChangeMessage(subscribe bool, changes subscriptions) ([]byte, error) {
	action := "subscribe"
	if !subscribe {
		action = "unsubscribe"
	}
	return msgpack.Marshal(map[string]interface{}{
		"action":    action,
		"trades":    changes.trades,
		"quotes":    changes.quotes,
		"bars":      changes.bars,
		"dailyBars": changes.dailyBars,
		"statuses":  changes.statuses,
		"lulds":     changes.lulds,
	})
}
