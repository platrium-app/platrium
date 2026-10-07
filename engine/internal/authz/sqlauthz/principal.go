package sqlauthz

import (
	"context"
	"fmt"
	"slices"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/groupclosure"
)

// Principal expands a user into the principal checks run as: their identity and
// every group they belong to, nested groups included. The groups come from the
// flattened closure table, so this is one indexed lookup regardless of how
// deeply groups nest.
//
// Whether the user exists, is enabled or still holds a valid session is not the
// engine's business; package actor settles that before asking.
func (a *Authorizer) Principal(ctx context.Context, tenantID, userID string) (authz.Principal, error) {
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
