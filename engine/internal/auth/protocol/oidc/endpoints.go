package oidc

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Env is the operator-facing configuration.
type Env struct {
	// AppURL is the public address users reach Platrium at, e.g.
	// https://files.example.com. Identity providers redirect back to it.
	AppURL string `env:"APP_URL,required"`
}

// Endpoints builds the public URLs identity providers are registered with.
// Every provider has its own, so a callback says which provider (and so which
// tenant) answered before anything else is looked at.
type Endpoints struct {
	base string // no trailing slash
}

// NewEndpoints validates the public address of the installation.
func NewEndpoints(appURL string) (Endpoints, error) {
	appURL = strings.TrimRight(strings.TrimSpace(appURL), "/")
	u, err := url.Parse(appURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return Endpoints{}, fmt.Errorf("APP_URL %q is not a valid http(s) URL", appURL)
	}
	return Endpoints{base: appURL}, nil
}

// EndpointsFromEnv reads APP_URL.
func EndpointsFromEnv() (Endpoints, error) {
	var cfg Env
	if err := env.Parse(&cfg); err != nil {
		return Endpoints{}, err
	}
	return NewEndpoints(cfg.AppURL)
}

// Secure reports whether the installation is served over https, which decides
// whether cookies it sets are marked Secure.
func (e Endpoints) Secure() bool { return strings.HasPrefix(e.base, "https://") }

// Callback is the redirect URI to register with the provider.
func (e Endpoints) Callback(idpID string) string {
	return e.base + "/api/auth/oidc/" + url.PathEscape(idpID) + "/callback"
}
