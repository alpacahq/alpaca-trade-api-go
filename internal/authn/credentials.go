package authn

import (
	"os"
	"strings"
)

type Credentials struct {
	ClientID     string
	ClientSecret string
	OAuthToken   string
	APIKey       string
	APISecret    string
	BrokerKey    string
	BrokerSecret string
}

func CredentialsFromEnv() Credentials {
	return Credentials{
		ClientID:     os.Getenv("APCA_API_CLIENT_ID"),
		ClientSecret: os.Getenv("APCA_API_CLIENT_SECRET"),
		OAuthToken:   os.Getenv("APCA_API_OAUTH"),
		APIKey:       os.Getenv("APCA_API_KEY_ID"),
		APISecret:    os.Getenv("APCA_API_SECRET_KEY"),
	}
}

func IsClientID(key string) bool {
	for _, prefix := range []string{"CK", "AK", "PK"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}

func populateCredentialsFromEnv(creds *Credentials) {
	credsFromEnv := CredentialsFromEnv()
	if creds.APIKey == "" {
		creds.APIKey = credsFromEnv.APIKey
	}
	if creds.APISecret == "" {
		creds.APISecret = credsFromEnv.APISecret
	}
	if creds.OAuthToken == "" {
		creds.OAuthToken = credsFromEnv.OAuthToken
	}
	if creds.ClientID == "" {
		creds.ClientID = credsFromEnv.ClientID
	}
	if creds.ClientSecret == "" {
		creds.ClientSecret = credsFromEnv.ClientSecret
	}
}
