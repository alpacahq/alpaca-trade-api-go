package authn

import (
	"context"
	"encoding/json"
	"fmt"
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
}

type AccessTokenProvider struct {
	clientID     string
	clientSecret string
	tokenURL     string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
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

	tokenResp, err := p.fetch()
	if err != nil {
		return "", err
	}

	expiresInWithSkew := time.Duration(tokenResp.ExpiresIn-10) * time.Second
	p.expiresAt = time.Now().Add(expiresInWithSkew)
	p.accessToken = tokenResp.AccessToken

	return p.accessToken, nil
}

func (p *AccessTokenProvider) fetch() (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("token request: %s", resp.Status)
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

func TokenURL() string {
	if tokenURLFromEnv := os.Getenv("APCA_API_TOKEN_URL"); tokenURLFromEnv != "" {
		return tokenURLFromEnv
	}

	return "https://authx.alpaca.markets/oauth2/token"
}
