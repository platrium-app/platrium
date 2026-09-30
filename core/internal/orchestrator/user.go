package orchestrator

import (
	"context"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"
)

// UserOrchestrator manages cross-domain business logic for Users.
type UserOrchestrator struct {
	userStore *identity.UserStore
	fsOps     *fsops.FSOps
}

func NewUserOrchestrator(us *identity.UserStore, fs *fsops.FSOps) *UserOrchestrator {
	return &UserOrchestrator{
		userStore: us,
		fsOps:     fs,
	}
}

// ProvisionUserTx creates a new User and their Drive, reusing an existing GraphDB transaction.
func (o *UserOrchestrator) ProvisionUserTx(ctx context.Context, tx graph.Tx, userId, tenantId, idpId, externalId, email, displayName string) (*identity.User, error) {
	// 1. Create the GraphDB User Node
	user, err := o.userStore.CreateUserTx(ctx, tx, userId, tenantId, idpId, externalId, email, displayName)
	if err != nil {
		return nil, err
	}

	// 2. Create Private Drive using the existing transaction!
	// CreateDriveTx natively creates the [:OWNS] edge.
	_, err = o.fsOps.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
		TenantID: tenantId,
		OwnerID:  user.ID,
		Name:     "My Drive",
		Type:     fsops.DriveTypePrivate,
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}
