package stream

import (
	"context"

	"github.com/alpacahq/alpaca-trade-api-go/v3/authn"
)

type streamAuthProviderOptions struct {
	tokenURL   string
	key        string
	secret     string
	clientType authn.ClientType
}

type streamAuthProvider struct {
	key           string
	secret        string
	clientType    authn.ClientType
	tokenProvider *authn.AccessTokenProvider
}

func newStreamAuthProvider(opts streamAuthProviderOptions) *streamAuthProvider {
	if opts.tokenURL == "" {
		opts.tokenURL = authn.TokenURL()
	}

	credsFromEnv := authn.NewCredentials(authn.CredentialsParams{})

	if opts.key == "" {
		opts.key = credsFromEnv.ClientID
	}

	if opts.secret == "" {
		opts.secret = credsFromEnv.ClientSecret
	}

	if opts.clientType == "" {
		opts.clientType = credsFromEnv.ClientType
	}

	provider := &streamAuthProvider{
		key:        opts.key,
		secret:     opts.secret,
		clientType: opts.clientType,
	}

	if opts.clientType == authn.ClientTypeClientSecret {
		provider.tokenProvider = authn.NewAccessTokenProvider(authn.AccessTokenProviderOptions{
			TokenURL:     opts.tokenURL,
			ClientID:     opts.key,
			ClientSecret: opts.secret,
		})
	}

	return provider
}

func (c *streamAuthProvider) authMessage(ctx context.Context) (map[string]string, error) {
	if c.clientType == authn.ClientTypeClientSecret {
		accessToken, err := c.tokenProvider.Token(ctx)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"action": "access_token",
			"key":    c.key,
			"secret": accessToken,
		}, nil
	}

	return map[string]string{
		"action": "auth",
		"key":    c.key,
		"secret": c.secret,
	}, nil
}
