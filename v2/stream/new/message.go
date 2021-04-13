package new

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

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
			err = c.handleTrade(d, n)
		case "q":
			err = c.handleQuote(d, n)
		case "b":
			err = c.handleBar(d, n)
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

func (c *client) handleTrade(d *msgpack.Decoder, n int) error {
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
	c.handlerMutex.RLock()
	tradeHandler := c.tradeHandler
	c.handlerMutex.RUnlock()
	tradeHandler(trade)
	return nil
}

func (c *client) handleQuote(d *msgpack.Decoder, n int) error {
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
	c.handlerMutex.RLock()
	quoteHandler := c.quoteHandler
	c.handlerMutex.RUnlock()
	quoteHandler(quote)
	return nil
}

func (c *client) handleBar(d *msgpack.Decoder, n int) error {
	bar := Bar{}
	for i := 0; i < n; i++ {
		key, err := d.DecodeString()
		if err != nil {
			return err
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
		default:
			err = d.Skip()
		}
		if err != nil {
			return err
		}
	}
	c.handlerMutex.RLock()
	barHandler := c.barHandler
	defer c.handlerMutex.RUnlock()
	barHandler(bar)
	return nil
}

//ErrSymbolLimitExceeded is returned when the client has subscribed to too many symbols
var ErrSymbolLimitExceeded = errors.New("symbol limit exceeded")

//ErrSlowClient is returned when the server has detected a slow client. In this case there's no guarantee
// that all prior messages are sent to the server so a subscription acknowledgement may not arrive
var ErrSlowClient = errors.New("slow client")

var errMessageHandler = func(c *client, e errorMessage) error {
	// {"T":"error","code":405,"msg":"symbol limit exceeded"}
	// {"T":"error","code":407,"msg":"slow client"}
	switch e.code {
	case 405, 407:
		c.pendingSubChangeMutex.Lock()
		defer c.pendingSubChangeMutex.Unlock()
		if c.pendingSubChange != nil {
			switch e.code {
			case 405:
				c.pendingSubChange.result <- ErrSymbolLimitExceeded
			case 407:
				c.pendingSubChange.result <- ErrSlowClient
			}
			c.pendingSubChange = nil
		}
		return nil
	}

	return fmt.Errorf("datav2stream: received unexpected error: %s", e.msg)
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

var subMessageHandler = func(c *client, s subscriptionMessage) error {
	c.pendingSubChangeMutex.Lock()
	defer c.pendingSubChangeMutex.Unlock()
	c.trades = s.trades
	c.quotes = s.quotes
	c.bars = s.bars
	if c.pendingSubChange != nil {
		c.handlerMutex.Lock()
		defer c.handlerMutex.Unlock()
		psc := c.pendingSubChange
		switch psc.handlerToChange {
		case tradeHandler:
			c.tradeHandler = *psc.tradeHandler
		case quoteHandler:
			c.quoteHandler = *psc.quoteHandler
		case barHandler:
			c.barHandler = *psc.barHandler
		}
		psc.result <- nil
		c.pendingSubChange = nil
	}

	return nil
}

func (c *client) handleSubscriptionMessage(d *msgpack.Decoder, n int) error {
	s := subscriptionMessage{}
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
