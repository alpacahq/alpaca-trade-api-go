package specsource

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestApprovedURLFailsClosed(t *testing.T) {
	for _, value := range []string{
		"http://docs.alpaca.markets/spec.json",
		"https://example.com/spec.json",
		"https://docs.alpaca.markets:443/spec.json",
		"https://user@docs.alpaca.markets/spec.json",
		"https://docs.alpaca.markets/spec.json?latest=true",
		"https://docs.alpaca.markets/spec.json#fragment",
	} {
		if _, err := ApprovedURL(value); err == nil {
			t.Fatalf("ApprovedURL(%q) unexpectedly passed", value)
		}
	}
}

func TestFetchPairRequiresIdenticalNonemptyBytes(t *testing.T) {
	responses := map[string]string{"/primary": "spec", "/redirect": "spec"}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(responses[request.URL.Path])),
			Header:     make(http.Header),
		}, nil
	})}
	pair, err := FetchPair(
		context.Background(),
		client,
		"https://docs.alpaca.markets/primary",
		"https://docs.alpaca.markets/redirect",
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(pair.Primary) != "spec" {
		t.Fatalf("primary = %q", pair.Primary)
	}
	responses["/redirect"] = "different"
	if _, err := FetchPair(context.Background(), client, "https://docs.alpaca.markets/primary", "https://docs.alpaca.markets/redirect"); err == nil {
		t.Fatal("mismatched URL forms unexpectedly passed")
	}
	responses["/primary"] = ""
	if _, err := Fetch(context.Background(), client, "https://docs.alpaca.markets/primary"); err == nil {
		t.Fatal("empty response unexpectedly passed")
	}
}
