package fsops

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/infra/graph"
)

type DriveItemRecord struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	ParentID  *string   `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	Labels    []string  `json:"labels"`
	Size      *int64    `json:"size,omitempty"`
	MimeType  *string   `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetItem fetches a single item by ID, ensuring tenant isolation.
func (f *FSOps) GetItem(ctx context.Context, tenantId, itemId string) (*DriveItemRecord, error) {
	query := `
		MATCH (n:Resource {id: $item_id, tenant_id: $tenant_id})
		OPTIONAL MATCH (n)-[:CHILD_OF]->(p:Resource)
		RETURN 
			n.id AS id,
			n.tenant_id AS tenant_id,
			p.id AS parent_id,
			n.name AS name,
			labels(n) AS labels,
			n.size AS size,
			n.mime_type AS mime_type,
			n.created_at AS created_at,
			n.updated_at AS updated_at
	`
	params := map[string]interface{}{
		"item_id":   itemId,
		"tenant_id": tenantId,
	}

	var item DriveItemRecord
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			if err := res.Err(); err != nil {
				return err
			}
			return fmt.Errorf("item not found or access denied")
		}

		if err := res.Scan(&item); err != nil {
			return fmt.Errorf("failed to scan item: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &item, nil
}

// GetItemPath traverses up the graph to find all ancestors for breadcrumbs.
func (f *FSOps) GetItemPath(ctx context.Context, tenantId, itemId string) ([]*Folder, error) {
	query := `
		MATCH path = shortestPath((child:Resource {id: $item_id, tenant_id: $tenant_id})-[:CHILD_OF*0..]->(root:Folder))
		WHERE NOT (root)-[:CHILD_OF]->()
		WITH nodes(path) AS pathNodes
		
		// Exclude the child itself from the path (it's the first node in pathNodes)
		WITH pathNodes[1..] AS ancestorNodes
		UNWIND ancestorNodes AS n
		WITH n WHERE n:Folder
		RETURN 
			n.id AS id,
			n.tenant_id AS tenant_id,
			n.name AS name,
			n.created_at AS created_at,
			n.updated_at AS updated_at
	`
	params := map[string]interface{}{
		"item_id":   itemId,
		"tenant_id": tenantId,
	}

	var folders []*Folder
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var folder Folder
			if err := res.Scan(&folder); err != nil {
				return fmt.Errorf("failed to scan folder in path: %w", err)
			}
			folders = append(folders, &folder)
		}

		return res.Err()
	})

	if err != nil {
		return nil, err
	}

	// Reverse to order from Root -> Child
	for i, j := 0, len(folders)-1; i < j; i, j = i+1, j-1 {
		folders[i], folders[j] = folders[j], folders[i]
	}

	return folders, nil
}

// GetFolderContents fetches the direct children of a folder with cursor pagination.
func (f *FSOps) GetFolderContents(ctx context.Context, tenantId, folderId string, limit int, after string) ([]*DriveItemRecord, error) {
	query := `
		MATCH (child:Resource)-[:CHILD_OF]->(parent:Folder {id: $folder_id, tenant_id: $tenant_id})
		WHERE ($after = "" OR child.id > $after)
		RETURN 
			child.id AS id,
			child.tenant_id AS tenant_id,
			parent.id AS parent_id,
			child.name AS name,
			labels(child) AS labels,
			child.size AS size,
			child.mime_type AS mime_type,
			child.created_at AS created_at,
			child.updated_at AS updated_at
		ORDER BY child.id ASC
		LIMIT $limit
	`
	params := map[string]interface{}{
		"folder_id": folderId,
		"tenant_id": tenantId,
		"after":     after,
		"limit":     limit,
	}

	var items []*DriveItemRecord
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var item DriveItemRecord
			if err := res.Scan(&item); err != nil {
				return err
			}
			items = append(items, &item)
		}
		return res.Err()
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}

// GetFolderContentsTotalCount counts the total children of a folder.
func (f *FSOps) GetFolderContentsTotalCount(ctx context.Context, tenantId, folderId string) (int, error) {
	query := `
		MATCH (child:Resource)-[:CHILD_OF]->(parent:Folder {id: $folder_id, tenant_id: $tenant_id})
		RETURN count(child) AS total
	`
	params := map[string]interface{}{
		"folder_id": folderId,
		"tenant_id": tenantId,
	}

	var total int
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if res.Next() {
			var result struct {
				Total int `json:"total"`
			}
			if err := res.Scan(&result); err != nil {
				return err
			}
			total = result.Total
		}
		return res.Err()
	})

	return total, err
}

// RenameItem renames a file or folder.
func (f *FSOps) RenameItem(ctx context.Context, tenantID string, itemID string, newName string) (*DriveItemRecord, error) {
	if newName == "" {
		return nil, fmt.Errorf("new name cannot be empty")
	}

	// TODO: Stub permissions check (CheckPermissions)

	query := `
		MATCH (n:Resource {id: $item_id, tenant_id: $tenant_id})
		SET n.name = $new_name
		RETURN n.id
	`
	params := map[string]interface{}{
		"item_id":   itemID,
		"tenant_id": tenantID,
		"new_name":  newName,
	}

	err := f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("item not found")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return f.GetItem(ctx, tenantID, itemID)
}

// MoveItem moves an item to a new parent folder.
func (f *FSOps) MoveItem(ctx context.Context, tenantID string, itemID string, newParentID string) (*DriveItemRecord, error) {
	// TODO: Stub permissions check (CheckPermissions)

	if itemID == newParentID {
		return nil, fmt.Errorf("Cannot move an item into itself")
	}

	// Optimize cycle detection by short-circuiting. The moment Neo4j finds ANY path
	// proving the destination is a descendant of the source, it stops searching.
	query := `
		MATCH (dest:Resource {id: $new_parent_id, tenant_id: $tenant_id})-[:CHILD_OF*0..]->(src:Resource {id: $item_id, tenant_id: $tenant_id})
		RETURN true AS is_cycle LIMIT 1
	`
	params := map[string]interface{}{
		"item_id":       itemID,
		"new_parent_id": newParentID,
		"tenant_id":     tenantID,
	}

	err := f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		// 1. Check for cycles
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}

		var isCycle bool
		if res.Next() {
			isCycle = true
		}
		res.Close()

		if isCycle {
			return fmt.Errorf("Cannot move a folder into itself or its own subfolder")
		}

		// 2. Perform Move
		moveQuery := `
			MATCH (item:Resource {id: $item_id, tenant_id: $tenant_id})-[oldRel:CHILD_OF]->(oldParent:Resource)
			MATCH (newParent:Resource {id: $new_parent_id, tenant_id: $tenant_id})
			DELETE oldRel
			CREATE (item)-[:CHILD_OF]->(newParent)
			RETURN item.id
		`
		moveParams := map[string]interface{}{
			"item_id":       itemID,
			"new_parent_id": newParentID,
			"tenant_id":     tenantID,
		}

		moveRes, err := tx.Query(ctx, moveQuery, moveParams)
		if err != nil {
			return err
		}
		defer moveRes.Close()

		if !moveRes.Next() {
			return fmt.Errorf("item or new parent not found")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return f.GetItem(ctx, tenantID, itemID)
}

// GetFolderChanges returns all children of a folder that were created or updated after a certain time.
func (f *FSOps) GetFolderChanges(ctx context.Context, tenantId, folderId string, since time.Time) ([]*DriveItemRecord, error) {
	query := `
		MATCH (child:Resource)-[:CHILD_OF]->(parent:Folder {id: $folder_id, tenant_id: $tenant_id})
		WHERE child.updated_at > $since
		RETURN 
			child.id AS id,
			child.tenant_id AS tenant_id,
			parent.id AS parent_id,
			child.name AS name,
			labels(child) AS labels,
			child.size AS size,
			child.mime_type AS mime_type,
			child.created_at AS created_at,
			child.updated_at AS updated_at
		ORDER BY child.updated_at ASC
	`
	params := map[string]interface{}{
		"folder_id": folderId,
		"tenant_id": tenantId,
		"since":     since,
	}

	var items []*DriveItemRecord
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var item DriveItemRecord
			if err := res.Scan(&item); err != nil {
				return err
			}
			items = append(items, &item)
		}
		return res.Err()
	})

	if err != nil {
		return nil, err
	}

	return items, nil
}
