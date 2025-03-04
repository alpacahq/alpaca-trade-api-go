package alpaca

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Benchmark for GetAccount endpoint.
func BenchmarkGetAccount(b *testing.B) {
	account := Account{
		ID:            "bench_id",
		AccountNumber: "123456789",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(account)
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{
		BaseURL:    ts.URL,
		RetryLimit: 0,
		RetryDelay: 0,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetAccount()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark for the defaultDo function.
func BenchmarkDefaultDo(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bench test body"))
	}))
	defer ts.Close()

	client := NewClient(ClientOpts{
		BaseURL:    ts.URL,
		RetryLimit: 0,
		RetryDelay: 0,
	})
	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := defaultDo(client, req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
