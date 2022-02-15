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

func (sc *stocksClient) SubscribeToUpdatedBars(handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.updatedBarHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(true, subscriptions{updatedBars: symbols})
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

func (sc *stocksClient) RegisterCancelErrors(handler func(TradeCancelError)) {
	sc.handler.mu.Lock()
	sc.handler.cancelErrorHandler = handler
	sc.handler.mu.Unlock()
}

func (sc *stocksClient) RegisterCorrections(handler func(TradeCorrection)) {
	sc.handler.mu.Lock()
	sc.handler.correctionHandler = handler
	sc.handler.mu.Unlock()
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

func (sc *stocksClient) UnsubscribeFromUpdatedBars(symbols ...string) error {
	return sc.handleSubChange(false, subscriptions{updatedBars: symbols})
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

func (sc *stocksClient) UnregisterCancelErrors() {
	sc.handler.mu.Lock()
	sc.handler.cancelErrorHandler = func(TradeCancelError) {}
	sc.handler.mu.Unlock()
}

func (sc *stocksClient) UnregisterCorrections() {
	sc.handler.mu.Lock()
	sc.handler.correctionHandler = func(TradeCorrection) {}
	sc.handler.mu.Unlock()
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

func (cc *cryptoClient) SubscribeToUpdatedBars(handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.updatedBarHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(true, subscriptions{updatedBars: symbols})
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

func (cc *cryptoClient) UnsubscribeFromUpdatedBars(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{updatedBars: symbols})
}

func (cc *cryptoClient) UnsubscribeFromDailyBars(symbols ...string) error {
	return cc.handleSubChange(false, subscriptions{dailyBars: symbols})
}

func (nc *newsClient) SubscribeToNews(handler func(News), symbols ...string) error {
	nc.handler.mu.Lock()
	nc.handler.newsHandler = handler
	nc.handler.mu.Unlock()
	return nc.client.handleSubChange(true, subscriptions{news: symbols})
}

func (nc *newsClient) UnsubscribeFromNews(symbols ...string) error {
	return nc.handleSubChange(false, subscriptions{news: symbols})
}

type subscriptions struct {
	trades       []string
	quotes       []string
	bars         []string
	updatedBars  []string
	dailyBars    []string
	statuses     []string
	lulds        []string
	cancelErrors []string // Subscribed automatically.
	corrections  []string // Subscribed automatically.
	news         []string
}

func (s subscriptions) noSubscribeCallNecessary() bool {
	return len(s.trades) == 0 && len(s.quotes) == 0 && len(s.bars) == 0 && len(s.updatedBars) == 0 &&
		len(s.dailyBars) == 0 && len(s.statuses) == 0 && len(s.lulds) == 0 && len(s.news) == 0
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
		"action":      action,
		"trades":      changes.trades,
		"quotes":      changes.quotes,
		"bars":        changes.bars,
		"updatedBars": changes.updatedBars,
		"dailyBars":   changes.dailyBars,
		"statuses":    changes.statuses,
		"lulds":       changes.lulds,
		"news":        changes.news,
		// No need to subscribe to cancel errors or corrections explicitly.
	})
}
