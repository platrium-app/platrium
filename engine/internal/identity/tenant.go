package identity

import (
	"context"
	"fmt"

	"platrium/internal/infra/graph"
)

// Tenant represents an organization or isolated billing unit.
type Tenant struct {
	ID        string `json:"id"`
	Alias     string `json:"alias"` // e.g., "acme" or "family"
	Name      string `json:"name"`
	IsNative  bool   `json:"is_native"`
	CreatedAt int64  `json:"created_at"`
}

// TenantStore manages Tenant nodes in the GraphDB.
type TenantStore struct {
	store graph.Graph
}

func NewTenantStore(store graph.Graph) *TenantStore {
	return &TenantStore{store: store}
}

// CreateTenant creates a new Tenant node and a Local IdP fallback.
func (r *TenantStore) HasNativeTenant(ctx context.Context) (bool, error) {
	var hasNative bool
	err := r.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, "MATCH (t:Tenant {isNative: true}) RETURN t IS NOT NULL AS exists LIMIT 1", nil)
		if err != nil {
			return err
		}
		defer res.Close()
		if res.Next() {
			var check struct {
				Exists bool `json:"exists"`
			}
			if err := res.Scan(&check); err != nil {
				return err
			}
			hasNative = check.Exists
		}
		return nil
	})
	return hasNative, err
}

// GetTenantCount returns the total number of Tenant nodes in the graph.
func (r *TenantStore) GetTenantCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, "MATCH (t:Tenant) RETURN count(t) AS count", nil)
		if err != nil {
			return err
		}
		defer res.Close()
		if res.Next() {
			var row struct {
				Count int64 `json:"count"`
			}
			if err := res.Scan(&row); err != nil {
				return err
			}
			count = row.Count
		}
		return nil
	})
	return count, err
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
// If alias is empty (""), it looks up the native default tenant (isNative: true).
func (r *TenantStore) GetPublicTenantAuthConfig(ctx context.Context, alias string) (*PublicTenantAuthConfig, error) {
	var query string
	var params map[string]any

	if alias == "" {
		query = `
			MATCH (t:Tenant {isNative: true})
			OPTIONAL MATCH (t)-[:USES_IDP]->(i:IdpProvider)
			RETURN t.id AS tenant_id, t.name AS name, t.alias AS alias, i.id AS idp_id, i.name AS idp_name, i.type AS idp_type
		`
		params = nil
	} else {
		query = `
			MATCH (t:Tenant {alias: $alias})
			OPTIONAL MATCH (t)-[:USES_IDP]->(i:IdpProvider)
			RETURN t.id AS tenant_id, t.name AS name, t.alias AS alias, i.id AS idp_id, i.name AS idp_name, i.type AS idp_type
		`
		params = map[string]any{"alias": alias}
	}

	var tenantID, name string
	var tenantAlias *string
	var providers []*PublicIdpProvider
	found := false

	err := r.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var row struct {
				TenantID string  `json:"tenant_id"`
				Name     string  `json:"name"`
				Alias    *string `json:"alias"`
				IDPID    *string `json:"idp_id"`
				IDPName  *string `json:"idp_name"`
				IDPType  *string `json:"idp_type"`
			}
			if err := res.Scan(&row); err != nil {
				return err
			}

			found = true
			tenantID = row.TenantID
			name = row.Name
			if row.Alias != nil {
				tenantAlias = row.Alias
			}

			if row.IDPID != nil && row.IDPName != nil && row.IDPType != nil {
				providers = append(providers, &PublicIdpProvider{
					ID:   *row.IDPID,
					Name: *row.IDPName,
					Type: *row.IDPType,
				})
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query tenant auth config: %w", err)
	}

	if !found {
		return nil, fmt.Errorf("tenant not found")
	}

	return &PublicTenantAuthConfig{
		TenantID:  tenantID,
		Name:      name,
		Alias:     tenantAlias,
		Providers: providers,
	}, nil
}
