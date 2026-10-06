package orchestrator

import (
	"context"
	"errors"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
)

// TenantOrchestrator orchestrates complex multi-domain operations for Tenants.
type TenantOrchestrator struct {
	db             *db.DB
	tenantStore    *identity.TenantStore
	idpStore       *auth.IdpStore
	localUserStore *local.LocalUserStore
}

func NewTenantOrchestrator(
	d *db.DB,
	ts *identity.TenantStore,
	is *auth.IdpStore,
	lus *local.LocalUserStore,
) *TenantOrchestrator {
	return &TenantOrchestrator{
		db:             d,
		tenantStore:    ts,
		idpStore:       is,
		localUserStore: lus,
	}
}

// HasNativeTenant checks if a native tenant already exists.
func (m *TenantOrchestrator) HasNativeTenant(ctx context.Context) (bool, error) {
	return m.tenantStore.HasNativeTenant(ctx)
}

// ProvisionNewTenant handles the entire atomic lifecycle of setting up a new organization.
// It creates the Tenant, provisions the fallback IdP, and creates the Super Admin.
func (m *TenantOrchestrator) ProvisionNewTenant(ctx context.Context, name, alias, adminEmail, adminPassword string, isNative bool) (*identity.Tenant, error) {
	if len(alias) < 2 {
		return nil, fmt.Errorf("tenant alias must be at least 2 characters")
	}

	adminEmail = local.NormalizeLogin(adminEmail)

	tenantId := nanoid.Must()
	idpId := nanoid.Must()
	userId := nanoid.Must()

	// Hash outside the transaction: bcrypt is slow and must not hold a connection.
	passwordHash, err := local.HashPassword(adminPassword)
	if err != nil {
		return nil, err
	}

	var createdTenant *identity.Tenant

	// One database transaction for the entire flow, credentials included.
	err = m.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error

		// A. Tenant and its Local IdP
		createdTenant, err = m.tenantStore.CreateTenantTx(ctx, tx, identity.CreateTenantParams{
			ID:       tenantId,
			Alias:    alias,
			Name:     name,
			IsNative: isNative,
		})
		if err != nil {
			return err
		}

		if _, err := m.idpStore.CreateLocalIdpTx(ctx, tx, tenantId, idpId); err != nil {
			return err
		}

		// B. The Super Admin: a local user with a password and a personal drive.
		if _, err := m.localUserStore.CreateUserTx(ctx, tx, identity.CreateUserParams{
			ID:          userId,
			TenantID:    tenantId,
			IdpID:       idpId,
			ExternalID:  adminEmail,
			Email:       adminEmail,
			DisplayName: "Admin",
			Role:        identity.RoleSuperAdmin,
		}, passwordHash); err != nil {
			return fmt.Errorf("failed to provision super admin user: %w", err)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, identity.ErrConflict) {
			// Rolled back; now it is safe to read why.
			err = fmt.Errorf("%w: %s", identity.ErrConflict, m.tenantStore.DescribeConflict(ctx, alias, isNative))
		}
		return nil, err
	}

	return createdTenant, nil
}
