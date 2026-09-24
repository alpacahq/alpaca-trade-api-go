package alpaca

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
