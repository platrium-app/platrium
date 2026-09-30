package orchestrators

import (
	"context"
	"fmt"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// TenantOrchestrator orchestrates complex multi-domain operations for Tenants.
type TenantOrchestrator struct {
	graphStore  graph.Graph
	tenantStore *identity.TenantStore
	idpStore    *identity.IdpStore
	userStore   *identity.UserStore
}

func NewTenantOrchestrator(g graph.Graph, ts *identity.TenantStore, is *identity.IdpStore, us *identity.UserStore) *TenantOrchestrator {
	return &TenantOrchestrator{
		graphStore:  g,
		tenantStore: ts,
		idpStore:    is,
		userStore:   us,
	}
}

// ProvisionNewTenant handles the entire atomic lifecycle of setting up a new organization.
// It creates the Tenant, provisions the fallback IdP, and creates the Super Admin.
func (m *TenantOrchestrator) ProvisionNewTenant(ctx context.Context, name, alias, adminEmail string) (*identity.Tenant, error) {
	if len(alias) < 2 {
		return nil, fmt.Errorf("tenant alias must be at least 2 characters")
	}

	tenantId := nanoid.Must()
	idpId := nanoid.Must()
	userId := nanoid.Must()

	var createdTenant identity.Tenant

	// We open EXACTLY ONE database transaction for the entire flow!
	err := m.graphStore.WriteTx(ctx, func(tx graph.Tx) error {

		// 1. Create the Tenant and Local IdP atomically
		// Note: In a fully refactored state, this would be m.tenantStore.CreateTenantTx(tx, ...)
		query1 := `
			CREATE (t:Tenant {
				id: $id,
				alias: $alias,
				name: $name,
				createdAt: timestamp()
			})
			CREATE (i:IdpConnection {
				id: $idpId,
				type: "LOCAL",
				name: "Platrium Authentication",
				configJSON: "{}"
			})
			CREATE (t)-[:USES_IDP]->(i)
			RETURN t.id AS id, t.alias AS alias, t.name AS name, t.createdAt AS createdAt
		`
		res1, err := tx.Query(ctx, query1, map[string]interface{}{
			"id":    tenantId,
			"alias": alias,
			"name":  name,
			"idpId": idpId,
		})

		if err != nil {
			return fmt.Errorf("failed to create tenant nodes: %w", err)
		}
		defer res1.Close()

		if !res1.Next() {
			return fmt.Errorf("failed to return created tenant")
		}
		if err := res1.Scan(&createdTenant); err != nil {
			return err
		}

		// 2. Create the Super Admin User and link them to the Tenant
		// Note: In a fully refactored state, this would be m.userStore.CreateUserTx(tx, ...)
		// TODO: This stuff needs to go into user manager then we use a Tx.
		query2 := `
			MATCH (t:Tenant {id: $tenantId})
			CREATE (u:User {
				id: $userId,
				email: $email,
				role: "SUPER_ADMIN",
				createdAt: timestamp()
			})
			CREATE (t)-[:HAS_USER]->(u)
		`

		res2, err := tx.Query(ctx, query2, map[string]interface{}{
			"tenantId": tenantId,
			"userId":   userId,
			"email":    adminEmail,
		})
		if err != nil {
			return fmt.Errorf("failed to create super admin: %w", err)
		}
		defer res2.Close()

		// 3. (Future) Create the User's Personal Drive here!

		return nil // Everything commits safely!
	})

	if err != nil {
		return nil, err
	}

	return &createdTenant, nil
}
