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
	handleUpdatedBar(d *msgpack.Decoder, n int) error
	handleDailyBar(d *msgpack.Decoder, n int) error
	handleTradingStatus(d *msgpack.Decoder, n int) error
	handleImbalance(d *msgpack.Decoder, n int) error
	handleLULD(d *msgpack.Decoder, n int) error
	handleCancelError(d *msgpack.Decoder, n int) error
	handleCorrection(d *msgpack.Decoder, n int) error
	handleOrderbook(d *msgpack.Decoder, n int) error
	handleNews(d *msgpack.Decoder, n int) error
	handleFuturesPricing(d *msgpack.Decoder, n int) error
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
		msgType, err := d.DecodeString()
		if err != nil {
			return err
		}
		n-- // T already processed

		if err := c.handleMessageType(msgType, d, n); err != nil {
			return err
		}
	}

	return nil
}

const msgTypeError = "error"

func (c *client) handleMessageType(msgType string, d *msgpack.Decoder, n int) error {
	switch msgType {
	case "t":
		return c.handler.handleTrade(d, n)
	case "q":
		return c.handler.handleQuote(d, n)
	case "b":
		return c.handler.handleBar(d, n)
	case "u":
		return c.handler.handleUpdatedBar(d, n)
	case "d":
		return c.handler.handleDailyBar(d, n)
	case "s":
		return c.handler.handleTradingStatus(d, n)
	case "i":
		return c.handler.handleImbalance(d, n)
	case "l":
		return c.handler.handleLULD(d, n)
	case "x":
		return c.handler.handleCancelError(d, n)
	case "c":
		return c.handler.handleCorrection(d, n)
	case "o":
		return c.handler.handleOrderbook(d, n)
	case "n":
		return c.handler.handleNews(d, n)
	case "p":
		return c.handler.handleFuturesPricing(d, n)
	case "subscription":
		return c.handleSubscriptionMessage(d, n)
	case msgTypeError:
		return c.handleErrorMessage(d, n)
	default:
		return c.handleOther(d, n)
	}
}

type stocksMsgHandler struct {
	mu                   sync.RWMutex
	tradeHandler         func(trade Trade)
	quoteHandler         func(quote Quote)
	barHandler           func(bar Bar)
	updatedBarHandler    func(bar Bar)
	dailyBarHandler      func(bar Bar)
	tradingStatusHandler func(ts TradingStatus)
	imbalanceHandler     func(ts Imbalance)
	luldHandler          func(luld LULD)
	cancelErrorHandler   func(tce TradeCancelError)
	correctionHandler    func(tc TradeCorrection)
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
		case "r":
			trade.internal.ReceivedAt, err = d.DecodeTime()
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
		case "r":
			quote.internal.ReceivedAt, err = d.DecodeTime()
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

func (h *stocksMsgHandler) handleUpdatedBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	updatedBarHandler := h.updatedBarHandler
	h.mu.RUnlock()
	updatedBarHandler(bar)
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

func (h *stocksMsgHandler) handleImbalance(d *msgpack.Decoder, n int) error {
	oi := Imbalance{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			oi.Symbol, err = d.DecodeString()
		case "p":
			oi.Price, err = d.DecodeFloat64()
		case "t":
			oi.Timestamp, err = d.DecodeTime()
		case "z":
			oi.Tape, err = d.DecodeString()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	handler := h.imbalanceHandler
	h.mu.RUnlock()
	handler(oi)
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

func (h *stocksMsgHandler) handleCancelError(d *msgpack.Decoder, n int) error {
	tce := TradeCancelError{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			tce.Symbol, err = d.DecodeString()
		case "i":
			tce.ID, err = d.DecodeInt64()
		case "x":
			tce.Exchange, err = d.DecodeString()
		case "p":
			tce.Price, err = d.DecodeFloat64()
		case "s":
			tce.Size, err = d.DecodeUint32()
		case "a":
			tce.CancelErrorAction, err = d.DecodeString()
		case "z":
			tce.Tape, err = d.DecodeString()
		case "t":
			tce.Timestamp, err = d.DecodeTime()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	handler := h.cancelErrorHandler
	h.mu.RUnlock()
	handler(tce)
	return nil
}

func (h *stocksMsgHandler) handleCorrection(d *msgpack.Decoder, n int) error {
	tc := TradeCorrection{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			tc.Symbol, err = d.DecodeString()
		case "x":
			tc.Exchange, err = d.DecodeString()
		case "oi":
			tc.OriginalID, err = d.DecodeInt64()
		case "op":
			tc.OriginalPrice, err = d.DecodeFloat64()
		case "os":
			tc.OriginalSize, err = d.DecodeUint32()
		case "oc":
			tc.OriginalConditions, err = decodeStringSlice(d)
		case "ci":
			tc.CorrectedID, err = d.DecodeInt64()
		case "cp":
			tc.CorrectedPrice, err = d.DecodeFloat64()
		case "cs":
			tc.CorrectedSize, err = d.DecodeUint32()
		case "cc":
			tc.CorrectedConditions, err = decodeStringSlice(d)
		case "z":
			tc.Tape, err = d.DecodeString()
		case "t":
			tc.Timestamp, err = d.DecodeTime()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	handler := h.correctionHandler
	h.mu.RUnlock()
	handler(tc)
	return nil
}

func (h *stocksMsgHandler) handleOrderbook(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *stocksMsgHandler) handleNews(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *stocksMsgHandler) handleFuturesPricing(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

type cryptoMsgHandler struct {
	mu                    sync.RWMutex
	tradeHandler          func(CryptoTrade)
	quoteHandler          func(CryptoQuote)
	barHandler            func(CryptoBar)
	updatedBarHandler     func(CryptoBar)
	dailyBarHandler       func(CryptoBar)
	orderbookHandler      func(CryptoOrderbook)
	futuresPricingHandler func(CryptoPerpPricing)
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
			trade.ID, err = d.DecodeInt64()
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

func (h *cryptoMsgHandler) handleFuturesPricing(d *msgpack.Decoder, n int) error {
	pricing := CryptoPerpPricing{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			pricing.Symbol, err = d.DecodeString()
		case "x":
			pricing.Exchange, err = d.DecodeString()
		case "ip":
			pricing.IndexPrice, err = d.DecodeFloat64()
		case "mp":
			pricing.MarkPrice, err = d.DecodeFloat64()
		case "fr":
			pricing.FundingRate, err = d.DecodeFloat64()
		case "oi":
			pricing.OpenInterest, err = d.DecodeFloat64()
		case "t":
			pricing.Timestamp, err = d.DecodeTime()
		case "ft":
			pricing.NextFundingTime, err = d.DecodeTime()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	pricingHandler := h.futuresPricingHandler
	h.mu.RUnlock()
	pricingHandler(pricing)
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

func (h *cryptoMsgHandler) handleUpdatedBar(d *msgpack.Decoder, n int) error {
	bar, err := h.decodeBar(d, n)
	if err != nil {
		return err
	}
	h.mu.RLock()
	updatedBarHandler := h.updatedBarHandler
	h.mu.RUnlock()
	updatedBarHandler(bar)
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

func (h *cryptoMsgHandler) handleOrderbook(d *msgpack.Decoder, n int) error {
	orderbook := CryptoOrderbook{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "S":
			orderbook.Symbol, err = d.DecodeString()
		case "x":
			orderbook.Exchange, err = d.DecodeString()
		case "t":
			orderbook.Timestamp, err = d.DecodeTime()
		case "b":
			orderbook.Bids, err = decodeCryptoOrderbookEntrySlice(d)
		case "a":
			orderbook.Asks, err = decodeCryptoOrderbookEntrySlice(d)
		case "r":
			orderbook.Reset, err = d.DecodeBool()
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	orderbookHandler := h.orderbookHandler
	h.mu.RUnlock()
	orderbookHandler(orderbook)
	return nil
}

func (h *cryptoMsgHandler) handleTradingStatus(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleImbalance(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleLULD(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleCancelError(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleCorrection(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *cryptoMsgHandler) handleNews(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

type optionsMsgHandler struct {
	mu           sync.RWMutex
	tradeHandler func(trade OptionTrade)
	quoteHandler func(quote OptionQuote)
}

var _ msgHandler = (*optionsMsgHandler)(nil)

func (h *optionsMsgHandler) handleTrade(d *msgpack.Decoder, n int) error {
	trade := OptionTrade{}
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
			trade.Size, err = d.DecodeUint32()
		case "t":
			trade.Timestamp, err = d.DecodeTime()
		case "c":
			trade.Condition, err = d.DecodeString()
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

func (h *optionsMsgHandler) handleQuote(d *msgpack.Decoder, n int) error {
	quote := OptionQuote{}
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
			quote.Condition, err = d.DecodeString()
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

func (h *optionsMsgHandler) handleBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleUpdatedBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleDailyBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleTradingStatus(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleImbalance(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleLULD(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleCancelError(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleCorrection(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleOrderbook(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleNews(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *optionsMsgHandler) handleFuturesPricing(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

type newsMsgHandler struct {
	mu          sync.RWMutex
	newsHandler func(news News)
}

var _ msgHandler = (*newsMsgHandler)(nil)

func (h *newsMsgHandler) handleTrade(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleQuote(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleUpdatedBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleDailyBar(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleTradingStatus(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleImbalance(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleLULD(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleCancelError(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleCorrection(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleOrderbook(d *msgpack.Decoder, n int) error {
	// should not happen!
	return discardMapContents(d, n)
}

func (h *newsMsgHandler) handleNews(d *msgpack.Decoder, n int) error {
	news := News{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
		}
		switch key {
		case "id":
			news.ID, err = d.DecodeInt()
		case "headline":
			news.Headline, err = d.DecodeString()
		case "summary":
			news.Summary, err = d.DecodeString()
		case "author":
			news.Author, err = d.DecodeString()
		case "content":
			news.Content, err = d.DecodeString()
		case "url":
			news.URL, err = d.DecodeString()
		case "created_at":
			news.CreatedAt, err = d.DecodeTime()
		case "updated_at":
			news.UpdatedAt, err = d.DecodeTime()
		case "symbols":
			news.Symbols, err = decodeStringSlice(d)
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	h.mu.RLock()
	newsHandler := h.newsHandler
	h.mu.RUnlock()
	newsHandler(news)
	return nil
}

func (h *newsMsgHandler) handleFuturesPricing(d *msgpack.Decoder, n int) error {
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
	c.sub.updatedBars = s.updatedBars
	c.sub.dailyBars = s.dailyBars
	c.sub.statuses = s.statuses
	c.sub.imbalances = s.imbalances
	c.sub.lulds = s.lulds
	c.sub.cancelErrors = s.cancelErrors
	c.sub.corrections = s.corrections
	c.sub.orderbooks = s.orderbooks
	c.sub.news = s.news
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
		case "updatedBars":
			s.updatedBars, err = decodeStringSlice(d)
		case "dailyBars":
			s.dailyBars, err = decodeStringSlice(d)
		case "statuses":
			s.statuses, err = decodeStringSlice(d)
		case "imbalances":
			s.imbalances, err = decodeStringSlice(d)
		case "lulds":
			s.lulds, err = decodeStringSlice(d)
		case "cancelErrors":
			s.cancelErrors, err = decodeStringSlice(d)
		case "corrections":
			s.corrections, err = decodeStringSlice(d)
		case "orderbooks":
			s.orderbooks, err = decodeStringSlice(d)
		case "news":
			s.news, err = decodeStringSlice(d)
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
		s, err := d.DecodeString()
		if err != nil {
			return nil, err
		}
		res[i] = s
	}
	return res, nil
}

func decodeCryptoOrderbookEntrySlice(d *msgpack.Decoder) ([]CryptoOrderbookEntry, error) {
	var length int
	var err error
	if length, err = d.DecodeArrayLen(); err != nil {
		return nil, err
	}
	if length < 0 {
		return []CryptoOrderbookEntry{}, nil
	}
	res := make([]CryptoOrderbookEntry, length)
	for i := 0; i < length; i++ {
		e, err := decodeCryptoOrderbookEntry(d)
		if err != nil {
			return nil, err
		}
		res[i] = e
	}
	return res, nil
}

func decodeCryptoOrderbookEntry(d *msgpack.Decoder) (CryptoOrderbookEntry, error) {
	var entry CryptoOrderbookEntry
	var err error
	n, err := d.DecodeMapLen()
	if err != nil {
		return entry, err
	}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return entry, err
		}
		switch key {
		case "p":
			entry.Price, err = d.DecodeFloat64()
		case "s":
			entry.Size, err = d.DecodeFloat64()
		default:
			err = d.Skip()
		}
		if err != nil {
			return entry, err
		}
	}
	return entry, err
}
