package fsops

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/driveitem"
)

// Folder represents a folder in a drive's tree.
type Folder struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Caps is what the calling principal may do with the folder.
	Caps authz.Capability `json:"-"`
}

func folderFromEnt(i *ent.DriveItem, caps authz.Capability) *Folder {
	return &Folder{
		Caps:      caps,
		ID:        i.ID,
		TenantID:  i.TenantID,
		ParentID:  i.ParentID,
		Name:      i.Name,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

// CreateFolder creates a new folder and attaches it to the parent. Requires
// CREATE on the parent.
func (f *FSOps) CreateFolder(ctx context.Context, p authz.Principal, parentID string, name string) (*Folder, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: folder name cannot be empty", ErrInvalid)
	}
	parentCaps, err := f.requireCaps(ctx, p, parentID, authz.CapCreate)
	if err != nil {
		return nil, err
	}
	tenantID := p.TenantID

	var folder *Folder
	err = f.db.WithTx(ctx, func(tx *ent.Tx) error {
		parent, err := tx.DriveItem.Query().
			Where(driveitem.ID(parentID), driveitem.TenantID(tenantID), driveitem.KindEQ(driveitem.KindFOLDER)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: parent folder not found or access denied", ErrNotFound)
			}
			return err
		}

		created, err := tx.DriveItem.Create().
			SetTenantID(tenantID).
			SetDriveID(parent.DriveID).
			SetParentID(parent.ID).
			SetKind(driveitem.KindFOLDER).
			SetName(name).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create folder: %w", err)
		}
		folder = folderFromEnt(created, parentCaps) // a new folder inherits exactly its parent's access
		return nil
	})
	if err != nil {
		return nil, err
	}
	return folder, nil
}
