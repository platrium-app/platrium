package identity

import (
	"context"
	"fmt"
	"slices"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/group"
	"platrium/internal/infra/db/ent/policygroup"
)

// Policies a tenant admin can scope to groups. The name is a string, so adding
// a policy needs no schema change.
const (
	// PolicySharedDriveCreators lists the groups whose members may create
	// shared drives (tenant admins always may).
	PolicySharedDriveCreators = "shared_drive_creators"
)

// PolicyStore manages which groups a tenant policy applies to.
type PolicyStore struct {
	db *db.DB
}

func NewPolicyStore(d *db.DB) *PolicyStore { return &PolicyStore{db: d} }

// Groups returns the group IDs a policy applies to, in a stable order.
func (s *PolicyStore) Groups(ctx context.Context, tenantID, policy string) ([]string, error) {
	ids, err := s.db.PolicyGroup.Query().
		Where(policygroup.TenantID(tenantID), policygroup.PolicyEQ(policy)).
		Select(policygroup.FieldGroupID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy groups: %w", err)
	}
	slices.Sort(ids)
	return ids, nil
}

// SetGroups replaces the set of groups a policy applies to. Every group must
// belong to the tenant.
func (s *PolicyStore) SetGroups(ctx context.Context, tenantID, policy string, groupIDs []string) error {
	want := slices.Clone(groupIDs)
	slices.Sort(want)
	want = slices.Compact(want)

	return s.db.WithTx(ctx, func(tx *ent.Tx) error {
		if len(want) > 0 {
			n, err := tx.Group.Query().Where(group.IDIn(want...), group.TenantID(tenantID)).Count(ctx)
			if err != nil {
				return err
			}
			if n != len(want) {
				return fmt.Errorf("%w: group", ErrNotFound)
			}
		}

		if _, err := tx.PolicyGroup.Delete().
			Where(policygroup.TenantID(tenantID), policygroup.PolicyEQ(policy), policygroup.GroupIDNotIn(want...)).
			Exec(ctx); err != nil {
			return err
		}
		have, err := tx.PolicyGroup.Query().
			Where(policygroup.TenantID(tenantID), policygroup.PolicyEQ(policy)).
			Select(policygroup.FieldGroupID).
			Strings(ctx)
		if err != nil {
			return err
		}
		for _, id := range want {
			if slices.Contains(have, id) {
				continue
			}
			if err := tx.PolicyGroup.Create().SetTenantID(tenantID).SetPolicy(policy).SetGroupID(id).Exec(ctx); err != nil {
				return fmt.Errorf("failed to add policy group: %w", err)
			}
		}
		return nil
	})
}

// AppliesTo reports whether a policy applies to any of the given groups. Pass a
// user's full group list (nested groups included) to ask "does it apply to them".
func (s *PolicyStore) AppliesTo(ctx context.Context, tenantID, policy string, groupIDs []string) (bool, error) {
	if len(groupIDs) == 0 {
		return false, nil
	}
	return s.db.PolicyGroup.Query().
		Where(policygroup.TenantID(tenantID), policygroup.PolicyEQ(policy), policygroup.GroupIDIn(groupIDs...)).
		Exist(ctx)
}
