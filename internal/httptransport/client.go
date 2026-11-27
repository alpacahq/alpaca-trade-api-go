// Package httptransport contains shared HTTP client safety policies.
package httptransport

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var ErrInsecureRedirect = errors.New("refusing HTTPS redirect to an insecure scheme")

// CloneWithSafeRedirects returns a shallow clone of base that preserves its
// transport, timeout, cookie jar, and redirect policy while preventing
// credentials from crossing origins or being restored after a cross-origin hop.
func CloneWithSafeRedirects(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	clone := *base
	checkRedirect := base.CheckRedirect
	clone.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		initialScheme, initialHost := origin(via[0].URL)
		previousScheme, _ := origin(via[len(via)-1].URL)
		crossedOrigin := hasCrossOriginHop(initialScheme, initialHost, via[1:])
		if checkRedirect != nil {
			if err := checkRedirect(request, via); err != nil {
				return err
			}
		}
		if err := enforceRedirectSafety(
			initialScheme,
			initialHost,
			previousScheme,
			crossedOrigin,
			request,
		); err != nil {
			return err
		}
		if checkRedirect != nil {
			return nil
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	return &clone
}

func origin(target *url.URL) (string, string) {
	if target == nil {
		return "", ""
	}
	scheme := strings.ToLower(target.Scheme)
	host := strings.ToLower(target.Hostname())
	port := target.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	return scheme, host
}

func hasCrossOriginHop(initialScheme, initialHost string, hops []*http.Request) bool {
	for _, hop := range hops {
		if hop == nil || differentOrigin(initialScheme, initialHost, hop.URL) {
			return true
		}
	}
	return false
}

func enforceRedirectSafety(
	initialScheme, initialHost, previousScheme string,
	crossedOrigin bool,
	request *http.Request,
) error {
	if crossedOrigin || differentOrigin(initialScheme, initialHost, request.URL) {
		stripCredentials(request)
	}
	if request.URL != nil &&
		strings.EqualFold(previousScheme, "https") &&
		!strings.EqualFold(request.URL.Scheme, "https") {
		return ErrInsecureRedirect
	}
	return nil
}

func differentOrigin(initialScheme, initialHost string, target *url.URL) bool {
	targetScheme, targetHost := origin(target)
	return target == nil ||
		!strings.EqualFold(initialScheme, targetScheme) ||
		!strings.EqualFold(initialHost, targetHost)
}

func stripCredentials(request *http.Request) {
	sensitive := map[string]struct{}{
		"apca-api-key-id":     {},
		"apca-api-secret-key": {},
		"authorization":       {},
		"proxy-authorization": {},
		"cookie":              {},
		"cookie2":             {},
	}
	for header := range request.Header {
		if _, ok := sensitive[strings.ToLower(header)]; ok {
			delete(request.Header, header)
		}
	}
}
