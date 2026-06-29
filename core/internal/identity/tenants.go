package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
)

// Tenant represents an organization or isolated billing unit.
type Tenant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

// TenantStore manages Tenant nodes in the GraphDB.
type TenantStore struct {
	store graph.Graph
}

func NewTenantStore(store graph.Graph) *TenantStore {
	return &TenantStore{store: store}
}

// CreateTenant creates a new Tenant node in Neo4j.
func (r *TenantStore) CreateTenant(ctx context.Context, name string) (*Tenant, error) {
	tenantId := uuid.New().String()

	query := `
		CREATE (t:Tenant {
			id: $id,
			name: $name,
			createdAt: timestamp()
		})
		RETURN t.id AS id, t.name AS name, t.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"id":       tenantId,
		"name":     name,
	}

	var tenant Tenant
	err := r.store.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("failed to return created tenant")
		}

		if err := res.Scan(&tenant); err != nil {
			return fmt.Errorf("failed to scan tenant: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	return &tenant, nil
}
