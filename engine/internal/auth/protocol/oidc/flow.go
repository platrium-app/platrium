package oidc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"platrium/internal/auth"
	"platrium/internal/secrets"
)

// How long a sign-in may take between leaving for the provider and coming back.
const flowTTL = 10 * time.Minute

var (
	// ErrFlowInvalid means the callback does not belong to a sign-in this
	// browser started: unknown, expired, replayed or foreign state, or another
	// provider. The wrapped message says which, for the log (not the browser).
	ErrFlowInvalid = errors.New("oidc: sign-in request is invalid or has expired")
	// ErrProviderDenied means the provider reported an error instead of a code.
	ErrProviderDenied = errors.New("oidc: the identity provider refused the sign-in")
	// ErrRejected means the provider authenticated the user but Platrium's
	// settings for it (verified email, an email at all) do not accept them.
	ErrRejected = errors.New("oidc: the identity provider's response was rejected")
)

// FlowState is what Platrium remembers about a sign-in in progress. It is not
// kept on the server at all: it is sealed (encrypted and authenticated) into a
// short-lived cookie in the browser that started the sign-in, and opened again
// on the callback. So a sign-in costs no storage, works whichever instance
// receives the callback, and each sign-in has its own cookie, so tabs and
// double clicks never interfere.
type FlowState struct {
	IdpID     string
	Nonce     string
	Verifier  string // PKCE code verifier
	ReturnTo  string
	CreatedAt time.Time
}

// Client runs OpenID Connect authorization-code flows with PKCE against any
// compliant provider.
type Client struct {
	endpoints Endpoints
	sealer    *secrets.Sealer
	http      *http.Client
	now       func() time.Time

	mu        sync.Mutex
	providers map[string]*cachedProvider
}

type cachedProvider struct {
	p  *gooidc.Provider
	at time.Time
}

const providerTTL = time.Hour

func NewClient(endpoints Endpoints, sealer *secrets.Sealer) *Client {
	return &Client{
		endpoints: endpoints,
		sealer:    sealer,
		http:      &http.Client{Timeout: 10 * time.Second},
		now:       time.Now,
		providers: map[string]*cachedProvider{},
	}
}

// WithHTTPClient replaces the client used to reach providers (tests).
func (c *Client) WithHTTPClient(h *http.Client) *Client { c.http = h; return c }

// newProvider runs discovery for issuer. The spec says the issuer a provider
// reports must equal the one it was reached by exactly, and the library
// enforces it, but providers disagree about a trailing slash (Auth0 reports
// one, most do not) and people reasonably leave it off. A difference of only a
// trailing slash is accepted, and from then on the provider's own spelling is
// the one ID tokens are checked against, character for character. Any other
// difference is still refused.
func newProvider(ctx context.Context, issuer string) (*gooidc.Provider, error) {
	p, err := gooidc.NewProvider(ctx, issuer)
	var mismatch *gooidc.IssuerMismatchError
	if errors.As(err, &mismatch) &&
		strings.TrimRight(mismatch.Provided, "/") == strings.TrimRight(mismatch.Discovered, "/") {
		return gooidc.NewProvider(gooidc.InsecureIssuerURLContext(ctx, mismatch.Discovered), issuer)
	}
	return p, err
}

// TODO(ee, before multi-tenancy ships): SSRF. The issuer is an
// administrator-supplied URL that this server fetches (here, in Discover, and
// in the code exchange), so it can reach loopback, private, link-local and
// cloud-metadata addresses. Harmless while only the cluster operator sets
// issuers; before tenants' own administrators can, c.http needs a dialer that
// refuses those ranges by the resolved IP (not the URL text), switchable off
// for self-hosted installs that legitimately use an internal IdP.
//
// provider discovers the provider at issuer, caching the result. Discovery
// verifies that the issuer the document reports is the one asked for.
func (c *Client) provider(issuer string) (*gooidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cp, ok := c.providers[issuer]; ok && c.now().Sub(cp.at) < providerTTL {
		return cp.p, nil
	}
	// Not the request's context: the provider keeps it to refresh signing keys.
	ctx := gooidc.ClientContext(context.Background(), c.http)
	p, err := newProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", issuer, err)
	}
	c.providers[issuer] = &cachedProvider{p: p, at: c.now()}
	return p, nil
}

func (c *Client) oauthConfig(p *gooidc.Provider, cfg *Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     p.Endpoint(),
		RedirectURL:  c.endpoints.Callback(cfg.IdpID),
		Scopes:       append([]string{gooidc.ScopeOpenID}, cfg.Scopes...),
	}
}

// maxReturnTo bounds the post-login path, which travels in a cookie.
const maxReturnTo = 512

// SafeReturnTo reduces a requested post-login destination to a same-origin
// path, so a sign-in link can not bounce the user to another site.
func SafeReturnTo(raw string) string {
	if raw == "" || len(raw) > maxReturnTo || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") ||
		strings.ContainsAny(raw, "\\\r\n") {
		return "/"
	}
	return raw
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Started is a sign-in that has left for the provider.
type Started struct {
	// AuthURL is where to send the browser.
	AuthURL string
	// State is the OAuth state; it names the sign-in (and its cookie).
	State string
	// Sealed is what the browser must carry back: the sign-in's state, sealed.
	// Hand it to the browser in a cookie and to Complete from that cookie.
	Sealed string
}

// Begin starts a sign-in: it returns the provider URL to send the browser to,
// and the sealed state the browser has to bring back.
func (c *Client) Begin(ctx context.Context, cfg *Config, returnTo string) (*Started, error) {
	p, err := c.provider(cfg.Issuer)
	if err != nil {
		return nil, err
	}
	oc := c.oauthConfig(p, cfg)

	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	authURL := oc.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), gooidc.Nonce(nonce))
	authURL, err = withExtraParams(authURL, cfg.ExtraAuthParams)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(FlowState{
		IdpID: cfg.IdpID, Nonce: nonce, Verifier: verifier,
		ReturnTo: SafeReturnTo(returnTo), CreatedAt: c.now(),
	})
	if err != nil {
		return nil, err
	}
	// Bound to this state: a sealed value only opens for the state it was made for.
	sealed, err := c.sealer.Seal(string(raw), state)
	if err != nil {
		return nil, err
	}
	return &Started{AuthURL: authURL, State: state, Sealed: sealed}, nil
}

// withExtraParams adds provider specific parameters (Auth0's "audience", say)
// to an authorization URL. The library has already set everything the flow
// depends on (redirect_uri, state, nonce, PKCE, scope); an extra parameter of
// the same name is ignored, so configuration can not weaken the flow.
func withExtraParams(authURL string, extra map[string]string) (string, error) {
	if len(extra) == 0 {
		return authURL, nil
	}
	u, err := url.Parse(authURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range extra {
		if !q.Has(k) {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Result is a verified sign-in.
type Result struct {
	Handoff  auth.IdpAuthHandoff
	ReturnTo string
}

// Complete finishes a sign-in from the callback's query values. It redeems the
// flow, exchanges the code, verifies the ID token, and maps its claims.
func (c *Client) Complete(ctx context.Context, cfg *Config, state, code, providerError, sealed string) (*Result, error) {
	flow, err := c.openFlow(state, sealed)
	if err != nil {
		return nil, err
	}
	if flow.IdpID != cfg.IdpID {
		return nil, fmt.Errorf("%w: the sign-in was started for a different provider", ErrFlowInvalid)
	}
	if age := c.now().Sub(flow.CreatedAt); age > flowTTL {
		return nil, fmt.Errorf("%w: the sign-in took %s, longer than the %s allowed", ErrFlowInvalid, age.Round(time.Second), flowTTL)
	}
	if providerError != "" {
		return nil, fmt.Errorf("%w: %s", ErrProviderDenied, providerError)
	}
	if code == "" {
		return nil, fmt.Errorf("%w: the callback carried no code", ErrFlowInvalid)
	}

	p, err := c.provider(cfg.Issuer)
	if err != nil {
		return nil, err
	}
	oc := c.oauthConfig(p, cfg)

	hctx := context.WithValue(ctx, oauth2.HTTPClient, c.http)
	tok, err := oc.Exchange(hctx, code, oauth2.VerifierOption(flow.Verifier))
	if err != nil {
		return nil, fmt.Errorf("oidc code exchange: %w", err)
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		return nil, fmt.Errorf("%w: no id_token in the token response", ErrRejected)
	}
	idToken, err := p.Verifier(&gooidc.Config{ClientID: cfg.ClientID}).Verify(gooidc.ClientContext(ctx, c.http), rawID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRejected, err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce mismatch", ErrRejected)
	}

	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: unreadable claims: %v", ErrRejected, err)
	}
	// Some providers keep profile claims out of the ID token. Ask the userinfo
	// endpoint for what is missing, but only trust it for the same subject.
	if _, has := claims[cfg.EmailClaim]; !has {
		if ui, err := p.UserInfo(gooidc.ClientContext(ctx, c.http), oc.TokenSource(hctx, tok)); err == nil && ui.Subject == idToken.Subject {
			extra := map[string]any{}
			if ui.Claims(&extra) == nil {
				for k, v := range extra {
					if _, exists := claims[k]; !exists {
						claims[k] = v
					}
				}
			}
		}
	}

	h, err := handoffFromClaims(cfg, idToken.Subject, claims)
	if err != nil {
		return nil, err
	}
	return &Result{Handoff: *h, ReturnTo: flow.ReturnTo}, nil
}

// openFlow opens the sealed state a browser brought back. Nothing identifies a
// sign-in except what only the browser that started it holds, so a callback
// without the matching cookie, or with someone else's, is refused.
func (c *Client) openFlow(state, sealed string) (*FlowState, error) {
	if state == "" {
		return nil, fmt.Errorf("%w: the callback carried no state", ErrFlowInvalid)
	}
	if sealed == "" {
		return nil, fmt.Errorf("%w: the browser sent no sign-in cookie (never started here, expired, already used, cookies blocked, or a different browser)", ErrFlowInvalid)
	}
	raw, err := c.sealer.Open(sealed, state)
	if err != nil {
		return nil, fmt.Errorf("%w: the sign-in cookie does not match this callback (another sign-in's, altered, or sealed under another key)", ErrFlowInvalid)
	}
	var flow FlowState
	if err := json.Unmarshal([]byte(raw), &flow); err != nil {
		return nil, fmt.Errorf("%w: unreadable sign-in state", ErrFlowInvalid)
	}
	return &flow, nil
}

// handoffFromClaims maps provider claims to the protocol-agnostic handoff.
func handoffFromClaims(cfg *Config, subject string, claims map[string]any) (*auth.IdpAuthHandoff, error) {
	if subject == "" {
		return nil, fmt.Errorf("%w: no subject", ErrRejected)
	}
	email := strings.TrimSpace(stringClaim(claims, cfg.EmailClaim))
	if email == "" {
		return nil, fmt.Errorf("%w: the provider sent no %q claim", ErrRejected, cfg.EmailClaim)
	}
	verified := boolClaim(claims, "email_verified")
	if cfg.RequireEmailVerified && !verified {
		return nil, fmt.Errorf("%w: the provider has not verified the email address", ErrRejected)
	}

	h := &auth.IdpAuthHandoff{
		IdpProviderID: cfg.IdpID,
		SubjectID:     subject,
		Email:         email,
		EmailVerified: verified,
		DisplayName:   strings.TrimSpace(stringClaim(claims, cfg.NameClaim)),
		AvatarURL:     stringClaim(claims, cfg.PictureClaim),
	}
	if h.DisplayName == "" {
		h.DisplayName = email
	}
	return h, nil
}

func stringClaim(claims map[string]any, name string) string {
	s, _ := claims[name].(string)
	return s
}

// boolClaim reads a boolean, accepting the string form some providers send.
func boolClaim(claims map[string]any, name string) bool {
	switch v := claims[name].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	}
	return false
}

// ErrDiscovery means the issuer could not be used as an OpenID provider. The
// cause is logged, not returned: a caller who chooses the URL must not be able
// to read what the server saw there.
var ErrDiscovery = errors.New("could not read an OpenID Connect discovery document from the issuer")

// Discovery is what a provider says about itself.
type Discovery struct {
	Issuer                string
	AuthorizationEndpoint string
	TokenEndpoint         string
	JWKSURI               string
	ScopesSupported       []string
	ClaimsSupported       []string
	// SupportsPKCE is false only when the provider lists its PKCE methods and
	// S256 is not among them; Platrium always sends an S256 challenge.
	SupportsPKCE bool
}

// Discover reads the discovery document of an issuer without caching it, so an
// administrator can check a provider before (or after) saving it.
func (c *Client) Discover(ctx context.Context, issuer string) (*Discovery, error) {
	issuer, err := ValidateIssuer(issuer)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(gooidc.ClientContext(ctx, c.http), 10*time.Second)
	defer cancel()

	p, err := newProvider(ctx, issuer)
	if err != nil {
		log.Printf("oidc discovery check for %s: %v", issuer, err)
		return nil, ErrDiscovery
	}
	var doc struct {
		JWKSURI              string   `json:"jwks_uri"`
		ScopesSupported      []string `json:"scopes_supported"`
		ClaimsSupported      []string `json:"claims_supported"`
		CodeChallengeMethods []string `json:"code_challenge_methods_supported"`
	}
	if err := p.Claims(&doc); err != nil {
		log.Printf("oidc discovery check for %s: %v", issuer, err)
		return nil, ErrDiscovery
	}
	return &Discovery{
		Issuer:                issuer,
		AuthorizationEndpoint: p.Endpoint().AuthURL,
		TokenEndpoint:         p.Endpoint().TokenURL,
		JWKSURI:               doc.JWKSURI,
		ScopesSupported:       doc.ScopesSupported,
		ClaimsSupported:       doc.ClaimsSupported,
		SupportsPKCE:          len(doc.CodeChallengeMethods) == 0 || slices.Contains(doc.CodeChallengeMethods, "S256"),
	}, nil
}
