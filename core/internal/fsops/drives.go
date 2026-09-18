package fsops

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
)

type DriveType string

const (
	DriveTypePrivate DriveType = "PRIVATE"
	DriveTypeShared  DriveType = "SHARED"
)

type Drive struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	TenantID     string `json:"tenant_id"`
	OwnerID      string    `json:"owner_id"`
	Type         DriveType `json:"type"` // "PRIVATE" or "SHARED"
	StorageUsed  int64     `json:"storage_used"`
	StorageQuota int64     `json:"storage_quota"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateDriveParams encapsulates inputs for creating a new Drive node.
type CreateDriveParams struct {
	TenantID string
	OwnerID  string
	Name     string
	Type     DriveType // "PRIVATE" or "SHARED"
}

// CreateDrive creates a root Drive node in Graph DB and links it to the owner user via [:OWNS].
func (f *FSOps) CreateDrive(ctx context.Context, params CreateDriveParams) (*Drive, error) {
	driveId := uuid.New().String()
	name := params.Name
	if name == "" {
		name = "My Drive"
	}
	driveType := params.Type
	if driveType == "" {
		driveType = DriveTypePrivate
	}

	query := `
		MATCH (u:User {id: $owner_id})
		CREATE (d:Resource:Folder {
			id: $drive_id,
			name: $name,
			tenant_id: $tenant_id,
			owner_id: $owner_id,
			drive_type: $drive_type,
			created_at: datetime()
		})
		CREATE (u)-[:OWNS]->(d)
		RETURN d.id AS id, d.name AS name, d.tenant_id AS tenant_id, d.owner_id AS owner_id, d.drive_type AS type, 0 AS storage_used, 0 AS storage_quota, d.created_at AS created_at
	`
	cypherParams := map[string]interface{}{
		"tenant_id":  params.TenantID,
		"owner_id":   params.OwnerID,
		"drive_id":   driveId,
		"name":       name,
		"drive_type": driveType,
	}

	var drive Drive
	err := f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, cypherParams)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("failed to return created drive or user not found")
		}

		if err := res.Scan(&drive); err != nil {
			return fmt.Errorf("failed to scan drive: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create drive: %w", err)
	}

	return &drive, nil
}

// GetUserDrives fetches all accessible private and shared drives for a user.
func (f *FSOps) GetUserDrives(ctx context.Context, tenantId, userId string) ([]*Drive, error) {
	query := `
		MATCH (u:User {id: $userId, tenantId: $tenantId})-[rel:OWNS|HAS_ACCESS]->(d:Resource)
		RETURN 
			d.id AS id,
			coalesce(d.name, "My Drive") AS name,
			d.tenant_id AS tenant_id,
			coalesce(d.owner_id, $userId) AS owner_id,
			CASE WHEN type(rel) = "OWNS" THEN "PRIVATE" ELSE "SHARED" END AS type,
			coalesce(d.storage_used, 0) AS storage_used,
			coalesce(d.storage_quota, 0) AS storage_quota,
			d.created_at AS created_at
	`
	params := map[string]any{
		"tenantId": tenantId,
		"userId":   userId,
	}

	var drives []*Drive
	err := f.graph.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var drive Drive
			if err := res.Scan(&drive); err != nil {
				return fmt.Errorf("failed to scan drive: %w", err)
			}
			drives = append(drives, &drive)
		}
		return res.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to fetch user drives: %w", err)
	}

	return drives, nil
}
