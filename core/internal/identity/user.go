package identity

import (
	"context"
	"fmt"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/infra/graph"
)

// User represents a structural identity node in the GraphDB used strictly for Authorization.
type User struct {
	ID          string `json:"id"`
	IdpID       string `json:"idpId"`
	ExternalID  string `json:"externalId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	CreatedAt   int64  `json:"createdAt"`
}

// UserStore manages User nodes in the GraphDB.
type UserStore struct {
	store graph.Graph
}

func NewUserStore(store graph.Graph) *UserStore {
	return &UserStore{store: store}
}

// CreateUserTx safely creates a user node within a provided transaction.
func (r *UserStore) CreateUserTx(ctx context.Context, tx graph.Tx, userId, tenantId, idpId, externalId, email, displayName string) (*User, error) {
	if userId == "" {
		userId = nanoid.Must()
	}

	query := `
		MATCH (t:Tenant {id: $tenantId})
		CREATE (u:User {
			id: $id,
			idpId: $idpId,
			externalId: $externalId,
			email: $email,
			displayName: $displayName,
			createdAt: timestamp()
		})
		CREATE (t)-[:HAS_USER]->(u)
		RETURN u.id AS id, u.idpId AS idpId, u.externalId AS externalId, u.email AS email, u.displayName AS displayName, u.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"tenantId":    tenantId,
		"id":          userId,
		"idpId":       idpId,
		"externalId":  externalId,
		"email":       email,
		"displayName": displayName,
	}

	var user User
	res, err := tx.Query(ctx, query, params)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	if !res.Next() {
		return nil, fmt.Errorf("failed to return created user or tenant not found")
	}

	if err := res.Scan(&user); err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	return &user, nil
}
