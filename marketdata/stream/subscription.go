package stream

import (
	"context"

	"github.com/vmihailenco/msgpack/v5"
)

type subChangeRequest struct {
	msg    []byte
	result chan error
}

func (sc *StocksClient) SubscribeToTrades(ctx context.Context, handler func(Trade), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.tradeHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{trades: symbols})
}

func (sc *StocksClient) SubscribeToQuotes(ctx context.Context, handler func(Quote), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.quoteHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{quotes: symbols})
}

func (sc *StocksClient) SubscribeToBars(ctx context.Context, handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.barHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{bars: symbols})
}

func (sc *StocksClient) SubscribeToUpdatedBars(ctx context.Context, handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.updatedBarHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{updatedBars: symbols})
}

func (sc *StocksClient) SubscribeToDailyBars(ctx context.Context, handler func(Bar), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.dailyBarHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{dailyBars: symbols})
}

func (sc *StocksClient) SubscribeToStatuses(ctx context.Context, handler func(TradingStatus), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.tradingStatusHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{statuses: symbols})
}

func (sc *StocksClient) SubscribeToLULDs(ctx context.Context, handler func(LULD), symbols ...string) error {
	sc.handler.mu.Lock()
	sc.handler.luldHandler = handler
	sc.handler.mu.Unlock()
	return sc.client.handleSubChange(ctx, true, subscriptions{lulds: symbols})
}

func (sc *StocksClient) RegisterCancelErrors(ctx context.Context, handler func(TradeCancelError)) {
	sc.handler.mu.Lock()
	sc.handler.cancelErrorHandler = handler
	sc.handler.mu.Unlock()
}

func (sc *StocksClient) RegisterCorrections(ctx context.Context, handler func(TradeCorrection)) {
	sc.handler.mu.Lock()
	sc.handler.correctionHandler = handler
	sc.handler.mu.Unlock()
}

func (sc *StocksClient) UnsubscribeFromTrades(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{trades: symbols})
}

func (sc *StocksClient) UnsubscribeFromQuotes(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{quotes: symbols})
}

func (sc *StocksClient) UnsubscribeFromBars(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{bars: symbols})
}

func (sc *StocksClient) UnsubscribeFromUpdatedBars(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{updatedBars: symbols})
}

func (sc *StocksClient) UnsubscribeFromDailyBars(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{dailyBars: symbols})
}

func (sc *StocksClient) UnsubscribeFromStatuses(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{statuses: symbols})
}

func (sc *StocksClient) UnsubscribeFromLULDs(ctx context.Context, symbols ...string) error {
	return sc.handleSubChange(ctx, false, subscriptions{lulds: symbols})
}

func (sc *StocksClient) UnregisterCancelErrors(ctx context.Context) {
	sc.handler.mu.Lock()
	sc.handler.cancelErrorHandler = func(TradeCancelError) {}
	sc.handler.mu.Unlock()
}

func (sc *StocksClient) UnregisterCorrections(ctx context.Context) {
	sc.handler.mu.Lock()
	sc.handler.correctionHandler = func(TradeCorrection) {}
	sc.handler.mu.Unlock()
}

func (cc *CryptoClient) SubscribeToTrades(ctx context.Context, handler func(CryptoTrade), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.tradeHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{trades: symbols})
}

func (cc *CryptoClient) SubscribeToQuotes(ctx context.Context, handler func(CryptoQuote), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.quoteHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{quotes: symbols})
}

func (cc *CryptoClient) SubscribeToBars(ctx context.Context, handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.barHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{bars: symbols})
}

func (cc *CryptoClient) SubscribeToUpdatedBars(ctx context.Context, handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.updatedBarHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{updatedBars: symbols})
}

func (cc *CryptoClient) SubscribeToDailyBars(ctx context.Context, handler func(CryptoBar), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.dailyBarHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{dailyBars: symbols})
}

func (cc *CryptoClient) SubscribeToOrderbooks(ctx context.Context, handler func(CryptoOrderbook), symbols ...string) error {
	cc.handler.mu.Lock()
	cc.handler.orderbookHandler = handler
	cc.handler.mu.Unlock()
	return cc.client.handleSubChange(ctx, true, subscriptions{orderbooks: symbols})
}

func (cc *CryptoClient) UnsubscribeFromTrades(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{trades: symbols})
}

func (cc *CryptoClient) UnsubscribeFromQuotes(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{quotes: symbols})
}

func (cc *CryptoClient) UnsubscribeFromBars(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{bars: symbols})
}

func (cc *CryptoClient) UnsubscribeFromUpdatedBars(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{updatedBars: symbols})
}

func (cc *CryptoClient) UnsubscribeFromDailyBars(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{dailyBars: symbols})
}

func (cc *CryptoClient) UnsubscribeFromOrderbooks(ctx context.Context, symbols ...string) error {
	return cc.handleSubChange(ctx, false, subscriptions{orderbooks: symbols})
}

func (nc *NewsClient) SubscribeToNews(ctx context.Context, handler func(News), symbols ...string) error {
	nc.handler.mu.Lock()
	nc.handler.newsHandler = handler
	nc.handler.mu.Unlock()
	return nc.client.handleSubChange(ctx, true, subscriptions{news: symbols})
}

func (nc *NewsClient) UnsubscribeFromNews(ctx context.Context, symbols ...string) error {
	return nc.handleSubChange(ctx, false, subscriptions{news: symbols})
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
	orderbooks   []string
	news         []string
}

func (s subscriptions) noSubscribeCallNecessary() bool {
	return len(s.trades) == 0 && len(s.quotes) == 0 && len(s.bars) == 0 && len(s.updatedBars) == 0 &&
		len(s.dailyBars) == 0 && len(s.statuses) == 0 && len(s.lulds) == 0 &&
		len(s.orderbooks) == 0 && len(s.news) == 0
}

func (c *client) handleSubChange(ctx context.Context, subscribe bool, changes subscriptions) error {
	if !c.connectCalled.Load() {
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
		result: make(chan error, 1),
		msg:    msg,
	}

	if err := c.setSubChangeRequest(&request); err != nil {
		return err
	}

	select {
	case err := <-request.result:
		return err
	case <-ctx.Done():
		c.pendingSubChangeMutex.Lock()
		defer c.pendingSubChangeMutex.Unlock()
		c.pendingSubChange = nil
		return ctx.Err()
	}
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
		"orderbooks":  changes.orderbooks,
		"news":        changes.news,
		// No need to subscribe to cancel errors or corrections explicitly.
	})
}
