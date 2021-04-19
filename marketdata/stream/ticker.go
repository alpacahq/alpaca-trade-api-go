package stream

import "time"

type ticker interface {
	C() <-chan time.Time
	Stop()
}

type timeTicker struct {
	ticker *time.Ticker
}

var _ ticker = (*timeTicker)(nil)

func (t *timeTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t *timeTicker) Stop() {
	t.ticker.Stop()
}
