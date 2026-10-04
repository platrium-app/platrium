package fsops

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/user"
)

type DriveType string

const (
	DriveTypePrivate DriveType = "PRIVATE"
	DriveTypeShared  DriveType = "SHARED"
)

type Drive struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	TenantID     string    `json:"tenant_id"`
	OwnerID      string    `json:"owner_id,omitempty"` // the owning user; empty for shared drives
	Type         DriveType `json:"type"`               // "PRIVATE" or "SHARED"
	StorageUsed  int64     `json:"storage_used"`
	StorageQuota int64     `json:"storage_quota"` // 0 means unlimited
	CreatedAt    time.Time `json:"created_at"`
	// Caps is what the calling principal may do with the drive.
	Caps authz.Capability `json:"-"`
}

func driveFromEnt(d *ent.Drive, caps authz.Capability) *Drive {
	var quota int64
	if d.StorageQuota != nil {
		quota = *d.StorageQuota
	}
	var owner string
	if d.OwnerID != nil {
		owner = *d.OwnerID
	}
	return &Drive{
		Caps:         caps,
		ID:           d.ID,
		Name:         d.Name,
		TenantID:     d.TenantID,
		OwnerID:      owner,
		Type:         DriveType(d.Type),
		StorageUsed:  d.StorageUsed,
		StorageQuota: quota,
		CreatedAt:    d.CreatedAt,
	}
}

// NormalizeDriveName trims a drive name. Shared-drive names are unique per
// tenant, ignoring case.
func NormalizeDriveName(name string) string { return strings.TrimSpace(name) }

// CreateDriveParams encapsulates inputs for creating a new Drive.
type CreateDriveParams struct {
	TenantID string
	// OwnerID is the owning user. It is required for PRIVATE drives and must be
	// empty for SHARED ones, which belong to the tenant.
	OwnerID string
	Name    string
	Type    DriveType // "PRIVATE" or "SHARED"
}

// CreateDriveTx creates a drive and its root folder within a given transaction.
// The root folder shares the drive's ID, so a drive ID can be used anywhere a
// folder ID is accepted.
//
// This only writes rows. Whether a caller may create a drive, and who can open
// it afterwards, are decided by the callers (see orchestrator.DriveOrchestrator).
func (f *FSOps) CreateDriveTx(ctx context.Context, tx *ent.Tx, params CreateDriveParams) (*Drive, error) {
	name := NormalizeDriveName(params.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: drive name cannot be empty", ErrInvalid)
	}

	driveID := nanoid.Must()
	create := tx.Drive.Create().
		SetID(driveID).
		SetTenantID(params.TenantID).
		SetName(name)

	switch params.Type {
	case DriveTypePrivate:
		// Tenant isolation: the owner must exist in the drive's tenant.
		owned, err := tx.User.Query().Where(user.ID(params.OwnerID), user.TenantID(params.TenantID)).Exist(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to look up drive owner: %w", err)
		}
		if !owned {
			return nil, fmt.Errorf("%w: owner not found in tenant", ErrNotFound)
		}
		create.SetType(drive.TypePRIVATE).SetOwnerID(params.OwnerID)
	case DriveTypeShared:
		if params.OwnerID != "" {
			return nil, fmt.Errorf("%w: a shared drive belongs to the tenant, not a user", ErrInvalid)
		}
		create.SetType(drive.TypeSHARED).SetSharedNameKey(strings.ToLower(name))
	default:
		return nil, fmt.Errorf("%w: invalid or missing drive type", ErrInvalid)
	}

	d, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) && params.Type == DriveTypeShared {
			return nil, fmt.Errorf("%w: a shared drive named %q already exists", ErrConflict, name)
		}
		return nil, fmt.Errorf("failed to create drive: %w", err)
	}

	if _, err := tx.DriveItem.Create().
		SetID(driveID).
		SetTenantID(params.TenantID).
		SetDriveID(driveID).
		SetKind("FOLDER").
		SetName(name).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("failed to create drive root folder: %w", err)
	}

	return driveFromEnt(d, authz.AllCaps), nil
}

// DiscardNewDrive removes a drive that was just created and never used: its
// root folder and the drive row. It refuses if anything was added to the drive
// since, so it can never destroy content. It exists to undo a half-finished
// creation.
func (f *FSOps) DiscardNewDrive(ctx context.Context, tenantID, driveID string) error {
	return f.db.WithTx(ctx, func(tx *ent.Tx) error {
		n, err := tx.DriveItem.Query().Where(driveitem.DriveID(driveID), driveitem.TenantID(tenantID)).Count(ctx)
		if err != nil {
			return err
		}
		if n > 1 { // the root itself is the only item a fresh drive has
			return fmt.Errorf("%w: drive is not empty", ErrInvalid)
		}
		if _, err := tx.DriveItem.Delete().Where(driveitem.ID(driveID), driveitem.TenantID(tenantID)).Exec(ctx); err != nil {
			return err
		}
		_, err = tx.Drive.Delete().Where(drive.ID(driveID), drive.TenantID(tenantID)).Exec(ctx)
		return err
	})
}

// CreateDrive creates a drive and its root folder in its own transaction.
func (f *FSOps) CreateDrive(ctx context.Context, params CreateDriveParams) (*Drive, error) {
	var created *Drive
	err := f.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		created, err = f.CreateDriveTx(ctx, tx, params)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create drive: %w", err)
	}
	return created, nil
}

// GetUserDrives lists the drives the actor can open: the ones they own, plus
// shared drives they hold access to (LIST on the drive root).
// Drives that are only shared inside someone's private drive appear through
// shared-with-me, not here.
func (f *FSOps) GetUserDrives(ctx context.Context, p authz.Principal) ([]*Drive, error) {
	if err := requireActor(p); err != nil {
		return nil, err
	}

	owned, err := f.db.Drive.Query().
		Where(drive.TenantID(p.TenantID), drive.OwnerID(p.UserID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user drives: %w", err)
	}
	driveCaps := make(map[string]authz.Capability, len(owned))
	for _, d := range owned {
		driveCaps[d.ID] = authz.AllCaps // the owner holds everything
	}

	tenantOwned, err := f.db.Drive.Query().
		Where(drive.TenantID(p.TenantID), drive.TypeEQ(drive.TypeSHARED)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shared drives: %w", err)
	}
	if len(tenantOwned) > 0 {
		ids := make([]string, len(tenantOwned))
		for i, d := range tenantOwned {
			ids[i] = d.ID // a drive's root item shares its ID
		}
		caps, err := f.authz.CapsMany(ctx, p, ids)
		if err != nil {
			return nil, err
		}
		for _, d := range tenantOwned {
			if caps[d.ID] != 0 && caps[d.ID].Has(authz.CapList) {
				owned = append(owned, d)
				driveCaps[d.ID] = caps[d.ID]
			}
		}
	}

	slices.SortFunc(owned, func(a, b *ent.Drive) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	drives := make([]*Drive, 0, len(owned))
	for _, d := range owned {
		drives = append(drives, driveFromEnt(d, driveCaps[d.ID]))
	}
	return drives, nil
}

// IsSharedDriveRoot reports whether an item is the root of a shared drive, which
// is where drive members are managed. Requires LIST on the item.
func (f *FSOps) IsSharedDriveRoot(ctx context.Context, p authz.Principal, itemID string) (bool, error) {
	if err := f.Require(ctx, p, itemID, authz.CapList); err != nil {
		return false, err
	}
	return f.db.DriveItem.Query().
		Where(driveitem.ID(itemID), driveitem.TenantID(p.TenantID), driveitem.ParentIDIsNil(),
			driveitem.HasDriveWith(drive.TypeEQ(drive.TypeSHARED))).
		Exist(ctx)
}
