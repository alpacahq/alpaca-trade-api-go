package authn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Provider struct {
	credentials credentials
	tokenURL    string
	httpClient  *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func NewProvider(httpClient *http.Client, tokenURL string, credentials CredentialsParams) *Provider {
	if tokenURL == "" {
		tokenURL = "https://authx.alpaca.markets/oauth2/token" //nolint:gosec
		if tokenURLFromEnv := os.Getenv("APCA_API_TOKEN_URL"); tokenURLFromEnv != "" {
			tokenURL = tokenURLFromEnv
		}
	}

	return &Provider{
		credentials: newCredentials(credentials),
		tokenURL:    tokenURL,
		httpClient:  httpClient,
	}
}

func (p *Provider) SetAuthHeader(req *http.Request, allowCorrespondentCreds bool) error {
	switch {
	case p.credentials.oAuthToken != "":
		setBearerTokenAuthHeader(req, p.credentials.oAuthToken)
		return nil
	case p.credentials.clientID != "":
		return p.setClientIDAuthHeader(req, allowCorrespondentCreds)
	default:
		return errors.New("invalid credentials")
	}
}

func (p *Provider) setClientIDAuthHeader(req *http.Request, allowCorrespondentCreds bool) error {
	if p.credentials.isCorrespondentClientID && !allowCorrespondentCreds {
		return errors.New("correspondent client credentials are not allowed")
	}

	switch p.credentials.clientType {
	case ClientTypeLegacy:
		if p.credentials.isCorrespondentClientID {
			req.SetBasicAuth(p.credentials.clientID, p.credentials.clientSecret)
			return nil
		}

		req.Header.Set("APCA-API-KEY-ID", p.credentials.clientID)
		req.Header.Set("APCA-API-SECRET-KEY", p.credentials.clientSecret)
		return nil
	case ClientTypeClientSecret:
		accessToken, err := p.token()
		if err != nil {
			return fmt.Errorf("token: %w", err)
		}

		setBearerTokenAuthHeader(req, accessToken)
		return nil
	default:
		return fmt.Errorf("unsupported client type: %s", p.credentials.clientType)
	}
}

func (p *Provider) token() (string, error) {
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

func (p *Provider) fetch() (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.credentials.clientID)
	form.Set("client_secret", p.credentials.clientSecret)

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

func setBearerTokenAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}
