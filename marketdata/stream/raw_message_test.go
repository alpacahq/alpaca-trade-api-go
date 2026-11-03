package stream

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

func TestRawMessageHandlerReceivesOwnedExactFrameBeforeBuffer(t *testing.T) {
	connection := newMockConn()
	client := newClient()
	client.conn = connection
	client.in = make(chan []byte, 1)
	captured := make(chan []byte, 1)
	client.rawMessageHandler = func(frame []byte) { captured <- frame }

	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	workers.Add(1)
	closeCh := make(chan struct{})
	go client.connReader(ctx, &workers, closeCh)

	want := []byte{0x91, 0x81, 0xa1, 'T', 0xa1, 'q'}
	connection.readCh <- want
	processed := <-client.in
	got := <-captured
	processed[0] = 0
	if !bytes.Equal(got, []byte{0x91, 0x81, 0xa1, 'T', 0xa1, 'q'}) {
		t.Fatalf("raw callback frame changed: %x", got)
	}

	cancel()
	select {
	case <-closeCh:
	case <-time.After(time.Second):
		t.Fatal("reader did not stop")
	}
	workers.Wait()
}
