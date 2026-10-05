package setup

import (
	"context"
	"fmt"
	"log"

	"platrium/internal/orchestrator"
)

// TODO: This needs to be split into flows for each thing like user creation etc, would become a huge ahh class otherwise
// Orchestrator handles cross-domain setup logic like Bootstrapping.
type Orchestrator struct {
	configStore        InstanceConfigStore
	tenantOrchestrator *orchestrator.TenantOrchestrator
}

func NewOrchestrator(configStore InstanceConfigStore, tenantOrchestrator *orchestrator.TenantOrchestrator) *Orchestrator {
	return &Orchestrator{
		configStore:        configStore,
		tenantOrchestrator: tenantOrchestrator,
	}
}

// TODO: Handle future horizontal scalability.
// Bootstrap runs on server startup to ensure critical infrastructure like
// the Native Tenant exists.
func (o *Orchestrator) Bootstrap(ctx context.Context) error {
	hasNative, err := o.tenantOrchestrator.HasNativeTenant(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for existing native tenant: %w", err)
	}

	// If we successfully found the native tenant ID, we're done.
	if hasNative {
		log.Println("Setup: Native Tenant verified in database")
		return nil
	}

	// If it wasn't found, we need to generate it.
	log.Println("Setup: Native Tenant not found. Generating...")

	// TODO: User Creation Flow, get preferred email from .env and for others/post-super-admin,
	//	     user says tenant super admin email in new tenant call.
	tenant, err := o.tenantOrchestrator.ProvisionNewTenant(
		ctx,
		"Example Tenant",
		"example",
		"example@platrium.org",
		"12345678",
		true, // Native Tenant is Provisioned during Setup
	)

	if err != nil {
		return err
	}

	log.Printf("Setup: Successfully bootstrapped Native Tenant (ID: %s)", tenant.ID)
	return nil
}
