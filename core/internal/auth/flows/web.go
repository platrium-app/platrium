package flows

import (
	"net/http"

	"platrium/internal/auth"
	"platrium/internal/identity"
)

// WebHandler handles all HTTP endpoints for browser-based authentication flows.
type WebHandler struct {
	authManager auth.AuthManager
	idpStore    *identity.IdpStore // Used to fetch OIDC configuration from GraphDB
}

func NewWebHandler(authManager auth.AuthManager, idpStore *identity.IdpStore) *WebHandler {
	return &WebHandler{
		authManager: authManager,
		idpStore:    idpStore,
	}
}

// Login handles `GET /auth/login?domain=acme.com`
func (h *WebHandler) Login(w http.ResponseWriter, r *http.Request) {
	// 1. Get routing domain from query param (Home Realm Discovery)
	// domain := r.URL.Query().Get("domain")

	// 2. Query Graph DB to find the specific OIDC connection for this domain
	// config, err := h.idpStore.GetIdpByDomain(r.Context(), domain, "OIDC")

	// 3. Unmarshal the config.ConfigJSON into an OIDC struct
	// 4. Construct the OIDC redirect URL and redirect the user
}

// Login handles `GET /auth/logout`
func (h *WebHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// 1. Get routing domain from query param (Home Realm Discovery)
	// domain := r.URL.Query().Get("domain")

	// 2. Query Graph DB to find the specific OIDC connection for this domain
	// config, err := h.idpStore.GetIdpByDomain(r.Context(), domain, "OIDC")

	// 3. Unmarshal the config.ConfigJSON into an OIDC struct
	// 4. Construct the OIDC redirect URL and redirect the user
}
