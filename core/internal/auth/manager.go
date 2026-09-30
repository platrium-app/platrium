package auth

import (
	"context"
	"net/http"
)

// IdpAuthHandoff represents the normalized claims parsed by any authentication flow (OIDC, SAML, Local).
// It contains exactly what the Platrium domain layer needs to issue a session.
type IdpAuthHandoff struct {
	IdpConnectionID string   // The NanoID of the IdP connection (e.g., "idp_okta_1")
	SubjectID       string   // The unique user ID from the IdP (e.g., OIDC "sub" or SAML "NameID")
	Email           string
	DisplayName     string
	AvatarURL       string
	JITGroupIDs     []string // Groups claimed in the token (if JIT is enabled)
}

// AuthManager is the core domain service that processes successful logins.
// It acts as the bridge between the HTTP flows and the GraphDB / Session layer.
type AuthManager interface {
	// HandleFederatedLogin takes the normalized claims, ensures the structural User node
	// exists in the GraphDB, creates the session in BadgerDB, and sets the HTTP cookies.
	HandleFederatedLogin(ctx context.Context, w http.ResponseWriter, handoff IdpAuthHandoff) error
}
