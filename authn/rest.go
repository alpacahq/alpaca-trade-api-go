package authn

import (
	"errors"
	"fmt"
	"net/http"
)

type RestAuthProviderOptions struct {
	Credentials CredentialsParams
	TokenURL    string
	HTTPClient  *http.Client
}

type RestAuthProvider struct {
	credentials         Credentials
	accessTokenProvider *AccessTokenProvider
}

func NewRestAuthProvider(opts RestAuthProviderOptions) *RestAuthProvider {
	return &RestAuthProvider{
		credentials: NewCredentials(opts.Credentials),
		accessTokenProvider: NewAccessTokenProvider(AccessTokenProviderOptions{
			ClientID:     opts.Credentials.ClientID,
			ClientSecret: opts.Credentials.ClientSecret,
			TokenURL:     opts.TokenURL,
			HTTPClient:   opts.HTTPClient,
		}),
	}
}

func (p *RestAuthProvider) SetAuthHeader(req *http.Request, allowCorrespondentCreds bool) error {
	switch {
	case p.credentials.OAuthToken != "":
		setBearerTokenAuthHeader(req, p.credentials.OAuthToken)
		return nil
	case p.credentials.ClientID != "":
		return p.setClientIDAuthHeader(req, allowCorrespondentCreds)
	default:
		return nil
	}
}

func (p *RestAuthProvider) setClientIDAuthHeader(req *http.Request, allowCorrespondentCreds bool) error {
	if p.credentials.isCorrespondentClientID && !allowCorrespondentCreds {
		return errors.New("correspondent client credentials are not allowed")
	}

	switch p.credentials.ClientType {
	case ClientTypeLegacy:
		if p.credentials.isCorrespondentClientID {
			req.SetBasicAuth(p.credentials.ClientID, p.credentials.ClientSecret)
			return nil
		}

		req.Header.Set("APCA-API-KEY-ID", p.credentials.ClientID)
		req.Header.Set("APCA-API-SECRET-KEY", p.credentials.ClientSecret)
		return nil
	case ClientTypeClientSecret:
		accessToken, err := p.accessTokenProvider.Token(req.Context())
		if err != nil {
			return fmt.Errorf("token: %w", err)
		}

		setBearerTokenAuthHeader(req, accessToken)
		return nil
	default:
		return fmt.Errorf("unsupported client type: %s", p.credentials.ClientType)
	}
}

func setBearerTokenAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}
