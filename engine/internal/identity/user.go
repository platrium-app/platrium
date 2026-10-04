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
	IdpID       string `json:"idp_id"`
	ExternalID  string `json:"external_id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	CreatedAt   int64  `json:"created_at"`
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
		
		// MERGE atomically gets or creates based on the unique combination
		MERGE (u:User {idpId: $idpId, externalId: $externalId})
		ON CREATE SET
			u.id = $id,
			u.email = $email,
			u.displayName = $displayName,
			u.createdAt = timestamp()
		
		// If u.id matches our generated $id, we created it. If not, it already existed!
		WITH t, u
		WHERE u.id = $id
		
		MERGE (t)-[:HAS_USER]->(u)
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
		return nil, fmt.Errorf("failed to create user: either tenant not found or a user with this email/externalId already exists")
	}

	if err := res.Scan(map[string]any{
		"id":          &user.ID,
		"idpId":       &user.IdpID,
		"externalId":  &user.ExternalID,
		"email":       &user.Email,
		"displayName": &user.DisplayName,
		"createdAt":   &user.CreatedAt,
	}); err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	return &user, nil
}

// GetUserByExternalId fetches a user and their parent tenant ID based on their IdP mapping.
func (r *UserStore) GetUserByExternalId(ctx context.Context, idpId, externalId string) (*User, string, error) {
	query := `
		MATCH (t:Tenant)-[:HAS_USER]->(u:User {idpId: $idpId, externalId: $externalId})
		RETURN u.id AS id, u.idpId AS idpId, u.externalId AS externalId, u.email AS email, u.displayName AS displayName, u.createdAt AS createdAt, t.id AS tenantId
	`
	params := map[string]interface{}{
		"idpId":      idpId,
		"externalId": externalId,
	}

	var user User
	var tenantId string

	err := r.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("user not found")
		}

		if err := res.Scan(map[string]any{
			"id":          &user.ID,
			"idpId":       &user.IdpID,
			"externalId":  &user.ExternalID,
			"email":       &user.Email,
			"displayName": &user.DisplayName,
			"createdAt":   &user.CreatedAt,
			"tenantId":    &tenantId,
		}); err != nil {
			return fmt.Errorf("failed to scan user: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, "", err
	}

	return &user, tenantId, nil
}
