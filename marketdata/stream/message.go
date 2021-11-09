package stream

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

type msgHandler interface {
	handleTrade(d *msgpack.Decoder, n int) error
	handleQuote(d *msgpack.Decoder, n int) error
	handleBar(d *msgpack.Decoder, n int) error
	handleDailyBar(d *msgpack.Decoder, n int) error
	handleTradingStatus(d *msgpack.Decoder, n int) error
	handleLULD(d *msgpack.Decoder, n int) error
}

func (c *client) handleMessage(b []byte) error {
	d := msgpack.GetDecoder()
	defer msgpack.PutDecoder(d)

	reader := bytes.NewReader(b)
	d.Reset(reader)

	arrLen, err := d.DecodeArrayLen()
	if err != nil || arrLen < 1 {
		return err
	}

	for i := 0; i < arrLen; i++ {
		var n int
		n, err = d.DecodeMapLen()
		if err != nil {
			return err
		}
		if n < 1 {
			continue
		}

		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		if key != "T" {
			return fmt.Errorf("first key is not T but: %s", key)
		}
		T, err := d.DecodeString()
		if err != nil {
			return err
		}
		n-- // T already processed

		switch T {
		case "t":
			err = c.handler.handleTrade(d, n)
		case "q":
			err = c.handler.handleQuote(d, n)
		case "b":
			err = c.handler.handleBar(d, n)
		case "d":
			err = c.handler.handleDailyBar(d, n)
		case "s":
			err = c.handler.handleTradingStatus(d, n)
		case "l":
			err = c.handler.handleLULD(d, n)
		case "subscription":
			err = c.handleSubscriptionMessage(d, n)
		case "error":
			err = c.handleErrorMessage(d, n)
		default:
			err = c.handleOther(d, n)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

type stocksMsgHandler struct {
	mu                   sync.RWMutex
	tradeHandler         func(trade Trade)
	quoteHandler         func(quote Quote)
	barHandler           func(bar Bar)
	dailyBarHandler      func(bar Bar)
	tradingStatusHandler func(ts TradingStatus)
	luldHandler          func(luld LULD)
}

var _ msgHandler = (*stocksMsgHandler)(nil)

func (h *stocksMsgHandler) handleTrade(d *msgpack.Decoder, n int) error {
	trade := Trade{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "i":
			trade.ID, err = d.DecodeInt64()
		case "S":
			trade.Symbol, err = d.DecodeString()
		case "x":
			trade.Exchange, err = d.DecodeString()
		case "p":
			trade.Price, err = d.DecodeFloat64()
		case "s":
			trade.Size, err = d.DecodeUint32()
		case "t":
			trade.Timestamp, err = d.DecodeTime()
		case "c":
			trade.Conditions, err = decodeStringSlice(d)
		case "z":
			trade.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	tradeHandler := h.tradeHandler
	h.mu.RUnlock()
	tradeHandler(trade)
	return nil
}

func (h *stocksMsgHandler) handleQuote(d *msgpack.Decoder, n int) error {
	quote := Quote{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			quote.Symbol, err = d.DecodeString()
		case "bx":
			quote.BidExchange, err = d.DecodeString()
		case "bp":
			quote.BidPrice, err = d.DecodeFloat64()
		case "bs":
			quote.BidSize, err = d.DecodeUint32()
		case "ax":
			quote.AskExchange, err = d.DecodeString()
		case "ap":
			quote.AskPrice, err = d.DecodeFloat64()
		case "as":
			quote.AskSize, err = d.DecodeUint32()
		case "t":
			quote.Timestamp, err = d.DecodeTime()
		case "c":
			quote.Conditions, err = decodeStringSlice(d)
		case "z":
			quote.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	quoteHandler := h.quoteHandler
	h.mu.RUnlock()
	quoteHandler(quote)
	return nil
}

func (h *stocksMsgHandler) decodeBar(d *msgpack.Decoder, n int) (Bar, error) {
	bar := Bar{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return bar, err
		}
		switch key {
		case "S":
			bar.Symbol, err = d.DecodeString()
		case "o":
			bar.Open, err = d.DecodeFloat64()
		case "h":
			bar.High, err = d.DecodeFloat64()
		case "l":
			bar.Low, err = d.DecodeFloat64()
		case "c":
			bar.Close, err = d.DecodeFloat64()
		case "v":
			bar.Volume, err = d.DecodeUint64()
		case "t":
			bar.Timestamp, err = d.DecodeTime()
		case "n":
			bar.TradeCount, err = d.DecodeUint64()
		case "vw":
			bar.VWAP, err = d.DecodeFloat64()
		default:
			err = d.Skip()
		}
		if err != nil {
			return bar, err
		}
	}
	return bar, nil
}

func (h *stocksMsgHandler) handleBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	barHandler := h.barHandler
	h.mu.RUnlock()
	barHandler(bar)
	return nil
}

func (h *stocksMsgHandler) handleDailyBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	dailyBarHandler := h.dailyBarHandler
	h.mu.RUnlock()
	dailyBarHandler(bar)
	return nil
}

func (h *stocksMsgHandler) handleTradingStatus(d *msgpack.Decoder, n int) error {
	ts := TradingStatus{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			ts.Symbol, err = d.DecodeString()
		case "sc":
			ts.StatusCode, err = d.DecodeString()
		case "sm":
			ts.StatusMsg, err = d.DecodeString()
		case "rc":
			ts.ReasonCode, err = d.DecodeString()
		case "rm":
			ts.ReasonMsg, err = d.DecodeString()
		case "t":
			ts.Timestamp, err = d.DecodeTime()
		case "z":
			ts.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	handler := h.tradingStatusHandler
	h.mu.RUnlock()
	handler(ts)
	return nil
}

func (h *stocksMsgHandler) handleLULD(d *msgpack.Decoder, n int) error {
	luld := LULD{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			luld.Symbol, err = d.DecodeString()
		case "u":
			luld.LimitUpPrice, err = d.DecodeFloat64()
		case "d":
			luld.LimitDownPrice, err = d.DecodeFloat64()
		case "i":
			luld.Indicator, err = d.DecodeString()
		case "t":
			luld.Timestamp, err = d.DecodeTime()
		case "z":
			luld.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	handler := h.luldHandler
	h.mu.RUnlock()
	handler(luld)
	return nil
}

type cryptoMsgHandler struct {
	mu              sync.RWMutex
	tradeHandler    func(trade CryptoTrade)
	quoteHandler    func(quote CryptoQuote)
	barHandler      func(bar CryptoBar)
	dailyBarHandler func(bar CryptoBar)
}

var _ msgHandler = (*cryptoMsgHandler)(nil)

func (h *cryptoMsgHandler) handleTrade(d *msgpack.Decoder, n int) error {
	trade := CryptoTrade{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			trade.Symbol, err = d.DecodeString()
		case "x":
			trade.Exchange, err = d.DecodeString()
		case "p":
			trade.Price, err = d.DecodeFloat64()
		case "s":
			trade.Size, err = d.DecodeFloat64()
		case "t":
			trade.Timestamp, err = d.DecodeTime()
		case "i":
			trade.Id, err = d.DecodeInt64()
		case "tks":
			trade.TakerSide, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	tradeHandler := h.tradeHandler
	h.mu.RUnlock()
	tradeHandler(trade)
	return nil
}

func (h *cryptoMsgHandler) handleQuote(d *msgpack.Decoder, n int) error {
	quote := CryptoQuote{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			quote.Symbol, err = d.DecodeString()
		case "x":
			quote.Exchange, err = d.DecodeString()
		case "bp":
			quote.BidPrice, err = d.DecodeFloat64()
		case "bs":
			quote.BidSize, err = d.DecodeFloat64()
		case "ap":
			quote.AskPrice, err = d.DecodeFloat64()
		case "as":
			quote.AskSize, err = d.DecodeFloat64()
		case "t":
			quote.Timestamp, err = d.DecodeTime()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	quoteHandler := h.quoteHandler
	h.mu.RUnlock()
	quoteHandler(quote)
	return nil
}

func (h *cryptoMsgHandler) decodeBar(d *msgpack.Decoder, n int) (CryptoBar, error) {
	bar := CryptoBar{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return bar, err
		}
		switch key {
		case "S":
			bar.Symbol, err = d.DecodeString()
		case "x":
			bar.Exchange, err = d.DecodeString()
		case "o":
			bar.Open, err = d.DecodeFloat64()
		case "h":
			bar.High, err = d.DecodeFloat64()
		case "l":
			bar.Low, err = d.DecodeFloat64()
		case "c":
			bar.Close, err = d.DecodeFloat64()
		case "v":
			bar.Volume, err = d.DecodeFloat64()
		case "t":
			bar.Timestamp, err = d.DecodeTime()
		case "n":
			bar.TradeCount, err = d.DecodeUint64()
		case "vw":
			bar.VWAP, err = d.DecodeFloat64()
		default:
			err = d.Skip()
		}
		if err != nil {
			return bar, err
		}
	}
	return bar, nil
}

func (h *cryptoMsgHandler) handleBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	barHandler := h.barHandler
	h.mu.RUnlock()
	barHandler(bar)
	return nil
}

func (h *cryptoMsgHandler) handleDailyBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	dailyBarHandler := h.dailyBarHandler
	h.mu.RUnlock()
	dailyBarHandler(bar)
	return nil
}

func (h *cryptoMsgHandler) handleTradingStatus(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleLULD(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func discardMapContents(d *msgpack.Decoder, n int) error {
	for i := 0; i < n; i++ {
		// key
		if _, err := d.DecodeString(); err != nil {
			return err
		}
		// value
		if err := d.Skip(); err != nil {
			return err
		}
	}
	return nil
}

var errMessageHandler = func(c *client, e errorMessage) error {
	c.pendingSubChangeMutex.Lock()
	defer c.pendingSubChangeMutex.Unlock()
	if c.pendingSubChange != nil {
		c.pendingSubChange.result <- e
		c.pendingSubChange = nil
	}

	if e.code == 0 || e.msg == "" {
		return fmt.Errorf("code: %d, msg: %s", e.code, e.msg)
	}

	c.logger.Warnf("datav2stream: got error from server: %s", e)
	return nil
}

func (c *client) handleErrorMessage(d *msgpack.Decoder, n int) error {
	e := errorMessage{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "msg":
			e.msg, err = d.DecodeString()
		case "code":
			e.code, err = d.DecodeInt()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}

	return errMessageHandler(c, e)
}

var subMessageHandler = func(c *client, s subscriptions) error {
	c.pendingSubChangeMutex.Lock()
	defer c.pendingSubChangeMutex.Unlock()
	c.sub.trades = s.trades
	c.sub.quotes = s.quotes
	c.sub.bars = s.bars
	c.sub.dailyBars = s.dailyBars
	c.sub.statuses = s.statuses
	c.sub.lulds = s.lulds
	if c.pendingSubChange != nil {
		psc := c.pendingSubChange
		psc.result <- nil
		c.pendingSubChange = nil
	}

	return nil
}

func (c *client) handleSubscriptionMessage(d *msgpack.Decoder, n int) error {
	s := subscriptions{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "trades":
			s.trades, err = decodeStringSlice(d)
		case "quotes":
			s.quotes, err = decodeStringSlice(d)
		case "bars":
			s.bars, err = decodeStringSlice(d)
		case "dailyBars":
			s.dailyBars, err = decodeStringSlice(d)
		case "statuses":
			s.statuses, err = decodeStringSlice(d)
		case "lulds":
			s.lulds, err = decodeStringSlice(d)
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}

	return subMessageHandler(c, s)
}

func (c *client) handleOther(d *msgpack.Decoder, n int) error {
	for i := 0; i < n; i++ {
		// key
		if err := d.Skip(); err != nil {
			return err
		}
		// value
		if err := d.Skip(); err != nil {
			return err
		}
	}
	return nil
}

func decodeStringSlice(d *msgpack.Decoder) ([]string, error) {
	var length int
	var err error
	if length, err = d.DecodeArrayLen(); err != nil {
		return nil, err
	}
	if length < 0 {
		return []string{}, nil
	}
	res := make([]string, length)
	for i := 0; i < length; i++ {
		if s, err := d.DecodeString(); err != nil {
			return nil, err
		} else {
			res[i] = s
		}
	}
	return res, nil
}
