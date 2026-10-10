package identity

import (
	"context"
	"fmt"
	"strings"

	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/predicate"
	"platrium/internal/infra/db/ent/user"
)

// UserStatus filters users by whether they can sign in.
type UserStatus string

const (
	UserStatusAny      UserStatus = ""
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusDisabled UserStatus = "DISABLED"
)

// UserFilter narrows a listing of a tenant's users. Zero fields match everything.
type UserFilter struct {
	Search string // matches name or email, case-insensitively
	IdpID  string // only users from this identity provider
	Status UserStatus
}

// UserCursor is the position after which a listing resumes. Listings are
// ordered by (display name, ID), so the pair identifies a position exactly.
type UserCursor struct {
	DisplayName string
	ID          string
}

// UserListItem is a user together with the identity provider they come from.
type UserListItem struct {
	*User
	IdpName string
	IdpType string // LOCAL, OIDC or SAML (see auth.IdpType)
}

func itemFromEnt(u *ent.User) *UserListItem {
	it := &UserListItem{User: userFromEnt(u)}
	if idp := u.Edges.Idp; idp != nil {
		it.IdpName, it.IdpType = idp.Name, string(idp.Type)
	}
	return it
}

func userPredicates(tenantID string, f UserFilter) []predicate.User {
	preds := []predicate.User{user.TenantID(tenantID)}
	if q := strings.TrimSpace(f.Search); q != "" {
		preds = append(preds, user.Or(user.DisplayNameContainsFold(q), user.EmailContainsFold(q)))
	}
	if f.IdpID != "" {
		preds = append(preds, user.IdpID(f.IdpID))
	}
	switch f.Status {
	case UserStatusActive:
		preds = append(preds, user.DisabledAtIsNil())
	case UserStatusDisabled:
		preds = append(preds, user.DisabledAtNotNil())
	}
	return preds
}

// List returns up to limit of a tenant's users in a stable order, starting
// after the cursor when given.
func (r *UserStore) List(ctx context.Context, tenantID string, f UserFilter, after *UserCursor, limit int) ([]*UserListItem, error) {
	if limit <= 0 {
		return nil, nil
	}
	preds := userPredicates(tenantID, f)
	if after != nil {
		preds = append(preds, user.Or(
			user.DisplayNameGT(after.DisplayName),
			user.And(user.DisplayNameEQ(after.DisplayName), user.IDGT(after.ID)),
		))
	}
	rows, err := r.db.User.Query().
		Where(preds...).
		WithIdp().
		Order(user.ByDisplayName(), user.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	out := make([]*UserListItem, 0, len(rows))
	for _, u := range rows {
		out = append(out, itemFromEnt(u))
	}
	return out, nil
}

// Count returns how many of a tenant's users match the filter.
func (r *UserStore) Count(ctx context.Context, tenantID string, f UserFilter) (int, error) {
	n, err := r.db.User.Query().Where(userPredicates(tenantID, f)...).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return n, nil
}

// GetItem fetches one user of the tenant with their identity provider.
func (r *UserStore) GetItem(ctx context.Context, tenantID, userID string) (*UserListItem, error) {
	return getItem(ctx, r.db.Client, tenantID, userID)
}

// GetItemTx is GetItem within a transaction.
func (r *UserStore) GetItemTx(ctx context.Context, tx *ent.Tx, tenantID, userID string) (*UserListItem, error) {
	return getItem(ctx, tx.Client(), tenantID, userID)
}

func getItem(ctx context.Context, c *ent.Client, tenantID, userID string) (*UserListItem, error) {
	u, err := c.User.Query().Where(user.ID(userID), user.TenantID(tenantID)).WithIdp().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	return itemFromEnt(u), nil
}

// UpdateProfileTx changes a user's display name and/or role. Nil leaves a field
// as it is. Whether the caller may is decided by the caller.
func (r *UserStore) UpdateProfileTx(ctx context.Context, tx *ent.Tx, tenantID, userID string, displayName, role *string) error {
	q := tx.User.Update().Where(user.ID(userID), user.TenantID(tenantID))
	if displayName != nil {
		q = q.SetDisplayName(*displayName)
	}
	if role != nil {
		q = q.SetRole(*role)
	}
	n, err := q.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: user", ErrNotFound)
	}
	return nil
}

// CountActiveWithRolesTx counts the tenant's enabled users who hold one of the
// roles, not counting excludeID. It backs "never remove the last administrator".
func (r *UserStore) CountActiveWithRolesTx(ctx context.Context, tx *ent.Tx, tenantID string, roles []string, excludeID string) (int, error) {
	n, err := tx.User.Query().
		Where(user.TenantID(tenantID), user.RoleIn(roles...), user.DisabledAtIsNil(), user.IDNEQ(excludeID)).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return n, nil
}
