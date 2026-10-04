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
	"platrium/internal/infra/db/ent/user"
)

type DriveType string

const (
	DriveTypePrivate DriveType = "PRIVATE"
	DriveTypeShared  DriveType = "SHARED"
)

// DriveOwnerType says who owns a drive.
type DriveOwnerType string

const (
	// DriveOwnedByUser is a private drive: its owner holds every capability.
	DriveOwnedByUser DriveOwnerType = "USER"
	// DriveOwnedByTenant is a shared drive owned by the organization. Nobody
	// owns it implicitly; access comes from grants, and it outlives its members.
	DriveOwnedByTenant DriveOwnerType = "TENANT"
)

type Drive struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	TenantID     string         `json:"tenant_id"`
	OwnerType    DriveOwnerType `json:"owner_type"`
	OwnerID      string         `json:"owner_id,omitempty"` // empty for tenant-owned drives
	Type         DriveType      `json:"type"`               // "PRIVATE" or "SHARED"
	StorageUsed  int64          `json:"storage_used"`
	StorageQuota int64          `json:"storage_quota"` // 0 means unlimited
	CreatedAt    time.Time      `json:"created_at"`
}

func driveFromEnt(d *ent.Drive) *Drive {
	var quota int64
	if d.StorageQuota != nil {
		quota = *d.StorageQuota
	}
	var owner string
	if d.OwnerID != nil {
		owner = *d.OwnerID
	}
	return &Drive{
		ID:           d.ID,
		Name:         d.Name,
		TenantID:     d.TenantID,
		OwnerType:    DriveOwnerType(d.OwnerType),
		OwnerID:      owner,
		Type:         DriveType(d.Type),
		StorageUsed:  d.StorageUsed,
		StorageQuota: quota,
		CreatedAt:    d.CreatedAt,
	}
}

// CreateDriveParams encapsulates inputs for creating a new Drive.
type CreateDriveParams struct {
	TenantID  string
	OwnerType DriveOwnerType // defaults to DriveOwnedByUser
	OwnerID   string         // required for user-owned drives, empty for tenant-owned
	Name      string
	Type      DriveType // "PRIVATE" or "SHARED"; tenant-owned drives are SHARED
}

// CreateDriveTx creates a drive and its root folder within a given transaction.
// The root folder shares the drive's ID, so a drive ID can be used anywhere a
// folder ID is accepted.
func (f *FSOps) CreateDriveTx(ctx context.Context, tx *ent.Tx, params CreateDriveParams) (*Drive, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("drive name cannot be empty")
	}
	if params.Type != DriveTypePrivate && params.Type != DriveTypeShared {
		return nil, fmt.Errorf("invalid or missing drive type")
	}

	ownerType := params.OwnerType
	if ownerType == "" {
		ownerType = DriveOwnedByUser
	}

	driveID := nanoid.Must()
	create := tx.Drive.Create().
		SetID(driveID).
		SetTenantID(params.TenantID).
		SetOwnerType(string(ownerType)).
		SetName(params.Name).
		SetType(drive.Type(params.Type))

	switch ownerType {
	case DriveOwnedByUser:
		// Tenant isolation: the owner must exist in the drive's tenant.
		owned, err := tx.User.Query().Where(user.ID(params.OwnerID), user.TenantID(params.TenantID)).Exist(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to look up drive owner: %w", err)
		}
		if !owned {
			return nil, fmt.Errorf("failed to create drive: owner not found in tenant")
		}
		create.SetOwnerID(params.OwnerID)
	case DriveOwnedByTenant:
		if params.OwnerID != "" {
			return nil, fmt.Errorf("%w: a tenant-owned drive has no owner user", ErrInvalid)
		}
		if params.Type != DriveTypeShared {
			return nil, fmt.Errorf("%w: a tenant-owned drive must be shared", ErrInvalid)
		}
	default:
		return nil, fmt.Errorf("%w: unknown drive owner type %q", ErrInvalid, ownerType)
	}

	d, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create drive: %w", err)
	}

	if _, err := tx.DriveItem.Create().
		SetID(driveID).
		SetTenantID(params.TenantID).
		SetDriveID(driveID).
		SetKind("FOLDER").
		SetName(params.Name).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("failed to create drive root folder: %w", err)
	}

	return driveFromEnt(d), nil
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
// tenant-owned shared drives they hold access to (LIST on the drive root).
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

	tenantOwned, err := f.db.Drive.Query().
		Where(drive.TenantID(p.TenantID), drive.OwnerType(string(DriveOwnedByTenant))).
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
		drives = append(drives, driveFromEnt(d))
	}
	return drives, nil
}
