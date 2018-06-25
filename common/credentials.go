package common

import (
	"os"
)

var key *APIKey

const (
	EnvApiKeyID     = "APCA_API_KEY_ID"
	EnvApiSecretKey = "APCA_API_SECRET_KEY"
)

func init() {
	key = &APIKey{
		ID:     os.Getenv(EnvApiKeyID),
		Secret: os.Getenv(EnvApiSecretKey),
	}
}

type APIKey struct {
	ID     string
	Secret string
}

// Credentials returns the user's Alpaca API key ID
// and secret for use through the SDK.
func Credentials() *APIKey {
	return key
}
