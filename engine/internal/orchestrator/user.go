package orchestrator

import (
	"context"

	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db/ent"
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

// ProvisionUserTx creates a new User and their private Drive, reusing an existing transaction.
func (o *UserOrchestrator) ProvisionUserTx(ctx context.Context, tx *ent.Tx, p identity.CreateUserParams) (*identity.User, error) {
	user, err := o.userStore.CreateUserTx(ctx, tx, p)
	if err != nil {
		return nil, err
	}

	_, err = o.fsOps.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
		TenantID: p.TenantID,
		OwnerID:  user.ID,
		Name:     "My Drive",
		Type:     fsops.DriveTypePrivate,
	})
	if err != nil {
		return nil, err
	}

	return user, nil
}
