package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
)

// DriveOrchestrator creates shared drives. It composes three domains: identity
// (is this caller allowed to), fsops (create the drive) and authz (make the
// creator its first administrator), which is why it is not a method on any one
// of them.
type DriveOrchestrator struct {
	db       *db.DB
	fsOps    *fsops.FSOps
	authz    authz.Authorizer
	users    *identity.UserStore
	policies *identity.PolicyStore
}

func NewDriveOrchestrator(d *db.DB, fs *fsops.FSOps, az authz.Authorizer, us *identity.UserStore, ps *identity.PolicyStore) *DriveOrchestrator {
	return &DriveOrchestrator{db: d, fsOps: fs, authz: az, users: us, policies: ps}
}

// isAdmin reports whether the actor administers their tenant.
func (o *DriveOrchestrator) isAdmin(ctx context.Context, p authz.Principal) (bool, error) {
	users, err := o.users.GetByIDs(ctx, p.TenantID, []string{p.UserID})
	if err != nil {
		return false, err
	}
	u, ok := users[p.UserID]
	return ok && identity.IsAdmin(u.Role), nil
}

func requireSignedIn(p authz.Principal) error {
	if p.IsAnonymous() || p.TenantID == "" {
		return fmt.Errorf("%w: sign in required", authz.ErrForbidden)
	}
	return nil
}

// CanCreateSharedDrive reports whether the actor may create shared drives:
// tenant admins always may, and so may members of the groups the admin has
// allowed. Clients use it to decide whether to offer the action.
func (o *DriveOrchestrator) CanCreateSharedDrive(ctx context.Context, p authz.Principal) (bool, error) {
	if err := requireSignedIn(p); err != nil {
		return false, nil
	}
	if admin, err := o.isAdmin(ctx, p); err != nil || admin {
		return admin, err
	}
	return o.policies.AppliesTo(ctx, p.TenantID, identity.PolicySharedDriveCreators, p.GroupIDs)
}

// CreateSharedDrive creates a shared drive owned by the tenant and makes the
// creator its Drive Admin, so someone can open it and add the others.
//
// The drive and its root folder are created in one transaction. The creator's
// grant goes through the authorizer, which may live in a different store, so it
// cannot join that transaction; if it fails the drive, still empty, is removed.
func (o *DriveOrchestrator) CreateSharedDrive(ctx context.Context, p authz.Principal, name string) (*fsops.Drive, error) {
	if err := requireSignedIn(p); err != nil {
		return nil, err
	}
	allowed, err := o.CanCreateSharedDrive(ctx, p)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("%w: you are not allowed to create shared drives", authz.ErrForbidden)
	}

	var created *fsops.Drive
	err = o.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		created, err = o.fsOps.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
			TenantID: p.TenantID,
			Name:     name,
			Type:     fsops.DriveTypeShared,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	// A drive's root item shares its ID.
	if err := o.authz.GrantInitial(ctx, p.TenantID, created.ID, p.UserID, authz.RoleDriveAdmin); err != nil {
		if discardErr := o.fsOps.DiscardNewDrive(ctx, p.TenantID, created.ID); discardErr != nil {
			err = errors.Join(err, fmt.Errorf("also failed to remove the new drive: %w", discardErr))
		}
		return nil, fmt.Errorf("failed to make you the drive's admin: %w", err)
	}

	created.Caps, err = o.authz.Caps(ctx, p, created.ID)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// SharedDriveCreators returns the groups allowed to create shared drives.
// Tenant admins only.
func (o *DriveOrchestrator) SharedDriveCreators(ctx context.Context, p authz.Principal) ([]string, error) {
	if err := o.requireAdmin(ctx, p); err != nil {
		return nil, err
	}
	return o.policies.Groups(ctx, p.TenantID, identity.PolicySharedDriveCreators)
}

// SetSharedDriveCreators replaces the groups allowed to create shared drives.
// Tenant admins only.
func (o *DriveOrchestrator) SetSharedDriveCreators(ctx context.Context, p authz.Principal, groupIDs []string) error {
	if err := o.requireAdmin(ctx, p); err != nil {
		return err
	}
	return o.policies.SetGroups(ctx, p.TenantID, identity.PolicySharedDriveCreators, groupIDs)
}

func (o *DriveOrchestrator) requireAdmin(ctx context.Context, p authz.Principal) error {
	if err := requireSignedIn(p); err != nil {
		return err
	}
	admin, err := o.isAdmin(ctx, p)
	if err != nil {
		return err
	}
	if !admin {
		return fmt.Errorf("%w: tenant administrators only", authz.ErrForbidden)
	}
	return nil
}
