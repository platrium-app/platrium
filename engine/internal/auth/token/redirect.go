package token

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ErrBadRedirect means a redirect_uri may not receive authorization codes.
var ErrBadRedirect = errors.New("redirect_uri not allowed")

// Schemes that are never a native app's callback.
var blockedSchemes = map[string]bool{
	"javascript": true, "data": true, "file": true, "vbscript": true, "about": true,
	"blob": true, "ftp": true, "ws": true, "wss": true, "mailto": true, "tel": true, "sms": true,
}

// ParseRedirect validates where an authorization code may be sent and returns
// the parsed URL. Allowed:
//   - a custom app scheme (platrium://callback, org.example.app:/cb)
//   - http on a loopback host with any port (CLI and FUSE listeners)
//   - https URLs that start with one of allowedHTTPS (universal links / app links)
//
// Anything else, including plain http to a real host, is refused so a code can
// not be sent to an arbitrary website.
func ParseRedirect(raw string, allowedHTTPS []string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.User != nil || u.Fragment != "" {
		return nil, ErrBadRedirect
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http":
		if !isLoopback(u.Hostname()) {
			return nil, ErrBadRedirect
		}
	case "https":
		for _, prefix := range allowedHTTPS {
			if prefix != "" && strings.HasPrefix(raw, prefix) {
				return u, nil
			}
		}
		return nil, ErrBadRedirect
	default:
		if blockedSchemes[scheme] {
			return nil, ErrBadRedirect
		}
	}
	return u, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
