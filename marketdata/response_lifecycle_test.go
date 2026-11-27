package marketdata

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
)

func TestPaginatedWorkflowsCloseTerminalResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		call func(*Client) error
	}{
		{
			name: "stock trades",
			body: `{"trades":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetMultiTrades([]string{"AAPL"}, GetTradesRequest{})
				return err
			},
		},
		{
			name: "stock quotes",
			body: `{"quotes":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetMultiQuotes([]string{"AAPL"}, GetQuotesRequest{})
				return err
			},
		},
		{
			name: "stock bars",
			body: `{"bars":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetMultiBars([]string{"AAPL"}, GetBarsRequest{})
				return err
			},
		},
		{
			name: "stock auctions",
			body: `{"auctions":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetMultiAuctions([]string{"AAPL"}, GetAuctionsRequest{})
				return err
			},
		},
		{
			name: "crypto trades",
			body: `{"trades":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetCryptoMultiTrades([]string{"BTC/USD"}, GetCryptoTradesRequest{})
				return err
			},
		},
		{
			name: "crypto quotes",
			body: `{"quotes":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetCryptoMultiQuotes([]string{"BTC/USD"}, GetCryptoQuotesRequest{})
				return err
			},
		},
		{
			name: "crypto bars",
			body: `{"bars":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetCryptoMultiBars([]string{"BTC/USD"}, GetCryptoBarsRequest{})
				return err
			},
		},
		{
			name: "news",
			body: `{"news":[],"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetNews(GetNewsRequest{})
				return err
			},
		},
		{
			name: "corporate actions",
			body: `{"corporate_actions":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetCorporateActions(GetCorporateActionsRequest{})
				return err
			},
		},
		{
			name: "option trades",
			body: `{"trades":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetOptionMultiTrades([]string{"AAPL271217C00150000"}, GetOptionTradesRequest{})
				return err
			},
		},
		{
			name: "option bars",
			body: `{"bars":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetMultiOptionBars([]string{"AAPL271217C00150000"}, GetOptionBarsRequest{})
				return err
			},
		},
		{
			name: "option snapshots",
			body: `{"snapshots":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetOptionSnapshots([]string{"AAPL271217C00150000"}, GetOptionSnapshotRequest{})
				return err
			},
		},
		{
			name: "option chain",
			body: `{"snapshots":{},"next_page_token":null}`,
			call: func(client *Client) error {
				_, err := client.GetOptionChain("AAPL", GetOptionChainRequest{})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := &scriptedTransport{
				t:        t,
				statuses: []int{http.StatusOK},
				bodies:   []string{test.body},
			}
			client := newScriptedClient(t, transport, -1)
			require.NoError(t, test.call(client))
			require.Len(t, transport.responses, 1)
			assertClosedAfterDrain(t, transport.responses[0])
		})
	}
}

func TestPaginatedWorkflowClosesEveryResponse(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusOK},
		bodies: []string{
			`{"bars":{"AAPL":[{"t":"2026-01-02T03:04:05Z","o":100,"h":103,` +
				`"l":99,"c":102,"v":40,"n":5,"vw":101.5}]},"next_page_token":"next"}`,
			`{"bars":{"AAPL":[{"t":"2026-01-03T03:04:05Z","o":101,"h":104,` +
				`"l":100,"c":103,"v":41,"n":6,"vw":102.5}]},"next_page_token":null}`,
		},
	}
	client := newScriptedClient(t, transport, -1)
	values, err := client.GetBars("AAPL", GetBarsRequest{
		TimeFrame: OneDay,
		PageLimit: 1,
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

func TestPaginatedWorkflowDecodesGzipAndClosesEveryResponse(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusOK},
		bodies: []string{
			string(gzipPayload(t, `{"bars":{"AAPL":[{"t":"2026-01-02T03:04:05Z",`+
				`"o":100,"h":103,"l":99,"c":102,"v":40,"n":5,"vw":101.5}]},`+
				`"next_page_token":"next"}`)),
			string(gzipPayload(t, `{"bars":{"AAPL":[{"t":"2026-01-03T03:04:05Z",`+
				`"o":101,"h":104,"l":100,"c":103,"v":41,"n":6,"vw":102.5}]},`+
				`"next_page_token":null}`)),
		},
		headers: []http.Header{
			{"Content-Encoding": {"gzip"}},
			{"Content-Encoding": {"gzip"}},
		},
	}
	client := newScriptedClient(t, transport, -1)
	values, err := client.GetBars("AAPL", GetBarsRequest{
		TimeFrame: OneDay,
		PageLimit: 1,
	})
	require.NoError(t, err)
	require.Len(t, values, 2)
	require.Len(t, transport.responses, 2)
	require.True(t, transport.closedBeforeCall[1],
		"first gzip page body must be closed before requesting the second page")
	for index, response := range transport.responses {
		assertClosedAfterDrain(t, response, "response %d", index)
	}
}

func TestPaginatedWorkflowClosesResponseBeforeSecondPageError(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusUnprocessableEntity},
		bodies: []string{
			`{"bars":{"AAPL":[{"t":"2026-01-02T03:04:05Z","o":100,"h":103,` +
				`"l":99,"c":102,"v":40,"n":5,"vw":101.5}]},"next_page_token":"next"}`,
			`{"code":42210000,"message":"second page rejected"}`,
		},
	}
	client := newScriptedClient(t, transport, -1)
	_, err := client.GetBars("AAPL", GetBarsRequest{
		TimeFrame: OneDay,
		PageLimit: 1,
	})
	require.Error(t, err)
	require.Len(t, transport.responses, 2)
	require.True(t, transport.closedBeforeCall[1],
		"first page body must be closed before requesting the failing second page")
	for index, response := range transport.responses {
		assertClosedAfterDrain(t, response, "response %d", index)
	}
}

func TestCorporateActionsPreservesPartialResultsOnLaterPageError(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK, http.StatusUnprocessableEntity},
		bodies: []string{
			`{"corporate_actions":{"forward_splits":[{"id":"split-1"}]},` +
				`"next_page_token":"next"}`,
			`{"code":42210000,"message":"second page rejected"}`,
		},
	}
	client := newScriptedClient(t, transport, -1)
	values, err := client.GetCorporateActions(GetCorporateActionsRequest{PageLimit: 1})

	require.Error(t, err)
	require.Len(t, values.ForwardSplits, 1)
	assert.Equal(t, "split-1", values.ForwardSplits[0].ID)
	require.Len(t, transport.responses, 2)
	require.True(t, transport.closedBeforeCall[1],
		"first page body must be closed before requesting the failing second page")
	for index, response := range transport.responses {
		assertClosedAfterDrain(t, response, "response %d", index)
	}
}

func TestPaginatedWorkflowClosesMalformedPageResponse(t *testing.T) {
	transport := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusOK},
		bodies:   []string{`{"snapshots":`},
	}
	client := newScriptedClient(t, transport, -1)
	_, err := client.GetOptionChain("AAPL", GetOptionChainRequest{})
	require.Error(t, err)
	require.Len(t, transport.responses, 1)
	assertClosedAfterDrain(t, transport.responses[0])
}

func TestCompressedAPIErrorClosesSourceResponseOnce(t *testing.T) {
	body := &trackingBody{
		r: strings.NewReader(string(gzipPayload(
			t,
			`{"code":42210000,"message":"compressed rejection"}`,
		))),
	}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: roundTripperFunc(
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnprocessableEntity,
					Status:     http.StatusText(http.StatusUnprocessableEntity),
					Header: http.Header{
						"Content-Encoding": {"gzip"},
						"Content-Type":     {"application/json"},
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
	var apiErr *alpaca.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 42210000, apiErr.Code)
	assert.Equal(t, "compressed rejection", apiErr.Message)
	assert.Equal(t, 1, body.closeCalls)
}

func TestMalformedGzipClosesMarketDataSourceResponseOnce(t *testing.T) {
	body := &trackingBody{r: strings.NewReader("not gzip")}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: roundTripperFunc(
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

func TestMalformedGzipMarketDataAPIErrorPreservesMetadata(t *testing.T) {
	body := &trackingBody{r: strings.NewReader("not a gzip payload")}
	client := NewClient(ClientOpts{
		HTTPClient: &http.Client{Transport: roundTripperFunc(
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

	var apiErr *alpaca.APIError
	require.ErrorAs(t, err, &apiErr)
	require.ErrorIs(t, err, gzip.ErrHeader)
	assert.Equal(t, http.StatusUnprocessableEntity, apiErr.StatusCode)
	assert.Equal(t, "malformed-request", apiErr.RequestID)
	assert.Equal(t, 1, body.closeCalls)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func gzipPayload(t *testing.T, value string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := writer.Write([]byte(value))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
