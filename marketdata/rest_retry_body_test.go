package marketdata

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
)

// trackingBody is a response body that records whether it was read to EOF, whether
// Close was called, and whether EOF had been reached when Close was first called.
type trackingBody struct {
	r              *strings.Reader
	eof            bool
	closed         bool
	eofBeforeClose bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if err == io.EOF {
		b.eof = true
	}
	return n, err
}

func (b *trackingBody) Close() error {
	if !b.closed {
		b.eofBeforeClose = b.eof
	}
	b.closed = true
	return nil
}

// scriptedTransport answers request i with statuses[i] and bodies[i]. Before answering
// request i it records, in closedBeforeCall[i], whether every earlier response body had
// already been closed. A request beyond the script fails the test with a clear message.
type scriptedTransport struct {
	t        *testing.T
	statuses []int
	bodies   []string

	responses        []*trackingBody
	closedBeforeCall []bool
}

func (s *scriptedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := len(s.responses)
	if i >= len(s.statuses) || i >= len(s.bodies) {
		s.t.Errorf("unexpected request %d to %s: script defines only %d responses",
			i+1, req.URL, len(s.statuses))
		return nil, errors.New("unexpected request beyond scripted responses")
	}

	allPrevClosed := true
	for _, b := range s.responses {
		allPrevClosed = allPrevClosed && b.closed
	}
	s.closedBeforeCall = append(s.closedBeforeCall, allPrevClosed)

	body := &trackingBody{r: strings.NewReader(s.bodies[i])}
	s.responses = append(s.responses, body)
	return &http.Response{
		StatusCode: s.statuses[i],
		Status:     http.StatusText(s.statuses[i]),
		Header:     http.Header{},
		Body:       body,
		Request:    req,
	}, nil
}

func newScriptedClient(t *testing.T, tr *scriptedTransport, retryLimit int) *Client {
	t.Helper()
	return NewClient(ClientOpts{
		BaseURL:    "http://marketdata.test",
		HTTPClient: &http.Client{Transport: tr},
		RetryLimit: retryLimit,
		RetryDelay: time.Nanosecond,
	})
}

// assertClosedAfterDrain checks that a body was closed and that it had been read to EOF
// first. Draining is what lets net/http reuse the connection.
func assertClosedAfterDrain(t *testing.T, b *trackingBody, msgAndArgs ...interface{}) {
	t.Helper()
	assert.True(t, b.closed, msgAndArgs...)
	assert.True(t, b.eofBeforeClose, msgAndArgs...)
}

// TestDefaultDo_RetryClosesBodyBeforeRetry covers both retry conditions (429 and 500).
// For each one, the failed response body must be drained and closed before the retry is
// sent, and the successful response must be returned intact.
func TestDefaultDo_RetryClosesBodyBeforeRetry(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{name: "429 is retried", status: http.StatusTooManyRequests},
		{name: "500 is retried", status: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := &scriptedTransport{
				t:        t,
				statuses: []int{tc.status, http.StatusOK},
				bodies:   []string{"transient failure", `{"ok":true}`},
			}
			c := newScriptedClient(t, tr, 3)

			req, err := http.NewRequest(http.MethodGet, "http://marketdata.test/v2/stocks/SPY/bars", nil)
			require.NoError(t, err)

			resp, err := defaultDo(c, req)
			require.NoError(t, err)

			require.Len(t, tr.responses, 2, "expected exactly one retry")
			assert.True(t, tr.closedBeforeCall[1],
				"%d response body must be closed before the retry request is sent", tc.status)
			assertClosedAfterDrain(t, tr.responses[0],
				"%d response body must be drained to EOF and closed", tc.status)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, `{"ok":true}`, string(b))
		})
	}
}

// TestDefaultDo_RetryClosesEveryRetriedBody checks that with retries exhausted, every
// intermediate 500 body is drained and closed before the next attempt, and the final
// error body is closed.
func TestDefaultDo_RetryClosesEveryRetriedBody(t *testing.T) {
	tr := &scriptedTransport{
		t: t,
		statuses: []int{
			http.StatusInternalServerError,
			http.StatusInternalServerError,
			http.StatusInternalServerError,
		},
		bodies: []string{"boom 1", "boom 2", "boom 3"},
	}
	c := newScriptedClient(t, tr, 2)

	req, err := http.NewRequest(http.MethodGet, "http://marketdata.test/v2/stocks/SPY/bars", nil)
	require.NoError(t, err)

	_, err = defaultDo(c, req)
	require.Error(t, err)

	require.Len(t, tr.responses, 3)
	assert.True(t, tr.closedBeforeCall[1], "first 500 body must be closed before the second attempt")
	assert.True(t, tr.closedBeforeCall[2], "second 500 body must be closed before the third attempt")
	for i, b := range tr.responses {
		assertClosedAfterDrain(t, b, "response %d body must be drained to EOF and closed", i)
	}
}

// TestDefaultDo_NonRetryableStatusIsNotRetried is a control: 503 is not a retry condition,
// so exactly one request is made, the error is an *alpaca.APIError carrying status 503,
// and the response body is drained and closed.
func TestDefaultDo_NonRetryableStatusIsNotRetried(t *testing.T) {
	tr := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusServiceUnavailable},
		bodies:   []string{"unavailable"},
	}
	c := newScriptedClient(t, tr, 3)

	req, err := http.NewRequest(http.MethodGet, "http://marketdata.test/v2/stocks/SPY/bars", nil)
	require.NoError(t, err)

	resp, err := defaultDo(c, req)
	require.Error(t, err)
	assert.Nil(t, resp)

	var apiErr *alpaca.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	assert.Equal(t, "unavailable", apiErr.Message)

	require.Len(t, tr.responses, 1, "503 must not be retried")
	assertClosedAfterDrain(t, tr.responses[0], "503 error response body must be drained to EOF and closed")
}
