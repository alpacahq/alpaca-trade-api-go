package httptransport

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloneWithSafeRedirectsStripsCrossOriginCredentials(t *testing.T) {
	base := &http.Client{
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			request.Header.Set("APCA-API-KEY-ID", "restored-by-callback")
			request.Header.Set("Authorization", "restored-by-callback")
			request.Header["authorization"] = []string{"lowercase secret"}
			request.Header["aPcA-aPi-SeCrEt-KeY"] = []string{"mixed-case secret"}
			return nil
		},
	}
	client := CloneWithSafeRedirects(base)
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "https://example.com/redirect")
	setCredentials(redirect)

	if err := client.CheckRedirect(redirect, []*http.Request{initial}); err != nil {
		t.Fatal(err)
	}
	assertNoCredentialHeaders(t, redirect)
	if base.CheckRedirect == nil {
		t.Fatal("base client was modified")
	}
}

func TestCloneWithSafeRedirectsPreservesSameOriginCredentials(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "https://api.alpaca.markets/v2/clock")
	setCredentials(redirect)

	if err := client.CheckRedirect(redirect, []*http.Request{initial}); err != nil {
		t.Fatal(err)
	}
	for _, header := range credentialHeaders() {
		if value := redirect.Header.Get(header); value == "" {
			t.Fatalf("%s was unexpectedly removed", header)
		}
	}
}

func TestCloneWithSafeRedirectsNormalizesDefaultPorts(t *testing.T) {
	tests := []struct {
		name     string
		initial  string
		redirect string
	}{
		{
			name:     "implicit to explicit HTTPS",
			initial:  "https://api.alpaca.markets/v2/account",
			redirect: "https://api.alpaca.markets:443/v2/clock",
		},
		{
			name:     "explicit to implicit HTTPS",
			initial:  "https://api.alpaca.markets:443/v2/account",
			redirect: "https://api.alpaca.markets/v2/clock",
		},
		{
			name:     "implicit to explicit HTTP",
			initial:  "http://localhost/v2/account",
			redirect: "http://localhost:80/v2/clock",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := CloneWithSafeRedirects(&http.Client{})
			initial := redirectRequest(t, test.initial)
			redirect := redirectRequest(t, test.redirect)
			setCredentials(redirect)

			if err := client.CheckRedirect(redirect, []*http.Request{initial}); err != nil {
				t.Fatal(err)
			}
			for _, header := range credentialHeaders() {
				if value := redirect.Header.Get(header); value == "" {
					t.Fatalf("%s was unexpectedly removed", header)
				}
			}
		})
	}
}

func TestCloneWithSafeRedirectsTreatsNonDefaultPortAsDifferentOrigin(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "https://api.alpaca.markets:8443/v2/clock")
	setCredentials(redirect)

	if err := client.CheckRedirect(redirect, []*http.Request{initial}); err != nil {
		t.Fatal(err)
	}
	assertNoCredentialHeaders(t, redirect)
}

func TestCloneWithSafeRedirectsRejectsHTTPSDowngrade(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "http://api.alpaca.markets/v2/account")
	setCredentials(redirect)

	err := client.CheckRedirect(redirect, []*http.Request{initial})
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("error = %v, want %v", err, ErrInsecureRedirect)
	}
	assertNoCredentialHeaders(t, redirect)
}

func TestCloneWithSafeRedirectsPreservesCustomStopDecision(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "http://api.alpaca.markets/v2/account")

	err := client.CheckRedirect(redirect, []*http.Request{initial})
	if !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("error = %v, want %v", err, http.ErrUseLastResponse)
	}
}

func TestCloneWithSafeRedirectsRejectsDowngradeFromPreviousHop(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	initial := redirectRequest(t, "http://api.alpaca.markets/v2/account")
	secureHop := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "http://api.alpaca.markets/v2/account")

	err := client.CheckRedirect(redirect, []*http.Request{initial, secureHop})
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("error = %v, want %v", err, ErrInsecureRedirect)
	}
}

func TestCloneWithSafeRedirectsKeepsCredentialsOffForeignRedirectChain(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	crossOriginHop := redirectRequest(t, "https://example.com/redirect")
	redirect := redirectRequest(t, "https://example.com/another-redirect")
	setCredentials(redirect)

	if err := client.CheckRedirect(redirect, []*http.Request{initial, crossOriginHop}); err != nil {
		t.Fatal(err)
	}
	assertNoCredentialHeaders(t, redirect)
}

func TestCloneWithSafeRedirectsRechecksCallbackMutation(t *testing.T) {
	base := &http.Client{
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			mutated := redirectRequest(t, "http://example.com/redirect").URL
			via[0].URL = mutated
			request.URL = mutated
			setCredentials(request)
			request.Header["authorization"] = []string{"lowercase secret"}
			request.Header["aPcA-aPi-KeY-iD"] = []string{"mixed-case secret"}
			return nil
		},
	}
	client := CloneWithSafeRedirects(base)
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	redirect := redirectRequest(t, "https://api.alpaca.markets/v2/clock")
	setCredentials(redirect)

	err := client.CheckRedirect(redirect, []*http.Request{initial})
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("error = %v, want %v", err, ErrInsecureRedirect)
	}
	assertNoCredentialHeaders(t, redirect)
}

func TestCloneWithSafeRedirectsPreservesDefaultLimit(t *testing.T) {
	client := CloneWithSafeRedirects(&http.Client{})
	redirect := redirectRequest(t, "https://api.alpaca.markets/redirect")
	via := make([]*http.Request, 10)
	for index := range via {
		via[index] = redirectRequest(t, "https://api.alpaca.markets/redirect")
	}
	if err := client.CheckRedirect(redirect, via); err == nil {
		t.Fatal("expected redirect limit error")
	}
}

func TestCloneWithSafeRedirectsSnapshotsClientConfiguration(t *testing.T) {
	var originalCalls atomic.Int32
	var replacementCalls atomic.Int32
	base := &http.Client{
		Timeout: time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			originalCalls.Add(1)
			return nil
		},
	}
	client := CloneWithSafeRedirects(base)
	base.Timeout = 2 * time.Second
	base.CheckRedirect = func(*http.Request, []*http.Request) error {
		replacementCalls.Add(1)
		return nil
	}
	initial := redirectRequest(t, "https://api.alpaca.markets/first")
	redirect := redirectRequest(t, "https://api.alpaca.markets/second")
	if err := client.CheckRedirect(redirect, []*http.Request{initial}); err != nil {
		t.Fatal(err)
	}
	if client.Timeout != time.Second || originalCalls.Load() != 1 || replacementCalls.Load() != 0 {
		t.Fatalf(
			"timeout=%s original calls=%d replacement calls=%d",
			client.Timeout,
			originalCalls.Load(),
			replacementCalls.Load(),
		)
	}
}

func TestCloneWithSafeRedirectsDoesNotSendCredentialsToRedirectTarget(t *testing.T) {
	var leaked atomic.Bool
	var target *httptest.Server
	target = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		for _, header := range credentialHeaders() {
			if request.Header.Get(header) != "" {
				leaked.Store(true)
			}
		}
		if request.URL.Path == "/first" {
			http.Redirect(response, request, target.URL+"/second", http.StatusFound)
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, target.URL+"/first", http.StatusFound)
	}))
	defer source.Close()

	request, err := http.NewRequest(http.MethodGet, source.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	setCredentials(request)
	response, err := CloneWithSafeRedirects(&http.Client{}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Body != nil {
		_ = response.Body.Close()
	}
	if leaked.Load() {
		t.Fatal("redirect target received credentials")
	}
}

func TestCloneWithSafeRedirectsDoesNotRestoreCredentialsAfterOriginBounce(t *testing.T) {
	type capturedRequest struct {
		method  string
		path    string
		body    string
		headers http.Header
	}

	var mu sync.Mutex
	var originRequests []capturedRequest
	var foreignRequests []capturedRequest

	var originServer *httptest.Server
	foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read foreign body: %v", err)
			return
		}
		mu.Lock()
		foreignRequests = append(foreignRequests, capturedRequest{
			method:  r.Method,
			path:    r.URL.Path,
			body:    string(bodyBytes),
			headers: r.Header.Clone(),
		})
		mu.Unlock()
		http.Redirect(w, r, originServer.URL+"/v2/orders?bounced=1", http.StatusTemporaryRedirect)
	}))
	defer foreignServer.Close()

	originServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read origin body: %v", err)
			return
		}
		mu.Lock()
		callIndex := len(originRequests)
		originRequests = append(originRequests, capturedRequest{
			method:  r.Method,
			path:    r.URL.RequestURI(),
			body:    string(bodyBytes),
			headers: r.Header.Clone(),
		})
		mu.Unlock()
		if callIndex == 0 {
			http.Redirect(w, r, foreignServer.URL+"/bounce", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer originServer.Close()

	const payload = `{"symbol":"AAPL","qty":"1","side":"buy","type":"market","time_in_force":"day"}`
	req, err := http.NewRequest(http.MethodPost, originServer.URL+"/v2/orders", bytes.NewReader([]byte(payload)))
	if err != nil {
		t.Fatal(err)
	}
	setCredentials(req)

	resp, err := CloneWithSafeRedirects(&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	mu.Lock()
	defer mu.Unlock()

	if len(originRequests) != 2 {
		t.Fatalf("origin requests = %d, want 2", len(originRequests))
	}
	if len(foreignRequests) != 1 {
		t.Fatalf("foreign requests = %d, want 1", len(foreignRequests))
	}
	if originRequests[0].headers.Get("APCA-API-KEY-ID") == "" {
		t.Fatal("initial origin request did not receive API key")
	}
	assertNoCredentialHeadersInHeader(t, "foreign hop", foreignRequests[0].headers)
	if foreignRequests[0].method != http.MethodPost || foreignRequests[0].body != payload {
		t.Fatalf("foreign hop = (%s, %q), want (POST, %q)", foreignRequests[0].method, foreignRequests[0].body, payload)
	}
	if originRequests[1].method != http.MethodPost || originRequests[1].body != payload {
		t.Fatalf("bounced hop = (%s, %q), want (POST, %q)", originRequests[1].method, originRequests[1].body, payload)
	}
	assertNoCredentialHeadersInHeader(t, "bounced origin hop", originRequests[1].headers)
}

func TestCloneWithSafeRedirectsKeepsCredentialsStrippedAfterSubdomainBounce(t *testing.T) {
	type captured struct {
		url     string
		headers http.Header
	}
	var hops []captured
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		hops = append(hops, captured{
			url:     r.URL.String(),
			headers: r.Header.Clone(),
		})
		switch len(hops) {
		case 1:
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": {"https://evil.api.alpaca.markets/bounce"}},
				Body:       http.NoBody,
				Request:    r,
			}, nil
		case 2:
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": {"https://api.alpaca.markets/v2/orders"}},
				Body:       http.NoBody,
				Request:    r,
			}, nil
		case 3:
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": {"https://api.alpaca.markets/v2/account"}},
				Body:       http.NoBody,
				Request:    r,
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
				Request:    r,
			}, nil
		}
	})

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.alpaca.markets/v2/orders",
		bytes.NewReader([]byte(`{"symbol":"AAPL"}`)),
	)
	if err != nil {
		t.Fatal(err)
	}
	setCredentials(req)

	resp, err := CloneWithSafeRedirects(&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	if len(hops) != 4 {
		t.Fatalf("hops = %d, want 4", len(hops))
	}
	for _, header := range credentialHeaders() {
		if hops[0].headers.Get(header) == "" {
			t.Fatalf("initial hop missing %s", header)
		}
	}
	assertNoCredentialHeadersInHeader(t, "subdomain hop", hops[1].headers)
	assertNoCredentialHeadersInHeader(t, "bounced origin hop", hops[2].headers)
	assertNoCredentialHeadersInHeader(t, "subsequent same-origin hop", hops[3].headers)
}

func TestCloneWithSafeRedirectsPreventsCallbackHidingCrossOriginHop(t *testing.T) {
	initial := redirectRequest(t, "https://api.alpaca.markets/v2/account")
	crossOriginHop := redirectRequest(t, "https://evil.example/bounce")
	returned := redirectRequest(t, "https://api.alpaca.markets/v2/orders")
	setCredentials(returned)

	base := &http.Client{
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			via[1].URL = initial.URL
			setCredentials(request)
			return nil
		},
	}
	client := CloneWithSafeRedirects(base)
	if err := client.CheckRedirect(returned, []*http.Request{initial, crossOriginHop}); err != nil {
		t.Fatal(err)
	}
	assertNoCredentialHeaders(t, returned)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func redirectRequest(t *testing.T, value string) *http.Request {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Request{URL: parsed, Header: http.Header{}}
}

func setCredentials(request *http.Request) {
	for _, header := range credentialHeaders() {
		request.Header.Set(header, "secret")
	}
}

func credentialHeaders() []string {
	return []string{
		"APCA-API-KEY-ID",
		"APCA-API-SECRET-KEY",
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Cookie2",
	}
}

func assertNoCredentialHeaders(t *testing.T, request *http.Request) {
	t.Helper()
	assertNoCredentialHeadersInHeader(t, "redirect", request.Header)
}

func assertNoCredentialHeadersInHeader(t *testing.T, label string, headers http.Header) {
	t.Helper()
	for actual := range headers {
		for _, sensitive := range credentialHeaders() {
			if strings.EqualFold(actual, sensitive) {
				t.Fatalf("%s retained %s after redirect safety policy", label, actual)
			}
		}
	}
}
