package oidc

import (
	"net/http"
	"platrium/internal/auth"
)

// OidcHTTPHandler handles the protocol-specific HTTP callbacks.
type OidcHTTPHandler struct {
	authManager auth.AuthManager
	client      *OIDCClient
}

func NewOidcHTTPHandler(authManager auth.AuthManager, client *OIDCClient) *OidcHTTPHandler {
	return &OidcHTTPHandler{
		authManager: authManager,
		client:      client,
	}
}

// Callback handles `GET /auth/oidc/callback`
// This is strictly an OIDC protocol endpoint. It translates the OIDC-specific
// `?code=` query parameter into a universal IdpAuthHandoff.
func (h *OidcHTTPHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// code := r.URL.Query().Get("code")

	// 1. Delegate cryptographic verification to the Protocol Client
	// handoff, err := h.client.Exchange(code)
	// if err != nil { ... }

	// Mocking the handoff for now
	handoff := auth.IdpAuthHandoff{
		IdpProviderID: "idp_12345",
		SubjectID:     "sub-xyz-99",
		Email:         "alice@acme.com",
		DisplayName:   "Alice",
	}

	// 2. Hand it back to the generic Session Orchestrator!
	if err := h.authManager.HandleFederatedLogin(r.Context(), w, handoff); err != nil {
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}
}
