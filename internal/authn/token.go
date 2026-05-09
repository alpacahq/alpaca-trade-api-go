package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type AccessTokenProviderOptions struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	HTTPClient   *http.Client
	RetryLimit   int
	RetryDelay   time.Duration
}

type AccessTokenProvider struct {
	clientID     string
	clientSecret string
	tokenURL     string
	httpClient   *http.Client
	retryLimit   int
	retryDelay   time.Duration

	mu          sync.Mutex // guards concurrent access to accessToken and expiresAt
	accessToken string
	expiresAt   time.Time
}

func TokenURL() string {
	if tokenURLFromEnv := os.Getenv("APCA_API_TOKEN_URL"); tokenURLFromEnv != "" {
		return tokenURLFromEnv
	}

	return "https://authx.alpaca.markets/v1/oauth2/token"
}

func NewAccessTokenProvider(opts AccessTokenProviderOptions) *AccessTokenProvider {
	if opts.TokenURL == "" {
		opts.TokenURL = TokenURL()
	}

	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}

	return &AccessTokenProvider{
		clientID:     opts.ClientID,
		clientSecret: opts.ClientSecret,
		tokenURL:     opts.TokenURL,
		httpClient:   opts.HTTPClient,
		retryLimit:   opts.RetryLimit,
		retryDelay:   opts.RetryDelay,
	}
}

func (p *AccessTokenProvider) Token(ctx context.Context) (string, error) {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.accessToken != "" && p.expiresAt.After(now) {
		t := p.accessToken
		return t, nil
	}

	tokenResp, err := p.fetch(ctx)
	if err != nil {
		return "", err
	}

	expiresInWithSkew := time.Duration(tokenResp.ExpiresIn-10) * time.Second
	p.expiresAt = time.Now().Add(expiresInWithSkew)
	p.accessToken = tokenResp.AccessToken

	return p.accessToken, nil
}

func (p *AccessTokenProvider) fetch(ctx context.Context) (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp *http.Response
	for i := 0; ; i++ {
		resp, err = p.httpClient.Do(req)
		if err != nil {
			return tokenResponse{}, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}

		if i >= p.retryLimit {
			break
		}

		time.Sleep(p.retryDelay)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, authErrorFromResponse(resp)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return tokenResponse{}, fmt.Errorf("decode: %w", err)
	}

	return tokenResp, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type authnError struct {
	StatusCode int      `json:"-"`
	ErrorCode  string   `json:"error"`
	Fields     []string `json:"fields"`
	Body       string   `json:"-"`
}

func authErrorFromResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var authErr authnError
	if err := json.Unmarshal(body, &authErr); err != nil {
		return fmt.Errorf("%s (HTTP %d)", body, resp.StatusCode)
	}
	authErr.StatusCode = resp.StatusCode
	authErr.Body = strings.TrimSpace(string(body))
	return &authErr
}

func (e *authnError) Error() string {
	msg := e.ErrorCode
	if msg == "" {
		msg = e.Body
	}

	info := []string{fmt.Sprintf("HTTP %d", e.StatusCode)}
	if len(e.Fields) > 0 {
		info = append(info, fmt.Sprintf("Fields %s", strings.Join(e.Fields, ", ")))
	}

	return fmt.Sprintf("%s (%s)", msg, strings.Join(info, ", "))
}
