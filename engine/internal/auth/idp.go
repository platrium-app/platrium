package auth

import (
	"context"
	"fmt"
	"platrium/internal/infra/graph"
)

// IdpProvider represents the structural definition of an Identity Provider in the Graph DB.
// It is agnostic to whether it is OIDC, SAML, or Local.
type IdpProvider struct {
	ID       string `json:"id"` // NanoID (e.g., "idp_google_1")
	TenantID string `json:"tenant_id"`
	Type     string `json:"type"` // "OIDC", "SAML", "LOCAL"
	Name     string `json:"name"` // Display name for the Login Picker (e.g., "Acme Azure AD")

	// ProtoConfig contains the serialized configuration specific to the Type.
	// For OIDC: {"client_id": "...", "client_secret": "..."}
	// For SAML: {"idp_metadata_url": "..."}
	ProtoConfig string `json:"proto_config"`
}

// IdpAuthHandoff represents the normalized claims parsed by any authentication flow (OIDC, SAML, Local).
// It contains exactly what the Platrium domain layer needs to issue a session.
type IdpAuthHandoff struct {
	IdpProviderID string `json:"idp_connection_id"` // The NanoID of the IdP connection (e.g., "idp_okta_1")
	SubjectID     string `json:"subject_id"`        // The unique user ID from the IdP (e.g., OIDC "sub" or SAML "NameID")
	Email         string
	DisplayName   string   `json:"display_name"`
	AvatarURL     string   `json:"avatar_url"`
	JITGroupIDs   []string // TODO: See if needed Groups claimed in the token (if JIT is enabled)
}

// IdpStore manages IdpProvider nodes in the GraphDB.
type IdpStore struct {
	store graph.Graph
}

func NewIdpStore(store graph.Graph) *IdpStore {
	return &IdpStore{store: store}
}

// GetIdpsByAlias looks up all configured IdPs for a specific tenant alias.
// This is incredibly fast (O(1) or O(log N)) because we will put a database index on Tenant.alias.
func (r *IdpStore) GetIdpsByAlias(ctx context.Context, alias string) ([]*IdpProvider, error) {
	query := `
		MATCH (t:Tenant {alias: $alias})-[:USES_IDP]->(i:IdpProvider)
		RETURN 
			i.id AS id,
			t.id AS tenant_id,
			i.type AS type,
			i.name AS name,
			i.configJSON AS proto_config
	`

	var idps []*IdpProvider
	err := r.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, map[string]any{"alias": alias})
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var idp IdpProvider
			if err := res.Scan(&idp); err != nil {
				return fmt.Errorf("failed to scan IdpProvider: %w", err)
			}
			idps = append(idps, &idp)
		}

		return res.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch idps by alias: %w", err)
	}

	return idps, nil
}
