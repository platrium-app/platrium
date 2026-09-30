package identity

import (
	"context"

	"platrium/internal/infra/graph"
)

// Tenant represents an organization or isolated billing unit.
type Tenant struct {
	ID        string `json:"id"`
	Alias     string `json:"alias"` // e.g., "acme" or "family"
	Name      string `json:"name"`
	IsNative  bool   `json:"isNative"`
	CreatedAt int64  `json:"createdAt"`
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
