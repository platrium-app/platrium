package fsops

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/driveitem"
)

// Item kinds, as stored in drive_items.kind.
const (
	KindFolder = "FOLDER"
	KindFile   = "FILE"
)

// listBatch is how many children are read per round when filtering a folder
// listing by visibility.
const listBatch = 100

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
	// Caps is what the calling principal may do with the item, so clients can
	// show or hide actions without guessing. Zero when not computed.
	Caps authz.Capability `json:"-"`
}

// IsFolder reports whether the item is a folder (including a drive root).
func (r *DriveItemRecord) IsFolder() bool { return r.Kind == KindFolder }

func recordFromEnt(i *ent.DriveItem, caps authz.Capability) *DriveItemRecord {
	return &DriveItemRecord{
		Caps:      caps,
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

// GetItem fetches a single item by ID. Requires VIEW. Anonymous callers may
// read items that are shared publicly.
func (f *FSOps) GetItem(ctx context.Context, p authz.Principal, itemId string) (*DriveItemRecord, error) {
	tenantID, caps, err := f.readAccess(ctx, p, itemId, authz.CapView)
	if err != nil {
		return nil, err
	}
	i, err := f.db.DriveItem.Query().
		Where(driveitem.ID(itemId), driveitem.TenantID(tenantID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: item not found or access denied", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch item: %w", err)
	}
	return recordFromEnt(i, caps), nil
}

// GetItems fetches several items in one round trip, skipping any the actor
// cannot see (LIST). The result follows the order of ids. Signed-in callers only.
func (f *FSOps) GetItems(ctx context.Context, p authz.Principal, ids []string) ([]*DriveItemRecord, error) {
	if err := requireActor(p); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	caps, err := f.authz.CapsMany(ctx, p, ids)
	if err != nil {
		return nil, err
	}
	rows, err := f.db.DriveItem.Query().
		Where(driveitem.IDIn(ids...), driveitem.TenantID(p.TenantID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch items: %w", err)
	}
	byID := make(map[string]*ent.DriveItem, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	out := make([]*DriveItemRecord, 0, len(ids))
	for _, id := range ids {
		if r, ok := byID[id]; ok && caps[id] != 0 && caps[id].Has(authz.CapList) {
			out = append(out, recordFromEnt(r, caps[id]))
		}
	}
	return out, nil
}

// GetItemPath returns the folders above an item, root first, for breadcrumbs.
// The item itself is not included. Requires LIST on the item.
//
// A user who was given access to a folder deep in a tree must not learn the
// names of the folders above it, so the path starts at the topmost ancestor in
// the unbroken run of visible ones above the item.
func (f *FSOps) GetItemPath(ctx context.Context, p authz.Principal, itemId string) ([]*Folder, error) {
	tenantID, _, err := f.readAccess(ctx, p, itemId, authz.CapList)
	if err != nil {
		return nil, err
	}
	ids, err := f.ancestorIDs(ctx, f.db, tenantID, itemId)
	if err != nil {
		return nil, err
	}
	if len(ids) <= 1 {
		return []*Folder{}, nil
	}
	ancestors := ids[1:] // drop the item itself; nearest parent first

	caps, err := f.authz.CapsMany(ctx, p, ancestors)
	if err != nil {
		return nil, err
	}
	visible := ancestors[:0:0]
	for _, id := range ancestors {
		if !caps[id].Has(authz.CapList) || caps[id] == 0 {
			break
		}
		visible = append(visible, id)
	}
	if len(visible) == 0 {
		return []*Folder{}, nil
	}

	rows, err := f.db.DriveItem.Query().
		Where(driveitem.IDIn(visible...), driveitem.TenantID(tenantID), driveitem.KindEQ(driveitem.KindFOLDER)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load path folders: %w", err)
	}
	byID := make(map[string]*ent.DriveItem, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}

	folders := make([]*Folder, 0, len(visible))
	for _, id := range slices.Backward(visible) { // root first
		if r, ok := byID[id]; ok {
			folders = append(folders, folderFromEnt(r, caps[id]))
		}
	}
	return folders, nil
}

// listable keeps the children p may see and returns the capabilities p holds
// on each of them. One batched check covers the whole page.
func (f *FSOps) listable(ctx context.Context, p authz.Principal, rows []*ent.DriveItem) ([]*ent.DriveItem, map[string]authz.Capability, error) {
	if len(rows) == 0 {
		return nil, nil, nil
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	caps, err := f.authz.CapsMany(ctx, p, ids)
	if err != nil {
		return nil, nil, err
	}

	out := make([]*ent.DriveItem, 0, len(rows))
	for _, r := range rows {
		if caps[r.ID] != 0 && caps[r.ID].Has(authz.CapList) {
			out = append(out, r)
		}
	}
	return out, caps, nil
}

// GetFolderContents lists the direct children of a folder with cursor
// pagination. Requires LIST on the folder; children restricted from it are
// left out, and pages are still filled to the requested size.
func (f *FSOps) GetFolderContents(ctx context.Context, p authz.Principal, folderId string, limit int, after string) ([]*DriveItemRecord, error) {
	tenantID, _, err := f.readAccess(ctx, p, folderId, authz.CapList)
	if err != nil {
		return nil, err
	}

	batch := max(limit, listBatch)
	items := make([]*DriveItemRecord, 0, limit)
	cursor := after
	for len(items) < limit {
		q := f.db.DriveItem.Query().
			Where(driveitem.TenantID(tenantID), driveitem.ParentID(folderId))
		if cursor != "" {
			q = q.Where(driveitem.IDGT(cursor))
		}
		rows, err := q.Order(driveitem.ByID()).Limit(batch).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list folder contents: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		visible, caps, err := f.listable(ctx, p, rows)
		if err != nil {
			return nil, err
		}
		for _, r := range visible {
			if len(items) == limit {
				break
			}
			items = append(items, recordFromEnt(r, caps[r.ID]))
		}
		cursor = rows[len(rows)-1].ID
		if len(rows) < batch {
			break
		}
	}
	return items, nil
}

// GetFolderContentsTotalCount counts the children of a folder the actor may
// see. Requires LIST on the folder. Inheriting children are counted in one
// query; only the usually few restricted ones are checked individually.
func (f *FSOps) GetFolderContentsTotalCount(ctx context.Context, p authz.Principal, folderId string) (int, error) {
	tenantID, _, err := f.readAccess(ctx, p, folderId, authz.CapList)
	if err != nil {
		return 0, err
	}

	inheriting, err := f.db.DriveItem.Query().
		Where(driveitem.TenantID(tenantID), driveitem.ParentID(folderId), driveitem.InheritPerms(true)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	restricted, err := f.db.DriveItem.Query().
		Where(driveitem.TenantID(tenantID), driveitem.ParentID(folderId), driveitem.InheritPerms(false)).
		All(ctx)
	if err != nil {
		return 0, err
	}
	visible, _, err := f.listable(ctx, p, restricted)
	if err != nil {
		return 0, err
	}
	return inheriting + len(visible), nil
}

// RenameItem renames a file or folder. Requires EDIT.
func (f *FSOps) RenameItem(ctx context.Context, p authz.Principal, itemID string, newName string) (*DriveItemRecord, error) {
	if newName == "" {
		return nil, fmt.Errorf("%w: new name cannot be empty", ErrInvalid)
	}
	if err := f.Require(ctx, p, itemID, authz.CapEdit); err != nil {
		return nil, err
	}

	// updated_at is bumped by the schema on every update.
	n, err := f.db.DriveItem.Update().
		Where(driveitem.ID(itemID), driveitem.TenantID(p.TenantID)).
		SetName(newName).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to rename item: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("%w: item not found", ErrNotFound)
	}

	return f.GetItem(ctx, p, itemID)
}

// MoveItem moves an item to a new parent folder within the same drive.
// Requires MOVE on the item and CREATE on the destination.
//
// Structural changes take a row lock on the item's drive, so two concurrent
// moves can never combine into a cycle (A into B while B into A). The lock is
// the one portable concurrency primitive across Postgres, MySQL and MariaDB;
// SQLite serializes writers on its own.
func (f *FSOps) MoveItem(ctx context.Context, p authz.Principal, itemID string, newParentID string) (*DriveItemRecord, error) {
	if itemID == newParentID {
		return nil, fmt.Errorf("%w: cannot move an item into itself", ErrInvalid)
	}
	if err := requireActor(p); err != nil {
		return nil, err
	}
	caps, err := f.authz.CapsMany(ctx, p, []string{itemID, newParentID})
	if err != nil {
		return nil, err
	}
	if err := denyUnless(caps[itemID], authz.CapMove, "item"); err != nil {
		return nil, err
	}
	if err := denyUnless(caps[newParentID], authz.CapCreate, "new parent"); err != nil {
		return nil, err
	}
	tenantID := p.TenantID

	// READ COMMITTED so that reads after the lock see other moves' committed
	// work; MySQL/MariaDB's default REPEATABLE READ would keep serving the
	// snapshot taken by the first read and let two moves form a cycle.
	err = f.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *ent.Tx) error {
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

	return f.GetItem(ctx, p, itemID)
}

// GetFolderChanges returns the children of a folder that were created or
// updated after a certain time, minus any the actor may not see. Requires LIST
// on the folder.
func (f *FSOps) GetFolderChanges(ctx context.Context, p authz.Principal, folderId string, since time.Time) ([]*DriveItemRecord, error) {
	if err := f.Require(ctx, p, folderId, authz.CapList); err != nil {
		return nil, err
	}
	rows, err := f.db.DriveItem.Query().
		Where(driveitem.TenantID(p.TenantID), driveitem.ParentID(folderId), driveitem.UpdatedAtGT(since.UTC())).
		Order(driveitem.ByUpdatedAt(), driveitem.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list folder changes: %w", err)
	}
	visible, caps, err := f.listable(ctx, p, rows)
	if err != nil {
		return nil, err
	}

	items := make([]*DriveItemRecord, 0, len(visible))
	for _, r := range visible {
		items = append(items, recordFromEnt(r, caps[r.ID]))
	}
	return items, nil
}
