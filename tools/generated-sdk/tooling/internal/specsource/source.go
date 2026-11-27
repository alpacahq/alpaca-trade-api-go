package specsource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	ApprovedHost = "docs.alpaca.markets"
	MaxBytes     = 64 << 20
)

type Pair struct {
	Primary  []byte
	Redirect []byte
}

func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many specification redirects")
			}
			_, err := ApprovedURL(request.URL.String())
			if err != nil {
				return errors.New("specification redirect left the approved HTTPS host")
			}
			return nil
		},
	}
}

func ApprovedURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" || parsed.Host != ApprovedHost ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("unapproved specification URL %q", value)
	}
	return parsed, nil
}

func Fetch(ctx context.Context, client *http.Client, value string) ([]byte, error) {
	approved, err := ApprovedURL(value)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, approved.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %s", value, response.Status)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) > MaxBytes {
		return nil, errors.New("specification response exceeds 64 MiB")
	}
	if len(content) == 0 {
		return nil, errors.New("specification response is empty")
	}
	return content, nil
}

func FetchPair(ctx context.Context, client *http.Client, primaryURL, redirectURL string) (Pair, error) {
	primary, err := Fetch(ctx, client, primaryURL)
	if err != nil {
		return Pair{}, err
	}
	redirect, err := Fetch(ctx, client, redirectURL)
	if err != nil {
		return Pair{}, err
	}
	if !bytes.Equal(primary, redirect) {
		return Pair{}, errors.New("approved specification URL forms returned different bytes")
	}
	return Pair{Primary: primary, Redirect: redirect}, nil
}
