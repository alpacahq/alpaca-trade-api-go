package new

import (
	"context"
	"errors"
	"sync"
)

var errClose = errors.New("closed")
var errPingDisabled = errors.New("ping disabled")

type mockConn struct {
	pingCh       chan struct{}
	closeCh      chan struct{}
	closeOnce    sync.Once
	readCh       chan []byte
	writeCh      chan []byte
	pingDisabled bool
}

var _ conn = (*mockConn)(nil)

func newMockConn() *mockConn {
	return &mockConn{
		pingCh:  make(chan struct{}, 10),
		closeCh: make(chan struct{}),
		readCh:  make(chan []byte, 10),
		writeCh: make(chan []byte, 10),
	}
}

func (c *mockConn) close() error {
	select {
	case <-c.closeCh:
	default:
		c.closeOnce.Do(func() {
			close(c.closeCh)
		})
	}
	return nil
}

func (c *mockConn) ping(ctx context.Context) error {
	if c.pingDisabled {
		return errPingDisabled
	}
	select {
	case <-c.closeCh:
		return errClose
	default:
	}
	c.pingCh <- struct{}{}
	return nil
}

func (c *mockConn) readMessage(ctx context.Context) (data []byte, err error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case data := <-c.readCh:
		return data, nil
	case <-c.closeCh:
		return nil, errClose
	}
}

func (c *mockConn) writeMessage(ctx context.Context, data []byte) error {
	select {
	case <-c.closeCh:
		return errClose
	default:
	}
	c.writeCh <- data
	return nil
}
