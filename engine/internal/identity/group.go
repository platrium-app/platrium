package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/group"
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

// GetByIDs returns the groups with the given IDs that belong to the tenant,
// keyed by ID. Unknown IDs, and IDs from other tenants, are simply absent.
func (r *GroupStore) GetByIDs(ctx context.Context, tenantID string, ids []string) (map[string]*Group, error) {
	out := make(map[string]*Group, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.Group.Query().Where(group.IDIn(ids...), group.TenantID(tenantID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}
	for _, g := range rows {
		out[g.ID] = &Group{
			ID:         g.ID,
			TenantID:   g.TenantID,
			IdpID:      g.IdpID,
			ExternalID: g.ExternalID,
			Name:       g.Name,
			CreatedAt:  g.CreatedAt,
		}
	}
	return out, nil
}

// Search finds groups in a tenant by name, case-insensitively, ordered by name.
func (r *GroupStore) Search(ctx context.Context, tenantID, query string, limit int) ([]*Group, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.Group.Query().
		Where(group.TenantID(tenantID), group.NameContainsFold(query)).
		Order(group.ByName(), group.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search groups: %w", err)
	}
	out := make([]*Group, 0, len(rows))
	for _, g := range rows {
		out = append(out, &Group{ID: g.ID, TenantID: g.TenantID, IdpID: g.IdpID, ExternalID: g.ExternalID, Name: g.Name, CreatedAt: g.CreatedAt})
	}
	return out, nil
}
