package orchestrators

import (
	"context"
	"platrium/internal/identity"
	"platrium/internal/infra/graph"
)

// UserOrchestrator manages cross-domain business logic for Users.
type UserOrchestrator struct {
	userStore *identity.UserStore
	// driveStore *objects.DriveStore // TODO: inject later
}

func NewUserOrchestrator(us *identity.UserStore) *UserOrchestrator {
	return &UserOrchestrator{
		userStore: us,
	}
}

// ProvisionUserTx creates a new User and their My Drive, reusing an existing GraphDB transaction.
func (o *UserOrchestrator) ProvisionUserTx(ctx context.Context, tx graph.Tx, userId, tenantId, idpId, externalId, email, displayName string) (*identity.User, error) {
	// 1. Create the GraphDB User Node
	user, err := o.userStore.CreateUserTx(ctx, tx, userId, tenantId, idpId, externalId, email, displayName)
	if err != nil {
		return nil, err
	}

	// 2. TODO: Create Personal Drive using driveStore.CreateDriveTx(tx, ...)
	// 3. TODO: Link User to Drive

	return user, nil
}
