package fsops

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/driveitem"
)

// Item kinds, as stored in drive_items.kind.
const (
	KindFolder = "FOLDER"
	KindFile   = "FILE"
)

type DriveItemRecord struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"` // KindFolder or KindFile
	Size      *int64    `json:"size,omitempty"`
	MimeType  *string   `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsFolder reports whether the item is a folder (including a drive root).
func (r *DriveItemRecord) IsFolder() bool { return r.Kind == KindFolder }

func recordFromEnt(i *ent.DriveItem) *DriveItemRecord {
	return &DriveItemRecord{
		ID:        i.ID,
		TenantID:  i.TenantID,
		ParentID:  i.ParentID,
		Name:      i.Name,
		Kind:      string(i.Kind),
		Size:      i.Size,
		MimeType:  i.MimeType,
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

// GetItem fetches a single item by ID, ensuring tenant isolation.
func (f *FSOps) GetItem(ctx context.Context, tenantId, itemId string) (*DriveItemRecord, error) {
	i, err := f.db.DriveItem.Query().
		Where(driveitem.ID(itemId), driveitem.TenantID(tenantId)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: item not found or access denied", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch item: %w", err)
	}
	return recordFromEnt(i), nil
}

// GetItemPath returns the folders above an item, ordered root first, for breadcrumbs.
// The item itself is not included.
func (f *FSOps) GetItemPath(ctx context.Context, tenantId, itemId string) ([]*Folder, error) {
	ids, err := f.ancestorIDs(ctx, f.db, tenantId, itemId)
	if err != nil {
		return nil, err
	}
	if len(ids) <= 1 {
		return []*Folder{}, nil
	}
	ancestors := ids[1:] // drop the item itself; nearest parent first

	rows, err := f.db.DriveItem.Query().
		Where(driveitem.IDIn(ancestors...), driveitem.TenantID(tenantId), driveitem.KindEQ(driveitem.KindFOLDER)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load path folders: %w", err)
	}
	byID := make(map[string]*ent.DriveItem, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	folders := make([]*Folder, 0, len(ancestors))
	for _, id := range slices.Backward(ancestors) { // root first
		if r, ok := byID[id]; ok {
			folders = append(folders, folderFromEnt(r))
		}
	}
	return folders, nil
}

// GetFolderContents fetches the direct children of a folder with cursor pagination.
func (f *FSOps) GetFolderContents(ctx context.Context, tenantId, folderId string, limit int, after string) ([]*DriveItemRecord, error) {
	q := f.db.DriveItem.Query().
		Where(driveitem.TenantID(tenantId), driveitem.ParentID(folderId))
	if after != "" {
		q = q.Where(driveitem.IDGT(after))
	}

	rows, err := q.Order(driveitem.ByID()).Limit(limit).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list folder contents: %w", err)
	}

	items := make([]*DriveItemRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, recordFromEnt(r))
	}
	return items, nil
}

// GetFolderContentsTotalCount counts the total children of a folder.
func (f *FSOps) GetFolderContentsTotalCount(ctx context.Context, tenantId, folderId string) (int, error) {
	return f.db.DriveItem.Query().
		Where(driveitem.TenantID(tenantId), driveitem.ParentID(folderId)).
		Count(ctx)
}

// RenameItem renames a file or folder.
func (f *FSOps) RenameItem(ctx context.Context, tenantID string, itemID string, newName string) (*DriveItemRecord, error) {
	if newName == "" {
		return nil, fmt.Errorf("%w: new name cannot be empty", ErrInvalid)
	}

	// TODO: Stub permissions check (CheckPermissions)

	// updated_at is bumped by the schema on every update.
	n, err := f.db.DriveItem.Update().
		Where(driveitem.ID(itemID), driveitem.TenantID(tenantID)).
		SetName(newName).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to rename item: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("%w: item not found", ErrNotFound)
	}

	return f.GetItem(ctx, tenantID, itemID)
}

// MoveItem moves an item to a new parent folder within the same drive.
//
// Structural changes take a row lock on the item's drive, so two concurrent
// moves can never combine into a cycle (A into B while B into A). The lock is
// the one portable concurrency primitive across Postgres, MySQL and MariaDB;
// SQLite serializes writers on its own.
func (f *FSOps) MoveItem(ctx context.Context, tenantID string, itemID string, newParentID string) (*DriveItemRecord, error) {
	// TODO: Stub permissions check (CheckPermissions)

	if itemID == newParentID {
		return nil, fmt.Errorf("%w: cannot move an item into itself", ErrInvalid)
	}

	// READ COMMITTED so that reads after the lock see other moves' committed
	// work; MySQL/MariaDB's default REPEATABLE READ would keep serving the
	// snapshot taken by the first read and let two moves form a cycle.
	err := f.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *ent.Tx) error {
		item, err := tx.DriveItem.Query().
			Where(driveitem.ID(itemID), driveitem.TenantID(tenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item not found", ErrNotFound)
			}
			return err
		}

		lock := tx.Drive.Query().Where(drive.ID(item.DriveID), drive.TenantID(tenantID))
		if f.db.RowLocks() {
			lock = lock.ForUpdate()
		}
		if _, err := lock.Only(ctx); err != nil {
			return fmt.Errorf("failed to lock drive: %w", err)
		}

		// Re-read under the lock: a concurrent move may have changed what we saw.
		item, err = tx.DriveItem.Query().
			Where(driveitem.ID(itemID), driveitem.TenantID(tenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item not found", ErrNotFound)
			}
			return err
		}
		if item.ParentID == nil {
			return fmt.Errorf("%w: a drive root cannot be moved", ErrInvalid)
		}

		dest, err := tx.DriveItem.Query().
			Where(driveitem.ID(newParentID), driveitem.TenantID(tenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: new parent not found", ErrNotFound)
			}
			return err
		}
		if dest.Kind != driveitem.KindFOLDER {
			return fmt.Errorf("%w: destination must be a folder", ErrInvalid)
		}
		if dest.DriveID != item.DriveID {
			return fmt.Errorf("%w: moving between drives is not supported", ErrInvalid)
		}

		// Cycle check: the destination's ancestor chain must not contain the item.
		chain, err := f.ancestorIDs(ctx, tx, tenantID, dest.ID)
		if err != nil {
			return err
		}
		if slices.Contains(chain, item.ID) {
			return fmt.Errorf("%w: cannot move a folder into itself or its own subfolder", ErrInvalid)
		}

		return tx.DriveItem.UpdateOneID(item.ID).SetParentID(dest.ID).Exec(ctx)
	})
	if err != nil {
		return nil, err
	}

	return f.GetItem(ctx, tenantID, itemID)
}

// GetFolderChanges returns all children of a folder that were created or updated after a certain time.
func (f *FSOps) GetFolderChanges(ctx context.Context, tenantId, folderId string, since time.Time) ([]*DriveItemRecord, error) {
	rows, err := f.db.DriveItem.Query().
		Where(driveitem.TenantID(tenantId), driveitem.ParentID(folderId), driveitem.UpdatedAtGT(since.UTC())).
		Order(driveitem.ByUpdatedAt(), driveitem.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list folder changes: %w", err)
	}

	items := make([]*DriveItemRecord, 0, len(rows))
	for _, r := range rows {
		items = append(items, recordFromEnt(r))
	}
	return items, nil
}
