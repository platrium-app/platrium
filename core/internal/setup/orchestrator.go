package setup

import (
	"context"
	"fmt"
	"log"

	"platrium/internal/identity"
)

// Orchestrator handles cross-domain setup logic like Bootstrapping.
type Orchestrator struct {
	configStore InstanceConfigStore
	tenantStore *identity.TenantStore
}

func NewOrchestrator(configStore InstanceConfigStore, tenantStore *identity.TenantStore) *Orchestrator {
	return &Orchestrator{
		configStore: configStore,
		tenantStore: tenantStore,
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

	// Save the ID to the KV Store instance config
	err = o.configStore.Set(ctx, KeyNativeTenantID, tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to save native tenant ID to KV: %w", err)
	}

	log.Printf("Setup: Successfully bootstrapped Native Tenant (ID: %s)", tenant.ID)
	return nil
}
