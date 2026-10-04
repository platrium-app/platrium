package sqlauthz

import (
	"context"
	"fmt"
	"slices"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/groupclosure"
	"platrium/internal/infra/db/ent/user"
)

// Principal resolves a signed-in user and every group they belong to, nested
// groups included. The groups come from the flattened closure table, so this
// is two indexed lookups regardless of how deeply groups nest.
func (a *Authorizer) Principal(ctx context.Context, tenantID, userID string) (authz.Principal, error) {
	ok, err := a.db.User.Query().Where(user.ID(userID), user.TenantID(tenantID)).Exist(ctx)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("failed to look up user: %w", err)
	}
	if !ok {
		return authz.Principal{}, fmt.Errorf("%w: user", authz.ErrNotFound)
	}

	groups, err := a.db.GroupClosure.Query().
		Where(groupclosure.UserID(userID), groupclosure.TenantID(tenantID)).
		Select(groupclosure.FieldGroupID).
		Strings(ctx)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("failed to load groups: %w", err)
	}
	slices.Sort(groups)

	return authz.Principal{TenantID: tenantID, UserID: userID, GroupIDs: groups}, nil
}
