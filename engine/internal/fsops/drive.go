package fsops

import (
	"context"
	"fmt"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
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
	OwnerID      string    `json:"owner_id"`
	Type         DriveType `json:"type"` // "PRIVATE" or "SHARED"
	StorageUsed  int64     `json:"storage_used"`
	StorageQuota int64     `json:"storage_quota"` // 0 means unlimited
	CreatedAt    time.Time `json:"created_at"`
}

func driveFromEnt(d *ent.Drive) *Drive {
	var quota int64
	if d.StorageQuota != nil {
		quota = *d.StorageQuota
	}
	return &Drive{
		ID:           d.ID,
		Name:         d.Name,
		TenantID:     d.TenantID,
		OwnerID:      d.OwnerID,
		Type:         DriveType(d.Type),
		StorageUsed:  d.StorageUsed,
		StorageQuota: quota,
		CreatedAt:    d.CreatedAt,
	}
}

// CreateDriveParams encapsulates inputs for creating a new Drive.
type CreateDriveParams struct {
	TenantID string
	OwnerID  string
	Name     string
	Type     DriveType // "PRIVATE" or "SHARED"
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

	// Tenant isolation: the owner must exist in the drive's tenant.
	owned, err := tx.User.Query().Where(user.ID(params.OwnerID), user.TenantID(params.TenantID)).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to look up drive owner: %w", err)
	}
	if !owned {
		return nil, fmt.Errorf("failed to create drive: owner not found in tenant")
	}

	driveID := nanoid.Must()
	d, err := tx.Drive.Create().
		SetID(driveID).
		SetTenantID(params.TenantID).
		SetOwnerID(params.OwnerID).
		SetName(params.Name).
		SetType(drive.Type(params.Type)).
		Save(ctx)
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

// GetUserDrives fetches all drives owned by a user within a tenant.
// TODO: include shared drives once drive access grants exist.
func (f *FSOps) GetUserDrives(ctx context.Context, tenantId, userId string) ([]*Drive, error) {
	rows, err := f.db.Drive.Query().
		Where(drive.TenantID(tenantId), drive.OwnerID(userId)).
		Order(drive.ByCreatedAt(), drive.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user drives: %w", err)
	}

	drives := make([]*Drive, 0, len(rows))
	for _, d := range rows {
		drives = append(drives, driveFromEnt(d))
	}
	return drives, nil
}
