package fsops

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"platrium/internal/infra/graph"
)

// Folder represents a folder node in the graph database.
type Folder struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ParentID     *string   `json:"parent_id,omitempty"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Drive specific metadata, if this folder acts as a root drive
	DriveMetadata *DriveMetadata `json:"drive_metadata,omitempty"`
}

type DriveMetadata struct {
	DriveType    string `json:"drive_type"`
	StorageUsed  int64  `json:"storage_used"`
	StorageQuota *int64 `json:"storage_quota,omitempty"`
}

// CreateFolder creates a new folder node and attaches it to the parent.
func (f *FSOps) CreateFolder(ctx context.Context, tenantID string, parentID string, name string) (*Folder, error) {
	if name == "" {
		return nil, fmt.Errorf("folder name cannot be empty")
	}
	// TODO: Stub permissions check (CheckPermissions)
	
	newID := uuid.New().String()
	now := time.Now().UTC()

	query := `
		MATCH (p:Resource:Folder {id: $parent_id, tenant_id: $tenant_id})
		CREATE (n:Resource:Folder {
			id: $new_id,
			tenant_id: $tenant_id,
			name: $name,
			created_at: $now,
			updated_at: $now
		})-[:CHILD_OF]->(p)
		RETURN n.id AS id, n.tenant_id AS tenant_id, n.name AS name, n.created_at AS created_at, n.updated_at AS updated_at
	`
	params := map[string]interface{}{
		"parent_id": parentID,
		"tenant_id": tenantID,
		"new_id":    newID,
		"name":      name,
		"now":       now,
	}

	var folder Folder
	err := f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			if err := res.Err(); err != nil {
				return err
			}
			return fmt.Errorf("parent folder not found or access denied")
		}

		if err := res.Scan(&folder); err != nil {
			return err
		}
		
		folder.ParentID = &parentID
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &folder, nil
}
