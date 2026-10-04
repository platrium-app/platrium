package sqlauthz

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
	"platrium/internal/infra/db/ent/tenant"
)

// generalRoles are the roles general access may carry.
var generalRoles = map[authz.GeneralAccessLevel][]authz.Role{
	authz.AccessTenant: {authz.RoleViewer, authz.RoleFullEditor}, // Viewer or Editor
	authz.AccessPublic: {authz.RoleViewer},
}

func roleAllowed(level authz.GeneralAccessLevel, role authz.Role) bool {
	for _, r := range generalRoles[level] {
		if r == role {
			return true
		}
	}
	return false
}

// SetGeneralAccess sets an item's general access, replacing whatever it was.
// An item has at most one general-access grant: a TENANT grant or a PUBLIC one.
func (a *Authorizer) SetGeneralAccess(ctx context.Context, actor authz.Principal, in authz.GeneralAccessInput) error {
	var caps authz.Capability
	switch in.Level {
	case authz.AccessRestricted:
	case authz.AccessTenant, authz.AccessPublic:
		if !roleAllowed(in.Level, in.Role) {
			return fmt.Errorf("%w: role %q is not available for %s access", authz.ErrInvalid, in.Role, in.Level)
		}
		c, err := authz.GrantCaps(in.Role, in.NoDownload)
		if err != nil {
			return fmt.Errorf("%w: role %q cannot be granted", authz.ErrInvalid, in.Role)
		}
		caps = authz.Normalize(c)
		if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("%w: expiry must be in the future", authz.ErrInvalid)
		}
	default:
		return fmt.Errorf("%w: unknown access level %q", authz.ErrInvalid, in.Level)
	}

	actorCaps, err := a.require(ctx, actor, in.ItemID, authz.CapShare)
	if err != nil {
		return err
	}
	if !caps.SubsetOf(actorCaps) {
		return fmt.Errorf("%w: cannot grant capabilities you do not hold", authz.ErrForbidden)
	}

	return a.db.WithTx(ctx, func(tx *ent.Tx) error {
		item, err := tx.DriveItem.Query().
			Where(driveitem.ID(in.ItemID), driveitem.TenantID(actor.TenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item", authz.ErrNotFound)
			}
			return err
		}

		// Remove the general-access grants that no longer apply.
		var drop []string
		switch in.Level {
		case authz.AccessRestricted:
			drop = []string{string(authz.SubjectTenant), string(authz.SubjectPublic)}
		case authz.AccessTenant:
			drop = []string{string(authz.SubjectPublic)}
		case authz.AccessPublic:
			drop = []string{string(authz.SubjectTenant)}
			if err := a.requirePublicAllowed(ctx, tx, actor.TenantID); err != nil {
				return err
			}
		}
		if _, err := tx.Grant.Delete().Where(grant.ResourceID(item.ID), grant.SubjectTypeIn(drop...)).Exec(ctx); err != nil {
			return err
		}

		switch in.Level {
		case authz.AccessTenant:
			_, err = upsertGrant(ctx, tx, item, authz.Subject{Type: authz.SubjectTenant, ID: actor.TenantID}, in.Role, caps, in.ExpiresAt, actor.UserID)
		case authz.AccessPublic:
			_, err = upsertGrant(ctx, tx, item, authz.Subject{Type: authz.SubjectPublic, ID: authz.PublicSubjectID}, in.Role, caps, in.ExpiresAt, actor.UserID)
		}
		return err
	})
}

// requirePublicAllowed fails unless the tenant permits public sharing.
func (a *Authorizer) requirePublicAllowed(ctx context.Context, tx *ent.Tx, tenantID string) error {
	t, err := tx.Tenant.Query().Where(tenant.ID(tenantID)).Only(ctx)
	if err != nil {
		return err
	}
	if !t.AllowPublicSharing {
		return fmt.Errorf("%w: your organization does not allow public sharing", authz.ErrForbidden)
	}
	return nil
}
