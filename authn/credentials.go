package authn

import (
	"log"
	"os"
	"strings"
)

type ClientType string

const (
	ClientTypeLegacy       ClientType = "legacy"
	ClientTypeClientSecret ClientType = "client_secret"

	correspondentClientIDPrefix = "CK"
)

type CredentialsParams struct {
	ClientID     string
	ClientSecret string
	ClientType   ClientType
	OAuthToken   string
	APIKey       string // deprecated
	APISecret    string // deprecated
	BrokerKey    string // deprecated
	BrokerSecret string // deprecated
}

type credentials struct {
	clientID                string
	clientSecret            string
	clientType              ClientType
	isCorrespondentClientID bool
	oAuthToken              string
}

func newCredentials(p CredentialsParams) credentials {
	if p.APIKey == "" {
		p.APIKey = os.Getenv("APCA_API_KEY_ID")
	}
	if p.APISecret == "" {
		p.APISecret = os.Getenv("APCA_API_SECRET_KEY")
	}
	if p.OAuthToken == "" {
		p.OAuthToken = os.Getenv("APCA_API_OAUTH")
	}
	if p.ClientID == "" {
		p.ClientID = os.Getenv("APCA_API_CLIENT_ID")
	}
	if p.ClientSecret == "" {
		p.ClientSecret = os.Getenv("APCA_API_CLIENT_SECRET")
	}
	if p.ClientType == "" {
		p.ClientType = ClientTypeLegacy
		if clientTypeFromEnv := os.Getenv("APCA_API_CLIENT_TYPE"); clientTypeFromEnv != "" {
			p.ClientType = ClientType(clientTypeFromEnv)
		}
	}

	switch {
	case p.OAuthToken != "":
		return credentials{
			oAuthToken: p.OAuthToken,
		}
	case p.BrokerKey != "":
		log.Println("broker_key and broker_secret are deprecated. use client_id and client_secret instead")
		return credentials{
			clientID:                p.BrokerKey,
			clientSecret:            p.BrokerSecret,
			clientType:              ClientTypeLegacy,
			isCorrespondentClientID: true,
		}
	case p.APIKey != "":
		log.Println("api_key and api_secret are deprecated. use client_id and client_secret instead")
		return credentials{
			clientID:     p.APIKey,
			clientSecret: p.APISecret,
			clientType:   ClientTypeLegacy,
		}
	default:
		return credentials{
			clientID:                p.ClientID,
			clientSecret:            p.ClientSecret,
			clientType:              p.ClientType,
			isCorrespondentClientID: strings.HasPrefix(p.ClientID, correspondentClientIDPrefix),
		}
	}
}
