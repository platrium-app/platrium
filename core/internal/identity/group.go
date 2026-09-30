package identity

import (
	"context"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/infra/graph"
)

// Group represents a structural group node in the GraphDB.
type Group struct {
	ID         string `json:"id"`
	IdpID      string `json:"idpId"`
	ExternalID string `json:"externalId"`
	Name       string `json:"name"`
	CreatedAt  int64  `json:"createdAt"`
}

// GroupStore manages Group nodes in the GraphDB.
type GroupStore struct {
	store graph.Graph
}

func NewGroupStore(store graph.Graph) *GroupStore {
	return &GroupStore{store: store}
}

// CreateGroupTx creates a group node within a provided transaction.
func (r *GroupStore) CreateGroupTx(ctx context.Context, tx graph.Tx, tenantId, idpId, externalId, name string) (*Group, error) {
	groupId := nanoid.Must()

	query := `
		MATCH (t:Tenant {id: $tenantId})
		CREATE (g:Group {
			id: $id,
			idpId: $idpId,
			externalId: $externalId,
			name: $name,
			createdAt: timestamp()
		})
		CREATE (t)-[:HAS_GROUP]->(g)
		RETURN g.id AS id, g.idpId AS idpId, g.externalId AS externalId, g.name AS name, g.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"tenantId":   tenantId,
		"id":         groupId,
		"idpId":      idpId,
		"externalId": externalId,
		"name":       name,
	}

	var group Group
	res, err := tx.Query(ctx, query, params)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	if !res.Next() {
		return nil, fmt.Errorf("failed to return created group or tenant not found")
	}

	if err := res.Scan(&group); err != nil {
		return nil, fmt.Errorf("failed to scan group: %w", err)
	}

	return &group, nil
}
