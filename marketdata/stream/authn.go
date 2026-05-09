package stream

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/internal/authn"
)

type streamAuthProviderOptions struct {
	tokenURL string
	key      string
	secret   string
}

type streamAuthProvider struct {
	key           string
	secret        string
	tokenProvider *authn.AccessTokenProvider
}

func newStreamAuthProvider(opts streamAuthProviderOptions) *streamAuthProvider {
	if opts.tokenURL == "" {
		opts.tokenURL = authn.TokenURL()
	}

	credsFromEnv := authn.CredentialsFromEnv()
	if opts.key == "" {
		opts.key = credsFromEnv.ClientID
	}

	if opts.secret == "" {
		opts.secret = credsFromEnv.ClientSecret
	}

	provider := &streamAuthProvider{
		key:    opts.key,
		secret: opts.secret,
	}

	if authn.IsClientID(opts.key) {
		provider.tokenProvider = authn.NewAccessTokenProvider(authn.AccessTokenProviderOptions{
			TokenURL:     opts.tokenURL,
			ClientID:     opts.key,
			ClientSecret: opts.secret,
		})
	}

	return provider
}

func (c *streamAuthProvider) authMessage(ctx context.Context) (map[string]string, error) {
	if c.tokenProvider != nil {
		accessToken, err := c.tokenProvider.Token(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"action": "auth",
			"key":    "access_token",
			"secret": accessToken,
		}, nil
	}

	return map[string]string{
		"action": "auth",
		"key":    c.key,
		"secret": c.secret,
	}, nil
}
