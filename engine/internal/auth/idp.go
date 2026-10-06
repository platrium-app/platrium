package auth

import (
	"context"
	"fmt"

	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/tenant"
)

// IdpProvider represents the structural definition of an Identity Provider.
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

func idpFromEnt(i *ent.IdpProvider) *IdpProvider {
	return &IdpProvider{
		ID:          i.ID,
		TenantID:    i.TenantID,
		Type:        string(i.Type),
		Name:        i.Name,
		ProtoConfig: i.ProtoConfig,
	}
}

// IdpStore manages IdpProvider records.
type IdpStore struct {
	db *db.DB
}

func NewIdpStore(d *db.DB) *IdpStore {
	return &IdpStore{db: d}
}

// CreateLocalIdpTx creates the built-in LOCAL identity provider for a tenant
// within the provided transaction.
func (r *IdpStore) CreateLocalIdpTx(ctx context.Context, tx *ent.Tx, tenantID, idpID string) (*IdpProvider, error) {
	create := tx.IdpProvider.Create().
		SetTenantID(tenantID).
		SetType(idpprovider.TypeLOCAL).
		SetName("Platrium Authentication").
		SetProtoConfig("{}")
	if idpID != "" {
		create.SetID(idpID)
	}

	i, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create local idp: %w", err)
	}
	return idpFromEnt(i), nil
}

// GetIdpsForTenant looks up all configured IdPs for a specific tenant alias.
func (r *IdpStore) GetIdpsForTenant(ctx context.Context, alias string) ([]*IdpProvider, error) {
	rows, err := r.db.IdpProvider.Query().
		Where(idpprovider.HasTenantWith(tenant.AliasEQ(identity.NormalizeAlias(alias)))).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch idps by alias: %w", err)
	}

	idps := make([]*IdpProvider, 0, len(rows))
	for _, i := range rows {
		idps = append(idps, idpFromEnt(i))
	}
	return idps, nil
}

// ListForTenant returns every IdP of a tenant, the built-in one first.
func (r *IdpStore) ListForTenant(ctx context.Context, tenantID string) ([]*IdpProvider, error) {
	rows, err := r.db.IdpProvider.Query().
		Where(idpprovider.TenantID(tenantID)).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list idps: %w", err)
	}
	idps := make([]*IdpProvider, 0, len(rows))
	for _, i := range rows {
		idps = append(idps, idpFromEnt(i))
	}
	return idps, nil
}

// LocalForTenant returns the tenant's built-in (LOCAL) identity provider.
func (r *IdpStore) LocalForTenant(ctx context.Context, tenantID string) (*IdpProvider, error) {
	i, err := r.db.IdpProvider.Query().
		Where(idpprovider.TenantID(tenantID), idpprovider.TypeEQ(idpprovider.TypeLOCAL)).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: local identity provider", identity.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch local idp: %w", err)
	}
	return idpFromEnt(i), nil
}

// GetIdpById fetches a specific IdP connection by its unique NanoID.
func (r *IdpStore) GetIdpById(ctx context.Context, id string) (*IdpProvider, error) {
	i, err := r.db.IdpProvider.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("idp not found")
		}
		return nil, fmt.Errorf("failed to fetch idp by id: %w", err)
	}
	return idpFromEnt(i), nil
}
