package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"platrium/internal/apperr"
	"platrium/internal/auth"
)

// Handler serves the OIDC specific parts of signing in.
type Handler struct {
	idps    *auth.IdpStore
	store   *Store
	client  *Client
	manager auth.AuthManager
}

func NewHandler(idps *auth.IdpStore, store *Store, client *Client, manager auth.AuthManager) *Handler {
	return &Handler{idps: idps, store: store, client: client, manager: manager}
}

// provider loads an OIDC provider and its config. A missing or non-OIDC
// provider is ErrNotFound. Whether the provider may be used (enabled, ...) is
// the generic sign-in gate's business, not the protocol's.
func (h *Handler) provider(ctx context.Context, idpID string) (*auth.IdpProvider, *Config, error) {
	idp, err := h.idps.GetIdpById(ctx, idpID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: identity provider", apperr.ErrNotFound)
	}
	if idp.Type != auth.IdpTypeOIDC {
		return nil, nil, fmt.Errorf("%w: identity provider", apperr.ErrNotFound)
	}
	cfg, err := h.store.Get(ctx, idp.ID)
	if err != nil {
		return nil, nil, err
	}
	return idp, cfg, nil
}

// flowCookiePrefix names the cookie carrying one sign-in in progress; the rest
// of the name is a short hash of the sign-in's state, so every sign-in has a
// cookie of its own and tabs never overwrite each other.
const flowCookiePrefix = "platrium_authverify_"

func flowCookieName(state string) string {
	sum := sha256.Sum256([]byte(state))
	return flowCookiePrefix + hex.EncodeToString(sum[:6])
}

func (h *Handler) flowCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/api/auth/oidc/", // only the callback needs it
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.client.endpoints.Secure(),
		SameSite: http.SameSiteLaxMode, // sent back on the provider's top-level redirect
	}
}

// Begin starts a sign-in with the given provider for the browser behind w and
// r and returns the URL to send it to. returnTo is where the user lands
// afterwards. The sign-in's state travels in a short-lived sealed cookie on the
// response, so nothing is stored on the server.
func (h *Handler) Begin(w http.ResponseWriter, r *http.Request, idpID, returnTo string) (string, error) {
	_, cfg, err := h.provider(r.Context(), idpID)
	if err != nil {
		return "", err
	}
	started, err := h.client.Begin(r.Context(), cfg, returnTo)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, h.flowCookie(flowCookieName(started.State), started.Sealed, int(flowTTL.Seconds())))
	return started.AuthURL, nil
}

// Sign-in failures send the user back to the login page with one of these
// codes. The detail goes to the log, never to the browser.
const (
	errDenied    = "sso_denied"     // the user or the provider declined
	errExpired   = "sso_expired"    // stale, replayed or foreign callback
	errNoAccount = "sso_no_account" // no account, and none may be created, or the email is not allowed
	errDisabled  = "sso_disabled"   // the account or the provider is disabled
	errFailed    = "sso_failed"     // anything else
)

func loginError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?"+url.Values{"error": {code}}.Encode(), http.StatusFound)
}

// Callback handles `GET /api/auth/oidc/{idpId}/callback`. It is a pure OIDC
// protocol endpoint: it turns the provider's response into a protocol-agnostic
// IdpAuthHandoff and gives it to the session layer.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idpID := chi.URLParam(r, "idpId")

	_, cfg, err := h.provider(ctx, idpID)
	if err != nil {
		log.Printf("oidc callback: provider %q: %v", idpID, err)
		loginError(w, r, errFailed)
		return
	}

	q := r.URL.Query()
	state := q.Get("state")
	var sealed string
	if state != "" {
		name := flowCookieName(state)
		if c, err := r.Cookie(name); err == nil {
			sealed = c.Value
		}
		// One use: the cookie is removed whatever the outcome.
		http.SetCookie(w, h.flowCookie(name, "", -1))
	}
	res, err := h.client.Complete(ctx, cfg, state, q.Get("code"), q.Get("error"), sealed)
	if err != nil {
		log.Printf("oidc callback: idp %s: %v", idpID, err)
		switch {
		case errors.Is(err, ErrProviderDenied):
			loginError(w, r, errDenied)
		case errors.Is(err, ErrFlowInvalid):
			loginError(w, r, errExpired)
		default:
			loginError(w, r, errFailed)
		}
		return
	}

	if err := h.manager.HandleFederatedLogin(ctx, res.Handoff); err != nil {
		log.Printf("oidc callback: idp %s: login: %v", idpID, err)
		switch {
		case errors.Is(err, auth.ErrNotProvisioned), errors.Is(err, auth.ErrEmailNotAllowed):
			loginError(w, r, errNoAccount)
		case errors.Is(err, auth.ErrUserDisabled), errors.Is(err, auth.ErrProviderDisabled):
			loginError(w, r, errDisabled)
		default:
			loginError(w, r, errFailed)
		}
		return
	}
	http.Redirect(w, r, res.ReturnTo, http.StatusFound)
}
