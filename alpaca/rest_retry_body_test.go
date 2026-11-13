package alpaca

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		BaseURL:    "http://alpaca.test",
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

// TestDefaultDo_ClosesRateLimitedBodyBeforeRetry proves that the 429 response body is
// drained and closed before the retry request is sent, and that the 200 response is
// returned intact.
func TestDefaultDo_ClosesRateLimitedBodyBeforeRetry(t *testing.T) {
	tr := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusTooManyRequests, http.StatusOK},
		bodies:   []string{"too many requests", `{"id":"acct-1"}`},
	}
	c := newScriptedClient(t, tr, 3)

	req, err := http.NewRequest(http.MethodGet, "http://alpaca.test/v2/account", nil)
	require.NoError(t, err)

	resp, err := defaultDo(c, req)
	require.NoError(t, err)

	require.Len(t, tr.responses, 2, "expected exactly one retry")
	assert.True(t, tr.closedBeforeCall[1],
		"429 response body must be closed before the retry request is sent")
	assertClosedAfterDrain(t, tr.responses[0], "429 response body must be drained to EOF and closed")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"id":"acct-1"}`, string(b))
}

// TestDefaultDo_ClosesEveryRetriedBody proves that when retries are exhausted, every
// intermediate 429 body is drained and closed before the next attempt, and the final
// error response is also closed.
func TestDefaultDo_ClosesEveryRetriedBody(t *testing.T) {
	tr := &scriptedTransport{
		t: t,
		statuses: []int{
			http.StatusTooManyRequests,
			http.StatusTooManyRequests,
			http.StatusTooManyRequests,
		},
		bodies: []string{"slow down 1", "slow down 2", "slow down 3"},
	}
	c := newScriptedClient(t, tr, 2)

	req, err := http.NewRequest(http.MethodGet, "http://alpaca.test/v2/account", nil)
	require.NoError(t, err)

	_, err = defaultDo(c, req)
	require.Error(t, err)

	require.Len(t, tr.responses, 3)
	assert.True(t, tr.closedBeforeCall[1], "first 429 body must be closed before the second attempt")
	assert.True(t, tr.closedBeforeCall[2], "second 429 body must be closed before the third attempt")
	for i, b := range tr.responses {
		assertClosedAfterDrain(t, b, "response %d body must be drained to EOF and closed", i)
	}
}

// TestDefaultDo_ServerErrorIsNotRetried is a control: 500 is not a retry condition in the
// alpaca client, so exactly one request is made, the error is an *APIError carrying the
// status and message, and the response body is drained and closed.
func TestDefaultDo_ServerErrorIsNotRetried(t *testing.T) {
	tr := &scriptedTransport{
		t:        t,
		statuses: []int{http.StatusInternalServerError},
		bodies:   []string{"internal error"},
	}
	c := newScriptedClient(t, tr, 3)

	req, err := http.NewRequest(http.MethodGet, "http://alpaca.test/v2/account", nil)
	require.NoError(t, err)

	resp, err := defaultDo(c, req)
	require.Error(t, err)
	assert.Nil(t, resp)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "internal error", apiErr.Message)

	require.Len(t, tr.responses, 1, "500 must not be retried")
	assertClosedAfterDrain(t, tr.responses[0], "500 error response body must be drained to EOF and closed")
}
