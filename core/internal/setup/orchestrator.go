package setup

import (
	"context"
	"fmt"
	"log"

	"platrium/internal/fsops"
	"platrium/internal/identity"
)

// TODO: This needs to be split into flows for each thing like user creation etc, would become a huge ahh class otherwise
// Orchestrator handles cross-domain setup logic like Bootstrapping.
type Orchestrator struct {
	configStore InstanceConfigStore
	tenantStore *identity.TenantStore
	userStore   *identity.UserStore
	fsOps       *fsops.FSOps
}

func NewOrchestrator(configStore InstanceConfigStore, tenantStore *identity.TenantStore, userStore *identity.UserStore, fsOps *fsops.FSOps) *Orchestrator {
	return &Orchestrator{
		configStore: configStore,
		tenantStore: tenantStore,
		userStore:   userStore,
		fsOps:       fsOps,
	}
}

// TODO: Handle future horizontal scalability.
// Bootstrap runs on server startup to ensure critical infrastructure like
// the Native Tenant exists.
func (o *Orchestrator) Bootstrap(ctx context.Context) error {
	existingId, err := o.configStore.Get(ctx, KeyNativeTenantID)

	// If we successfully found the native tenant ID, we're done.
	if err == nil && existingId != "" {
		log.Printf("Setup: Native Tenant verified (ID: %s)", existingId)
		return nil
	}

	// If it wasn't found, we need to generate it.
	log.Println("Setup: Native Tenant not found. Generating...")
	tenant, err := o.tenantStore.CreateTenant(ctx, "Platrium Native Tenant")
	if err != nil {
		return fmt.Errorf("failed to create native tenant in GraphDB: %w", err)
	}

	// TODO: User Creation Flow, get preferred email from .env and for others/post-super-admin,
	//	     user says tenant super admin email in new tenant call.
	// Hardcode a default admin user for the native tenant
	user, err := o.userStore.CreateUser(ctx, tenant.ID, "admin@example.com")
	if err != nil {
		return fmt.Errorf("failed to create native super admin: %w", err)
	}
	log.Printf("Setup: Created Native Super Admin (ID: %s, Email: %s)", user.ID, user.Email)

	// Hardcode a private drive creation for the user (to be moved to user creation flow later)
	drive, err := o.fsOps.CreatePrivateDrive(ctx, tenant.ID, user.ID)
	if err != nil {
		return fmt.Errorf("failed to create private drive for native super admin: %w", err)
	}
	log.Printf("Setup: Created Private Drive (ID: %s) for Native Super Admin", drive.ID)

	// Save the ID to the KV Store instance config
	err = o.configStore.Set(ctx, KeyNativeTenantID, tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to save native tenant ID to KV: %w", err)
	}

	log.Printf("Setup: Successfully bootstrapped Native Tenant (ID: %s)", tenant.ID)
	return nil
}
