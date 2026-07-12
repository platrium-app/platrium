package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"platrium/internal/infra/graph"
)

// User represents a minimal user entity.
// TODO: Future user management and complex auth flows should be moved to a separate usermgmt data plane app.
type User struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenantId"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"createdAt"`
}

// UserStore manages User nodes in the GraphDB.
type UserStore struct {
	store graph.Graph
}

func NewUserStore(store graph.Graph) *UserStore {
	return &UserStore{store: store}
}

// CreateUser creates a new User node in Neo4j and links it to a Tenant.
func (r *UserStore) CreateUser(ctx context.Context, tenantId, email string) (*User, error) {
	userId := uuid.New().String()

	query := `
		MATCH (t:Tenant {id: $tenantId})
		CREATE (u:User {
			id: $id,
			tenantId: $tenantId,
			email: $email,
			createdAt: timestamp()
		})-[:BELONGS_TO]->(t)
		RETURN u.id AS id, u.tenantId AS tenantId, u.email AS email, u.createdAt AS createdAt
	`
	params := map[string]interface{}{
		"tenantId": tenantId,
		"id":       userId,
		"email":    email,
	}

	var user User
	err := r.store.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("failed to return created user or tenant not found")
		}

		if err := res.Scan(&user); err != nil {
			return fmt.Errorf("failed to scan user: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}
