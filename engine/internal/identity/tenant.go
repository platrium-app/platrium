package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/tenant"
)

// Tenant represents an organization or isolated billing unit.
type Tenant struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias"` // e.g., "acme" or "family"
	Name      string    `json:"name"`
	IsNative  bool      `json:"is_native"`
	CreatedAt time.Time `json:"created_at"`
}

func tenantFromEnt(t *ent.Tenant) *Tenant {
	return &Tenant{
		ID:        t.ID,
		Alias:     t.Alias,
		Name:      t.Name,
		IsNative:  t.NativeSlot != nil,
		CreatedAt: t.CreatedAt,
	}
}

// NormalizeAlias canonicalizes a tenant alias. Aliases are stored lowercase so
// uniqueness is identical on case-sensitive and case-insensitive collations.
func NormalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// TenantStore manages Tenant records.
type TenantStore struct {
	db *db.DB
}

func NewTenantStore(d *db.DB) *TenantStore {
	return &TenantStore{db: d}
}

// CreateTenantParams are the inputs for creating a tenant.
type CreateTenantParams struct {
	ID       string // optional; generated when empty
	Alias    string
	Name     string
	IsNative bool
}

// CreateTenantTx creates a tenant within the provided transaction. Uniqueness
// of the alias and of the native tenant is enforced by the schema, so a
// violation surfaces as ErrConflict (see DescribeConflict for the reason).
func (r *TenantStore) CreateTenantTx(ctx context.Context, tx *ent.Tx, p CreateTenantParams) (*Tenant, error) {
	create := tx.Tenant.Create().SetAlias(NormalizeAlias(p.Alias)).SetName(p.Name)
	if p.ID != "" {
		create.SetID(p.ID)
	}
	if p.IsNative {
		create.SetNativeSlot(1)
	}

	t, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("%w: tenant alias or native tenant already exists: %v", ErrConflict, err)
		}
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	return tenantFromEnt(t), nil
}

// DescribeConflict explains why creating a tenant with this alias failed. It
// reads outside any transaction (a failed write poisons the transaction on some
// backends), so call it after rollback, on the failure path only.
func (r *TenantStore) DescribeConflict(ctx context.Context, alias string, isNative bool) string {
	if taken, err := r.db.Tenant.Query().Where(tenant.AliasEQ(NormalizeAlias(alias))).Exist(ctx); err == nil && taken {
		return fmt.Sprintf("a tenant with alias '%s' already exists", NormalizeAlias(alias))
	}
	if isNative {
		if has, err := r.HasNativeTenant(ctx); err == nil && has {
			return "a native cluster tenant already exists"
		}
	}
	return "tenant already exists"
}

// HasNativeTenant reports whether the native (cluster) tenant exists.
func (r *TenantStore) HasNativeTenant(ctx context.Context) (bool, error) {
	return r.db.Tenant.Query().Where(tenant.NativeSlotNotNil()).Exist(ctx)
}

// GetTenantCount returns the total number of tenants.
func (r *TenantStore) GetTenantCount(ctx context.Context) (int64, error) {
	n, err := r.db.Tenant.Query().Count(ctx)
	return int64(n), err
}

// PublicIdpProvider represents the public metadata for an Identity Provider.
type PublicIdpProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// PublicTenantAuthConfig represents public tenant authentication details.
type PublicTenantAuthConfig struct {
	TenantID     string               `json:"tenantId"`
	Name         string               `json:"name"`
	Alias        *string              `json:"alias,omitempty"`
	DefaultIdpID *string              `json:"defaultIdpId,omitempty"`
	Providers    []*PublicIdpProvider `json:"providers"`
}

// GetPublicTenantAuthConfig returns public tenant auth details by alias.
// If alias is empty (""), it looks up the native default tenant.
func (r *TenantStore) GetPublicTenantAuthConfig(ctx context.Context, alias string) (*PublicTenantAuthConfig, error) {
	q := r.db.Tenant.Query().WithIdpProviders(func(iq *ent.IdpProviderQuery) {
		iq.Order(ent.Asc(idpprovider.FieldCreatedAt), ent.Asc(idpprovider.FieldID))
	})
	if alias == "" {
		q = q.Where(tenant.NativeSlotNotNil())
	} else {
		q = q.Where(tenant.AliasEQ(NormalizeAlias(alias)))
	}

	t, err := q.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("tenant not found")
		}
		return nil, fmt.Errorf("failed to query tenant auth config: %w", err)
	}

	cfg := &PublicTenantAuthConfig{
		TenantID: t.ID,
		Name:     t.Name,
		Alias:    &t.Alias,
	}
	for _, i := range t.Edges.IdpProviders {
		cfg.Providers = append(cfg.Providers, &PublicIdpProvider{ID: i.ID, Name: i.Name, Type: string(i.Type)})
	}
	return cfg, nil
}
