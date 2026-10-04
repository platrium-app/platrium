package identity

import (
	"context"
	"fmt"
	"strings"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/user"
)

// Tenant roles. A user's role says what they may administer in their tenant,
// separate from what they can do with any one file (see package authz).
const (
	// RoleSuperAdmin is granted to a tenant's first administrator.
	RoleSuperAdmin = "SUPER_ADMIN"
	// RoleAdmin administers the tenant: settings, policies, shared drives.
	RoleAdmin = "ADMIN"
	// RoleMember is an ordinary user.
	RoleMember = "MEMBER"
)

// IsAdmin reports whether a tenant role administers the tenant.
func IsAdmin(role string) bool { return role == RoleSuperAdmin || role == RoleAdmin }

// User represents an identity belonging to exactly one tenant.
type User struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	IdpID       string    `json:"idp_id"`
	ExternalID  string    `json:"external_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func userFromEnt(u *ent.User) *User {
	return &User{
		ID:          u.ID,
		TenantID:    u.TenantID,
		IdpID:       u.IdpID,
		ExternalID:  u.ExternalID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
	}
}

// UserStore manages User records.
type UserStore struct {
	db *db.DB
}

func NewUserStore(d *db.DB) *UserStore {
	return &UserStore{db: d}
}

// CreateUserParams are the inputs for creating a user.
type CreateUserParams struct {
	ID          string // optional; generated when empty
	TenantID    string
	IdpID       string
	ExternalID  string
	Email       string
	DisplayName string
	Role        string // optional; defaults to the schema default
}

// CreateUserTx creates a user within the provided transaction. The IdP must
// belong to the same tenant as the user.
func (r *UserStore) CreateUserTx(ctx context.Context, tx *ent.Tx, p CreateUserParams) (*User, error) {
	if p.ID == "" {
		p.ID = nanoid.Must()
	}

	// Tenant isolation: an IdP from another tenant must never back this user.
	// This also proves the tenant exists, so no separate tenant lookup is needed.
	if ok, err := tx.IdpProvider.Query().Where(idpprovider.ID(p.IdpID), idpprovider.TenantID(p.TenantID)).Exist(ctx); err != nil {
		return nil, fmt.Errorf("failed to look up idp: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("%w: idp not found in tenant", ErrNotFound)
	}

	create := tx.User.Create().
		SetID(p.ID).
		SetTenantID(p.TenantID).
		SetIdpID(p.IdpID).
		SetExternalID(p.ExternalID).
		SetEmail(p.Email).
		SetDisplayName(p.DisplayName)
	if p.Role != "" {
		create.SetRole(p.Role)
	}

	u, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("%w: a user with this externalId already exists for the idp: %v", ErrConflict, err)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return userFromEnt(u), nil
}

// GetUserByExternalId fetches a user and their tenant ID based on their IdP mapping.
func (r *UserStore) GetUserByExternalId(ctx context.Context, idpId, externalId string) (*User, string, error) {
	u, err := r.db.User.Query().
		Where(user.IdpID(idpId), user.ExternalID(externalId)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, "", fmt.Errorf("%w: user", ErrNotFound)
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}
	return userFromEnt(u), u.TenantID, nil
}

// GetByIDs returns the users with the given IDs that belong to the tenant, keyed
// by ID. Unknown IDs, and IDs from other tenants, are simply absent.
func (r *UserStore) GetByIDs(ctx context.Context, tenantID string, ids []string) (map[string]*User, error) {
	out := make(map[string]*User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.User.Query().Where(user.IDIn(ids...), user.TenantID(tenantID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}
	for _, u := range rows {
		out[u.ID] = userFromEnt(u)
	}
	return out, nil
}

// Search finds users in a tenant by name or email, case-insensitively, ordered
// by name. Every member of a tenant may search its directory.
func (r *UserStore) Search(ctx context.Context, tenantID, query string, limit int) ([]*User, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.User.Query().
		Where(user.TenantID(tenantID), user.Or(user.DisplayNameContainsFold(query), user.EmailContainsFold(query))).
		Order(user.ByDisplayName(), user.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	out := make([]*User, 0, len(rows))
	for _, u := range rows {
		out = append(out, userFromEnt(u))
	}
	return out, nil
}
