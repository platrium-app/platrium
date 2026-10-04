package sqlauthz

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/group"
	"platrium/internal/infra/db/ent/groupclosure"
	"platrium/internal/infra/db/ent/groupmember"
	"platrium/internal/infra/db/ent/tenant"
	"platrium/internal/infra/db/ent/user"
)

const closureBatch = 500

// AddMember adds a user or a group to a group. It is idempotent. Adding a group
// that would create a cycle (a group inside itself or one of its own members)
// is rejected.
func (a *Authorizer) AddMember(ctx context.Context, tenantID, groupID string, mt authz.MemberType, memberID string) error {
	return a.mutateMembership(ctx, tenantID, func(tx *ent.Tx) ([]string, error) {
		if err := validateGroupAndMember(ctx, tx, tenantID, groupID, mt, memberID); err != nil {
			return nil, err
		}

		ancestors, err := ancestorGroups(ctx, tx, tenantID, groupID)
		if err != nil {
			return nil, err
		}
		if mt == authz.MemberGroup && (memberID == groupID || slices.Contains(ancestors, memberID)) {
			return nil, fmt.Errorf("%w: adding this group would create a cycle", authz.ErrInvalid)
		}

		exists, err := tx.GroupMember.Query().
			Where(groupmember.GroupID(groupID), groupmember.MemberType(string(mt)), groupmember.MemberID(memberID)).
			Exist(ctx)
		if err != nil {
			return nil, err
		}
		if !exists {
			if err := tx.GroupMember.Create().
				SetTenantID(tenantID).
				SetGroupID(groupID).
				SetMemberType(string(mt)).
				SetMemberID(memberID).
				Exec(ctx); err != nil {
				return nil, fmt.Errorf("failed to add member: %w", err)
			}
		}
		return append(ancestors, groupID), nil
	})
}

// RemoveMember removes a direct membership. Removing one that does not exist
// is a no-op. Users who still belong through another path keep their access.
func (a *Authorizer) RemoveMember(ctx context.Context, tenantID, groupID string, mt authz.MemberType, memberID string) error {
	return a.mutateMembership(ctx, tenantID, func(tx *ent.Tx) ([]string, error) {
		if ok, err := tx.Group.Query().Where(group.ID(groupID), group.TenantID(tenantID)).Exist(ctx); err != nil {
			return nil, err
		} else if !ok {
			return nil, fmt.Errorf("%w: group", authz.ErrNotFound)
		}

		if _, err := tx.GroupMember.Delete().
			Where(groupmember.GroupID(groupID), groupmember.MemberType(string(mt)), groupmember.MemberID(memberID)).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("failed to remove member: %w", err)
		}

		ancestors, err := ancestorGroups(ctx, tx, tenantID, groupID)
		if err != nil {
			return nil, err
		}
		return append(ancestors, groupID), nil
	})
}

// RebuildClosure recomputes the flattened membership of every group in a
// tenant from the direct memberships. It is a repair tool, safe to run at any
// time.
func (a *Authorizer) RebuildClosure(ctx context.Context, tenantID string) error {
	return a.mutateMembership(ctx, tenantID, func(tx *ent.Tx) ([]string, error) {
		return tx.Group.Query().Where(group.TenantID(tenantID)).IDs(ctx)
	})
}

// mutateMembership runs a membership change and then re-derives the closure for
// the groups it reports as affected, in one transaction. The tenant row is
// locked first, which serializes group edits within a tenant. They are rare
// next to checks, and it keeps the derived table consistent.
func (a *Authorizer) mutateMembership(ctx context.Context, tenantID string, change func(tx *ent.Tx) ([]string, error)) error {
	// READ COMMITTED so reads after the lock see other edits' committed work
	// on MySQL and MariaDB (see fsops.MoveItem).
	return a.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *ent.Tx) error {
		lock := tx.Tenant.Query().Where(tenant.ID(tenantID))
		if a.db.RowLocks() {
			lock = lock.ForUpdate()
		}
		if _, err := lock.Only(ctx); err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: tenant", authz.ErrNotFound)
			}
			return err
		}

		affected, err := change(tx)
		if err != nil {
			return err
		}
		return rebuildClosure(ctx, tx, tenantID, affected)
	})
}

func validateGroupAndMember(ctx context.Context, tx *ent.Tx, tenantID, groupID string, mt authz.MemberType, memberID string) error {
	if ok, err := tx.Group.Query().Where(group.ID(groupID), group.TenantID(tenantID)).Exist(ctx); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: group", authz.ErrNotFound)
	}

	switch mt {
	case authz.MemberUser:
		ok, err := tx.User.Query().Where(user.ID(memberID), user.TenantID(tenantID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: user", authz.ErrNotFound)
		}
	case authz.MemberGroup:
		ok, err := tx.Group.Query().Where(group.ID(memberID), group.TenantID(tenantID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: member group", authz.ErrNotFound)
		}
	default:
		return fmt.Errorf("%w: unknown member type %q", authz.ErrInvalid, mt)
	}
	return nil
}

// ancestorGroups returns every group that contains g, directly or through
// other groups.
func ancestorGroups(ctx context.Context, tx *ent.Tx, tenantID, g string) ([]string, error) {
	visited := map[string]struct{}{g: {}}
	var result []string
	frontier := []string{g}
	for len(frontier) > 0 {
		parents, err := tx.GroupMember.Query().
			Where(groupmember.TenantID(tenantID), groupmember.MemberType(string(authz.MemberGroup)), groupmember.MemberIDIn(frontier...)).
			Select(groupmember.FieldGroupID).
			Strings(ctx)
		if err != nil {
			return nil, err
		}
		frontier = frontier[:0]
		for _, id := range parents {
			if _, seen := visited[id]; !seen {
				visited[id] = struct{}{}
				result = append(result, id)
				frontier = append(frontier, id)
			}
		}
	}
	return result, nil
}

// descendantUsers returns every user in g, directly or through nested groups.
func descendantUsers(ctx context.Context, tx *ent.Tx, tenantID, g string) (map[string]struct{}, error) {
	users := map[string]struct{}{}
	visited := map[string]struct{}{g: {}}
	frontier := []string{g}
	for len(frontier) > 0 {
		members, err := tx.GroupMember.Query().
			Where(groupmember.TenantID(tenantID), groupmember.GroupIDIn(frontier...)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		frontier = frontier[:0]
		for _, m := range members {
			switch authz.MemberType(m.MemberType) {
			case authz.MemberUser:
				users[m.MemberID] = struct{}{}
			case authz.MemberGroup:
				if _, seen := visited[m.MemberID]; !seen {
					visited[m.MemberID] = struct{}{}
					frontier = append(frontier, m.MemberID)
				}
			}
		}
	}
	return users, nil
}

// rebuildClosure re-derives the flattened membership of the given groups from
// the direct memberships and applies the difference.
func rebuildClosure(ctx context.Context, tx *ent.Tx, tenantID string, groupIDs []string) error {
	seen := map[string]struct{}{}
	for _, g := range groupIDs {
		if _, dup := seen[g]; dup {
			continue
		}
		seen[g] = struct{}{}

		want, err := descendantUsers(ctx, tx, tenantID, g)
		if err != nil {
			return err
		}
		have, err := tx.GroupClosure.Query().Where(groupclosure.GroupID(g)).Select(groupclosure.FieldUserID).Strings(ctx)
		if err != nil {
			return err
		}

		haveSet := make(map[string]struct{}, len(have))
		var stale []string
		for _, u := range have {
			haveSet[u] = struct{}{}
			if _, ok := want[u]; !ok {
				stale = append(stale, u)
			}
		}
		for chunk := range slices.Chunk(stale, closureBatch) {
			if _, err := tx.GroupClosure.Delete().Where(groupclosure.GroupID(g), groupclosure.UserIDIn(chunk...)).Exec(ctx); err != nil {
				return err
			}
		}

		var missing []*ent.GroupClosureCreate
		for u := range want {
			if _, ok := haveSet[u]; !ok {
				missing = append(missing, tx.GroupClosure.Create().SetTenantID(tenantID).SetGroupID(g).SetUserID(u))
			}
		}
		for chunk := range slices.Chunk(missing, closureBatch) {
			if err := tx.GroupClosure.CreateBulk(chunk...).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
