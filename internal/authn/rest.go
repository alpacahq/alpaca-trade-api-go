package authn

import (
	"fmt"
	"net/http"
)

type RestAuthProviderOptions struct {
	Credentials Credentials
	TokenURL    string
	HTTPClient  *http.Client
}

type RestAuthProvider struct {
	credentials         Credentials
	accessTokenProvider *AccessTokenProvider
}

func NewRestAuthProvider(opts RestAuthProviderOptions) *RestAuthProvider {
	populateCredentialsFromEnv(&opts.Credentials)

	provider := &RestAuthProvider{
		credentials: opts.Credentials,
	}
	if provider.credentials.ClientID != "" {
		provider.accessTokenProvider = NewAccessTokenProvider(AccessTokenProviderOptions{
			ClientID:     provider.credentials.ClientID,
			ClientSecret: provider.credentials.ClientSecret,
			TokenURL:     opts.TokenURL,
			HTTPClient:   opts.HTTPClient,
		})
	}

	return provider
}

func (p *RestAuthProvider) SetAuthHeader(req *http.Request, allowCorrespondentCreds bool) error {
	switch {
	case p.credentials.OAuthToken != "":
		setBearerTokenAuthHeader(req, p.credentials.OAuthToken)
	case p.credentials.APIKey != "":
		req.Header.Set("APCA-API-KEY-ID", p.credentials.APIKey)
		req.Header.Set("APCA-API-SECRET-KEY", p.credentials.APISecret)
	case p.credentials.BrokerKey != "":
		req.SetBasicAuth(p.credentials.BrokerKey, p.credentials.BrokerSecret)
	case p.credentials.ClientID != "":
		accessToken, err := p.accessTokenProvider.Token(req.Context())
		if err != nil {
			return fmt.Errorf("token: %w", err)
		}

		setBearerTokenAuthHeader(req, accessToken)
	}

	return nil
}

func setBearerTokenAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}
