package orchestrator

import (
	"context"
	"fmt"
	"platrium/internal/auth"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"

	"platrium/internal/auth/protocol/local"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"
)

// TenantOrchestrator orchestrates complex multi-domain operations for Tenants.
type TenantOrchestrator struct {
	graphStore       graph.Graph
	tenantStore      *identity.TenantStore
	idpStore         *auth.IdpStore
	userOrchestrator *UserOrchestrator
	localUserStore   *local.LocalUserStore
}

func NewTenantOrchestrator(
	g graph.Graph,
	ts *identity.TenantStore,
	is *auth.IdpStore,
	uo *UserOrchestrator,
	lus *local.LocalUserStore,
) *TenantOrchestrator {
	return &TenantOrchestrator{
		graphStore:       g,
		tenantStore:      ts,
		idpStore:         is,
		userOrchestrator: uo,
		localUserStore:   lus,
	}
}

// HasNativeTenant checks if a native tenant already exists in the graph.
func (m *TenantOrchestrator) HasNativeTenant(ctx context.Context) (bool, error) {
	return m.tenantStore.HasNativeTenant(ctx)
}

// ProvisionNewTenant handles the entire atomic lifecycle of setting up a new organization.
// It creates the Tenant, provisions the fallback IdP, and creates the Super Admin.
func (m *TenantOrchestrator) ProvisionNewTenant(ctx context.Context, name, alias, adminEmail, adminPassword string, isNative bool) (*identity.Tenant, error) {
	if len(alias) < 2 {
		return nil, fmt.Errorf("tenant alias must be at least 2 characters")
	}

	tenantId := nanoid.Must()
	idpId := nanoid.Must()
	userId := nanoid.Must()

	// 1. Write-Harmless-First: Set KV Store password with a 10-minute TTL
	err := m.localUserStore.CreateTemporaryUser(ctx, userId, adminPassword, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to pre-provision local user credentials: %w", err)
	}

	var createdTenant identity.Tenant

	// 2. We open EXACTLY ONE database transaction for the entire GraphDB flow!
	err = m.graphStore.WriteTx(ctx, func(tx graph.Tx) error {
		// 0. Check constraints: Alias must be unique, and only one native tenant can exist
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

		// A. Create the Tenant and Local IdP atomically
		// Note: In a fully refactored state, this would be m.tenantStore.CreateTenantTx(tx, ...)
		query1 := `
			CREATE (t:Tenant {
				id: $id,
				alias: $alias,
				name: $name,
				isNative: $isNative,
				createdAt: timestamp()
			})
			CREATE (i:IdpProvider {
				id: $idpId,
				type: "LOCAL",
				name: "Platrium Authentication",
				configJSON: "{}"
			})
			CREATE (t)-[:USES_IDP]->(i)
			RETURN t.id AS id, t.alias AS alias, t.name AS name, t.isNative AS isNative, t.createdAt AS createdAt
		`
		res1, err := tx.Query(ctx, query1, map[string]interface{}{
			"id":       tenantId,
			"alias":    alias,
			"name":     name,
			"isNative": isNative,
			"idpId":    idpId,
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

		// B. Create the Super Admin User via UserOrchestrator!
		// It creates the User node, their Personal Drive, and links everything up atomically.
		user, err := m.userOrchestrator.ProvisionUserTx(ctx, tx, userId, tenantId, idpId, userId, adminEmail, "Admin")
		if err != nil {
			return fmt.Errorf("failed to provision super admin user: %w", err)
		}

		// C. Elevate the user to SUPER_ADMIN role (since ProvisionUserTx is generic, we just add the role property to the HAS_USER edge)
		query2 := `
			MATCH (t:Tenant {id: $tenantId})-[r:HAS_USER]->(u:User {id: $userId})
			SET r.role = "SUPER_ADMIN"
		`
		res2, err := tx.Query(ctx, query2, map[string]interface{}{
			"tenantId": tenantId,
			"userId":   user.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to elevate super admin: %w", err)
		}
		defer res2.Close()

		return nil // Everything commits safely!
	})

	if err != nil {
		// GraphDB rolled back. The KV Store password will expire in 10 minutes.
		return nil, err
	}

	// 3. Finalize: Remove the TTL from the KV Store now that the GraphDB is safe!
	err = m.localUserStore.FinalizeTemporaryUser(ctx, userId)
	if err != nil {
		// The GraphDB succeeded, but making the password permanent failed.
		// It's technically safe because the Admin can just hit "Reset Password" later!
		fmt.Printf("warning: failed to finalize password for user %s: %v\n", userId, err)
	}

	return &createdTenant, nil
}
