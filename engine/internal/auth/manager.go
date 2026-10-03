package auth

import (
	"context"
	"net/http"
)

// AuthManager is the core domain service that processes successful logins.
// It acts as the bridge between the HTTP flows and the GraphDB / Session layer.
type AuthManager interface {
	// HandleFederatedLogin takes the normalized claims, ensures the structural User node
	// exists in the GraphDB, creates the session in BadgerDB, and sets the HTTP cookies.
	HandleFederatedLogin(ctx context.Context, w http.ResponseWriter, handoff IdpAuthHandoff) error
}
