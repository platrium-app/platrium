package identity

import (
	"context"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"

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
func (r *TenantStore) CreateTenant(ctx context.Context, name, alias string, isNative bool) (*Tenant, error) {
	if len(alias) < 2 {
		return nil, fmt.Errorf("tenant alias must be at least 2 characters")
	}

	tenantId := nanoid.Must()

	// The query creates the Tenant and the default Local IdP atomically.
	query := `
		// 1. Create the Tenant
		CREATE (t:Tenant {
			id: $id,
			alias: $alias,
			name: $name,
			isNative: $isNative,
			createdAt: timestamp()
		})
		
		// 2. Create default LOCAL IdpConnection and link it
		CREATE (i:IdpConnection {
			id: $idpId,
			type: "LOCAL",
			name: "Platrium Authentication",
			configJSON: "{}"
		})
		CREATE (t)-[:USES_IDP]->(i)

		RETURN t.id AS id, t.alias AS alias, t.name AS name, t.isNative AS isNative, t.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"id":       tenantId,
		"alias":    alias,
		"name":     name,
		"isNative": isNative,
		"idpId":    nanoid.Must(),
	}

	var tenant Tenant
	err := r.store.WriteTx(ctx, func(tx graph.Tx) error {
		// 1. Check constraints: Alias must be unique, and only one native tenant can exist
		checkQuery := `
			OPTIONAL MATCH (t1:Tenant {alias: $alias})
			OPTIONAL MATCH (t2:Tenant {isNative: true})
			RETURN t1 IS NOT NULL AS aliasExists, t2 IS NOT NULL AS nativeExists
		`
		checkRes, err := tx.Query(ctx, checkQuery, map[string]interface{}{"alias": alias})
		if err != nil {
			return fmt.Errorf("failed to check tenant constraints: %w", err)
		}

		var check struct {
			AliasExists  bool `json:"aliasExists"`
			NativeExists bool `json:"nativeExists"`
		}

		if checkRes.Next() {
			if err := checkRes.Scan(&check); err != nil {
				checkRes.Close()
				return fmt.Errorf("failed to scan constraint results: %w", err)
			}
		}
		checkRes.Close()

		if check.AliasExists {
			return fmt.Errorf("a tenant with alias '%s' already exists", alias)
		}

		if isNative && check.NativeExists {
			return fmt.Errorf("a native cluster tenant already exists")
		}

		// 2. Execute Creation
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
