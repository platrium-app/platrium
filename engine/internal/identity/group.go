package identity

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
)

// Group represents a group mirrored from an IdP within a tenant.
type Group struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	IdpID      string    `json:"idp_id"`
	ExternalID string    `json:"external_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}

// GroupStore manages Group records.
type GroupStore struct {
	db *db.DB
}

func NewGroupStore(d *db.DB) *GroupStore {
	return &GroupStore{db: d}
}

// CreateGroupTx creates a group within a provided transaction. The IdP must
// belong to the same tenant as the group.
func (r *GroupStore) CreateGroupTx(ctx context.Context, tx *ent.Tx, tenantId, idpId, externalId, name string) (*Group, error) {
	if ok, err := tx.IdpProvider.Query().Where(idpprovider.ID(idpId), idpprovider.TenantID(tenantId)).Exist(ctx); err != nil {
		return nil, fmt.Errorf("failed to look up idp: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("%w: idp not found in tenant", ErrNotFound)
	}

	g, err := tx.Group.Create().
		SetTenantID(tenantId).
		SetIdpID(idpId).
		SetExternalID(externalId).
		SetName(name).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("%w: a group with this externalId already exists for the idp: %v", ErrConflict, err)
		}
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return &Group{
		ID:         g.ID,
		TenantID:   g.TenantID,
		IdpID:      g.IdpID,
		ExternalID: g.ExternalID,
		Name:       g.Name,
		CreatedAt:  g.CreatedAt,
	}, nil
}
