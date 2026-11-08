package alpaca

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamTradeUpdates(t *testing.T) {
	nextMsg := make(chan struct{}, 10)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2023-01-20T01:02:03.456789Z", r.URL.Query().Get("since"))
		flusher, err := w.(http.Flusher)
		if !err {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, `data: {"execution_id":"first"}`+"\n\n")
		flusher.Flush()
		<-nextMsg

		fmt.Fprintf(w, `data: {"execution_id":"second"}`+"\n\n")
		flusher.Flush()
		<-nextMsg
	}))

	c := NewClient(
		ClientOpts{
			BaseURL: ts.URL,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	i := 0
	require.NoError(t, c.StreamTradeUpdates(ctx, func(tu TradeUpdate) {
		switch i {
		case 0:
			assert.Equal(t, "first", tu.ExecutionID)
			i++
			nextMsg <- struct{}{}
		case 1:
			assert.Equal(t, "second", tu.ExecutionID)
			i++
			nextMsg <- struct{}{}
		}
	}, StreamTradeUpdatesRequest{
		Since: time.Date(2023, 1, 20, 1, 2, 3, 456789000, time.UTC),
	}))
	require.NoError(t, ctx.Err())
}

func TestStreamTradeUpdates_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "req-stream")
		http.Error(w, `{"code":40110000,"message":"request is not authorized"}`, http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewClient(ClientOpts{BaseURL: ts.URL})
	err := c.StreamTradeUpdates(context.Background(), func(TradeUpdate) {}, StreamTradeUpdatesRequest{})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, 40110000, apiErr.Code)
	assert.Equal(t, "req-stream", apiErr.RequestID)
}

func TestStreamTradeUpdatesInBackground_ReconnectsAfterServerClose(t *testing.T) {
	var conns atomic.Int32
	reconnected := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if conns.Add(1) == 1 {
			// the server ends the stream cleanly after one update
			fmt.Fprint(w, `data: {"at":"2023-01-20T01:02:03.456789Z","event":"new"}`+"\n\n")
			return
		}
		select {
		case reconnected <- r.URL.Query().Get("since"):
		default:
		}
		<-r.Context().Done()
	}))
	defer ts.Close()

	c := NewClient(ClientOpts{BaseURL: ts.URL})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.StreamTradeUpdatesInBackground(ctx, func(TradeUpdate) {})

	select {
	case since := <-reconnected:
		assert.Equal(t, "2023-01-20T01:02:03.456789001Z", since)
	case <-time.After(2 * time.Second):
		t.Fatalf("stream did not reconnect after the server closed it (connections: %d)", conns.Load())
	}
}

type countingWriter struct{ n atomic.Int64 }

func (w *countingWriter) Write(p []byte) (int, error) {
	w.n.Add(1)
	return len(p), nil
}

func TestStreamTradeUpdatesInBackground_StopsOnDeadline(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer ts.Close()

	logs := &countingWriter{}
	log.SetOutput(logs)
	defer log.SetOutput(os.Stderr)

	c := NewClient(ClientOpts{BaseURL: ts.URL})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	c.StreamTradeUpdatesInBackground(ctx, func(TradeUpdate) {})

	<-ctx.Done()
	time.Sleep(300 * time.Millisecond)
	assert.Zero(t, logs.n.Load(), "kept retrying after the context deadline passed")
}
