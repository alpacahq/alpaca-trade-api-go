package alpaca

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompatibilityOperationsCloseSourceResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		call   func(*Client) error
	}{
		{
			name:   "cancel order",
			status: http.StatusNoContent,
			body:   "ignored response",
			call: func(client *Client) error {
				return client.CancelOrder("order-id")
			},
		},
		{
			name:   "cancel all orders",
			status: http.StatusOK,
			body:   "ignored response",
			call: func(client *Client) error {
				return client.CancelAllOrders()
			},
		},
		{
			name:   "terminal option contracts page",
			status: http.StatusOK,
			body:   `{"option_contracts":[{"id":"contract-id","symbol":"AAPL271217C00150000"}],"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetOptionContracts(GetOptionContractsRequest{
					UnderlyingSymbols: "AAPL",
					TotalLimit:        1,
				})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := &scriptedTransport{
				t:        t,
				statuses: []int{test.status},
				bodies:   []string{test.body},
			}
			client := newScriptedClient(t, transport, -1)
			require.NoError(t, test.call(client))
			require.Len(t, transport.responses, 1)
			assertClosedAfterDrain(t, transport.responses[0])
		})
	}
}

func TestOptionContractsCloseEveryPaginatedResponse(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusOK},
		bodies: []string{
			`{"option_contracts":[{"id":"contract-1","symbol":"AAPL271217C00150000"}],"next_page_token":"next"}`,
			`{"option_contracts":[{"id":"contract-2","symbol":"AAPL271217C00160000"}],"next_page_token":null}`,
		},
	}
	client := newScriptedClient(t, transport, -1)
	values, err := client.GetOptionContracts(GetOptionContractsRequest{
		UnderlyingSymbols: "AAPL",
		PageLimit:         1,
	})
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Len(t, transport.responses, 2)
	require.True(t, transport.closedBeforeCall[1],
		"first page body must be closed before requesting the second page")
	for index, response := range transport.responses {
		assertClosedAfterDrain(t, response, "response %d", index)
	}
}

func TestOptionContractsCloseResponseBeforeSecondPageError(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusUnprocessableEntity},
		bodies: []string{
			`{"option_contracts":[{"id":"contract-1","symbol":"AAPL271217C00150000"}],"next_page_token":"next"}`,
			`{"code":42210000,"message":"second page rejected"}`,
		},
	}
	client := newScriptedClient(t, transport, -1)
	_, err := client.GetOptionContracts(GetOptionContractsRequest{
		UnderlyingSymbols: "AAPL",
		PageLimit:         1,
	})
	require.Error(t, err)
	require.Len(t, transport.responses, 2)
	require.True(t, transport.closedBeforeCall[1],
		"first page body must be closed before requesting the failing second page")
	for index, response := range transport.responses {
		assertClosedAfterDrain(t, response, "response %d", index)
	}
}

func TestOptionContractsCloseMalformedPageResponse(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK},
		bodies:   []string{`{"option_contracts":[`},
	}
	client := newScriptedClient(t, transport, -1)
	_, err := client.GetOptionContracts(GetOptionContractsRequest{
		UnderlyingSymbols: "AAPL",
	})
	require.Error(t, err)
	require.Len(t, transport.responses, 1)
	assertClosedAfterDrain(t, transport.responses[0])
}

func TestCompressedTradingAPIErrorClosesSourceResponseOnce(t *testing.T) {
	body := &trackingBody{
		r: strings.NewReader(string(gzipTradingPayload(
			t,
			`{"code":42210000,"message":"compressed rejection"}`,
		))),
	}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: alpacaRoundTripperFunc(
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
					Status:     http.StatusText(http.StatusUnprocessableEntity),
					Header: http.Header{
						"Content-Encoding": {"gzip"},
						"Content-Type":     {"application/json"},
						"X-Request-Id":     {"compressed-request"},
					},
					Body:    body,
					Request: request,
				}, nil
			},
		)},
	})
	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	_, err = defaultDo(client, request)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 42210000, apiErr.Code)
	assert.Equal(t, "compressed rejection", apiErr.Message)
	assert.Equal(t, "compressed-request", apiErr.RequestID)
	assert.Equal(t, 1, body.closeCalls)
}

func TestMalformedGzipClosesTradingSourceResponseOnce(t *testing.T) {
	body := &trackingBody{r: strings.NewReader("not gzip")}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: alpacaRoundTripperFunc(
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Encoding": {"gzip"}},
					Body:       body,
					Request:    request,
				}, nil
			},
		)},
	})
	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	_, err = defaultDo(client, request)
	require.Error(t, err)
	assert.Equal(t, 1, body.closeCalls)
}

func TestMalformedGzipTradingAPIErrorPreservesMetadata(t *testing.T) {
	body := &trackingBody{r: strings.NewReader("not a gzip payload")}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: alpacaRoundTripperFunc(
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
					Header: http.Header{
						"Content-Encoding": {"gzip"},
						"X-Request-Id":     {"malformed-request"},
					},
					Body:    body,
					Request: request,
				}, nil
			},
		)},
	})
	request, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	_, err = defaultDo(client, request)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.ErrorIs(t, err, gzip.ErrHeader)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Equal(t, "malformed-request", apiErr.RequestID)
	assert.Equal(t, 1, body.closeCalls)
}

func TestPlaceOrderDoesNotRestoreCredentialsAfterCrossOrigin307Bounce(t *testing.T) {
	tests := []struct {
		name string
		opts func(baseURL string) ClientOpts
	}{
		{
			name: "API key credentials",
			opts: func(baseURL string) ClientOpts {
				return ClientOpts{
					APIKey:    "test-key",
					APISecret: "test-secret",
					BaseURL:   baseURL,
				}
			},
		},
		{
			name: "OAuth bearer credentials",
			opts: func(baseURL string) ClientOpts {
				return ClientOpts{
					OAuth:   "test-oauth-token",
					BaseURL: baseURL,
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			var originHeaders []http.Header
			var foreignHeaders []http.Header
			var bouncedBody string

			var originServer *httptest.Server
			foreignServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				mu.Lock()
				foreignHeaders = append(foreignHeaders, r.Header.Clone())
				mu.Unlock()
				http.Redirect(w, r, originServer.URL+"/v2/orders", http.StatusTemporaryRedirect)
			}))
			defer foreignServer.Close()

			originServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read origin body: %v", err)
					return
				}
				mu.Lock()
				callIndex := len(originHeaders)
				originHeaders = append(originHeaders, r.Header.Clone())
				if callIndex == 1 {
					bouncedBody = string(bodyBytes)
				}
				mu.Unlock()
				if callIndex == 0 {
					http.Redirect(w, r, foreignServer.URL+"/bounce", http.StatusTemporaryRedirect)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":40110000,"message":"request is not authorized"}`))
			}))
			defer originServer.Close()

			qty := decimal.NewFromInt(1)
			client := NewClient(test.opts(originServer.URL))
			_, err := client.PlaceOrder(PlaceOrderRequest{
				Symbol:      "AAPL",
				Qty:         &qty,
				Side:        Buy,
				Type:        Market,
				TimeInForce: Day,
			})
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)

			mu.Lock()
			defer mu.Unlock()
			require.Len(t, foreignHeaders, 1)
			require.Len(t, originHeaders, 2)
			assert.Contains(t, bouncedBody, `"symbol":"AAPL"`)
			for _, headers := range []http.Header{foreignHeaders[0], originHeaders[1]} {
				assert.Empty(t, headers.Get("APCA-API-KEY-ID"))
				assert.Empty(t, headers.Get("APCA-API-SECRET-KEY"))
				assert.Empty(t, headers.Get("Authorization"))
			}
		})
	}
}

type alpacaRoundTripperFunc func(*http.Request) (*http.Response, error)

func (function alpacaRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func gzipTradingPayload(t *testing.T, value string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(value))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
