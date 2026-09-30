package auth

import (
	"context"
	"platrium/internal/infra/graph"
)

// IdpConnection represents the structural definition of an Identity Provider in the Graph DB.
// It is agnostic to whether it is OIDC, SAML, or Local.
type IdpConnection struct {
	ID       string // NanoID (e.g., "idp_google_1")
	TenantID string
	Type     string // "OIDC", "SAML", "LOCAL"
	Name     string // Display name for the Login Picker (e.g., "Acme Azure AD")

	// ConfigJSON contains the serialized configuration specific to the Type.
	// For OIDC: {"client_id": "...", "client_secret": "..."}
	// For SAML: {"idp_metadata_url": "..."}
	ConfigJSON string
}

// IdpAuthHandoff represents the normalized claims parsed by any authentication flow (OIDC, SAML, Local).
// It contains exactly what the Platrium domain layer needs to issue a session.
type IdpAuthHandoff struct {
	IdpConnectionID string // The NanoID of the IdP connection (e.g., "idp_okta_1")
	SubjectID       string // The unique user ID from the IdP (e.g., OIDC "sub" or SAML "NameID")
	Email           string
	DisplayName     string
	AvatarURL       string
	JITGroupIDs     []string // Groups claimed in the token (if JIT is enabled)
}

// IdpStore manages IdpConnection nodes in the GraphDB.
type IdpStore struct {
	store graph.Graph
}

func NewIdpStore(store graph.Graph) *IdpStore {
	return &IdpStore{store: store}
}

// GetIdpsByDomain looks up all configured IdPs for a given email domain.
func (r *IdpStore) GetIdpsByDomain(ctx context.Context, domain string) ([]*IdpConnection, error) {
	// Cypher query conceptually:
	// MATCH (d:Domain {name: $domain})<-[:OWNS_DOMAIN]-(t:Tenant)-[:USES_IDP]->(i:IdpConnection)
	// RETURN i

	// Implementation omitted for brevity
	return nil, nil
}

// GetIdpsByAlias looks up all configured IdPs for a specific tenant alias.
// This is incredibly fast (O(1) or O(log N)) because we will put a database index on Tenant.alias.
func (r *IdpStore) GetIdpsByAlias(ctx context.Context, alias string) ([]*IdpConnection, error) {
	// Cypher query conceptually:
	// MATCH (t:Tenant {alias: $alias})-[:USES_IDP]->(i:IdpConnection)
	// RETURN i

	// Implementation omitted for brevity
	return nil, nil
}
