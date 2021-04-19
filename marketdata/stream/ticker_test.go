package stream

import "time"

type testTicker struct {
	ch chan time.Time
}

var _ ticker = (*testTicker)(nil)

func (t *testTicker) C() <-chan time.Time {
	return t.ch
}

func (t *testTicker) Stop() {
}

func (t *testTicker) Tick() {
	t.ch <- time.Now()
}

func newTestTicker() *testTicker {
	return &testTicker{ch: make(chan time.Time)}
}
