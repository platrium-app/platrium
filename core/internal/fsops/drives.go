package fsops

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
)

type Drive struct {
	ID        string `json:"id"`
	OwnerID   string `json:"ownerId"` // Tenant ID or User ID
	Type      string `json:"type"`    // "private" or "shared"
	CreatedAt int64  `json:"createdAt"`
}

// CreatePrivateDrive creates a root PrivateDrive node in Neo4j and links it to the User.
func (f *FSOps) CreatePrivateDrive(ctx context.Context, tenantId, userId string) (*Drive, error) {
	driveId := uuid.New().String()

	query := `
		MATCH (u:User {id: $userId})
		CREATE (d:Resource:Folder:PrivateDrive {
			id: $driveId,
			tenant_id: $tenantId,
			createdAt: timestamp()
		})
		CREATE (u)-[:OWNS]->(d)
		RETURN d.id AS id, d.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"tenantId": tenantId,
		"userId":   userId,
		"driveId":  driveId,
	}

	var drive Drive
	drive.OwnerID = userId
	drive.Type = "private"

	err := f.graph.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
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
		return nil, fmt.Errorf("failed to create private drive: %w", err)
	}

	return &drive, nil
}
