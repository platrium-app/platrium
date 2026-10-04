package sqlauthz

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
	"platrium/internal/infra/db/ent/group"
	"platrium/internal/infra/db/ent/user"
)

// require loads the actor's capabilities on an item and checks they include
// need. An item the actor cannot see at all is ErrNotFound, so existence never
// leaks; an item they can see but may not touch is ErrForbidden.
func (a *Authorizer) require(ctx context.Context, actor authz.Principal, itemID string, need authz.Capability) (authz.Capability, error) {
	if actor.IsAnonymous() {
		return 0, fmt.Errorf("%w: sign in required", authz.ErrForbidden)
	}
	caps, err := a.Caps(ctx, actor, itemID)
	if err != nil {
		return 0, err
	}
	if caps == 0 {
		return 0, fmt.Errorf("%w: item", authz.ErrNotFound)
	}
	if !caps.Has(need) {
		return 0, fmt.Errorf("%w: requires %s", authz.ErrForbidden, need)
	}
	return caps, nil
}

func grantFromEnt(g *ent.Grant) *authz.Grant {
	out := &authz.Grant{
		ID:        g.ID,
		TenantID:  g.TenantID,
		DriveID:   g.DriveID,
		ItemID:    g.ResourceID,
		Subject:   authz.Subject{Type: authz.SubjectType(g.SubjectType), ID: g.SubjectID},
		Role:      authz.Role(g.Role),
		Caps:      authz.Capability(g.Caps),
		ExpiresAt: g.ExpiresAt,
		CreatedAt: g.CreatedAt,
	}
	if g.CreatedBy != nil {
		out.CreatedBy = *g.CreatedBy
	}
	return out
}

// Grant shares an item with a subject. Sharing again with the same subject
// replaces the earlier grant.
func (a *Authorizer) Grant(ctx context.Context, actor authz.Principal, in authz.GrantInput) (*authz.Grant, error) {
	caps, err := authz.GrantCaps(in.Role, in.NoDownload)
	if err != nil {
		return nil, fmt.Errorf("%w: role %q cannot be granted", authz.ErrInvalid, in.Role)
	}
	caps = authz.Normalize(caps)
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("%w: expiry must be in the future", authz.ErrInvalid)
	}

	actorCaps, err := a.require(ctx, actor, in.ItemID, authz.CapShare)
	if err != nil {
		return nil, err
	}
	// Nobody can hand out more than they hold.
	if !caps.SubsetOf(actorCaps) {
		return nil, fmt.Errorf("%w: cannot grant capabilities you do not hold", authz.ErrForbidden)
	}

	var result *ent.Grant
	err = a.db.WithTx(ctx, func(tx *ent.Tx) error {
		item, err := tx.DriveItem.Query().
			Where(driveitem.ID(in.ItemID), driveitem.TenantID(actor.TenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item", authz.ErrNotFound)
			}
			return err
		}
		if err := a.validateSubject(ctx, tx, actor.TenantID, in.Subject); err != nil {
			return err
		}
		result, err = upsertGrant(ctx, tx, item, in.Subject, in.Role, caps, in.ExpiresAt, actor.UserID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return grantFromEnt(result), nil
}

// validateSubject checks the subject exists in the actor's tenant. Grants never
// cross tenants.
func (a *Authorizer) validateSubject(ctx context.Context, tx *ent.Tx, tenantID string, s authz.Subject) error {
	switch s.Type {
	case authz.SubjectUser:
		ok, err := tx.User.Query().Where(user.ID(s.ID), user.TenantID(tenantID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: user", authz.ErrNotFound)
		}
	case authz.SubjectGroup:
		ok, err := tx.Group.Query().Where(group.ID(s.ID), group.TenantID(tenantID)).Exist(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: group", authz.ErrNotFound)
		}
	case authz.SubjectTenant:
		if s.ID != tenantID {
			return fmt.Errorf("%w: a tenant grant must name the actor's tenant", authz.ErrInvalid)
		}
	case authz.SubjectPublic:
		if s.ID != authz.PublicSubjectID {
			return fmt.Errorf("%w: a public grant uses subject %q", authz.ErrInvalid, authz.PublicSubjectID)
		}
		return a.requirePublicAllowed(ctx, tx, tenantID)
	default:
		return fmt.Errorf("%w: unknown subject type %q", authz.ErrInvalid, s.Type)
	}
	return nil
}

// upsertGrant writes the single grant for (item, subject), replacing any
// existing one.
func upsertGrant(ctx context.Context, tx *ent.Tx, item *ent.DriveItem, s authz.Subject, role authz.Role, caps authz.Capability, expires *time.Time, createdBy string) (*ent.Grant, error) {
	existing, err := tx.Grant.Query().
		Where(grant.ResourceID(item.ID), grant.SubjectType(string(s.Type)), grant.SubjectID(s.ID)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	if existing != nil {
		upd := tx.Grant.UpdateOne(existing).SetRole(string(role)).SetCaps(int64(caps))
		if expires != nil {
			upd.SetExpiresAt(expires.UTC())
		} else {
			upd.ClearExpiresAt()
		}
		return upd.Save(ctx)
	}

	create := tx.Grant.Create().
		SetTenantID(item.TenantID).
		SetDriveID(item.DriveID).
		SetResourceID(item.ID).
		SetSubjectType(string(s.Type)).
		SetSubjectID(s.ID).
		SetRole(string(role)).
		SetCaps(int64(caps))
	if expires != nil {
		create.SetExpiresAt(expires.UTC())
	}
	if createdBy != "" {
		create.SetCreatedBy(createdBy)
	}
	g, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("%w: grant already exists: %v", authz.ErrConflict, err)
		}
		return nil, err
	}
	return g, nil
}

// Revoke removes a grant. The actor needs CapShare on the grant's item.
func (a *Authorizer) Revoke(ctx context.Context, actor authz.Principal, grantID string) error {
	if actor.IsAnonymous() {
		return fmt.Errorf("%w: sign in required", authz.ErrForbidden)
	}
	g, err := a.db.Grant.Query().Where(grant.ID(grantID), grant.TenantID(actor.TenantID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: grant", authz.ErrNotFound)
		}
		return err
	}
	if _, err := a.require(ctx, actor, g.ResourceID, authz.CapShare); err != nil {
		return err
	}
	return a.db.Grant.DeleteOneID(g.ID).Exec(ctx)
}

// ListGrants returns the grants on an item, oldest first. Requires CapShare.
func (a *Authorizer) ListGrants(ctx context.Context, actor authz.Principal, itemID string) ([]authz.Grant, error) {
	if _, err := a.require(ctx, actor, itemID, authz.CapShare); err != nil {
		return nil, err
	}
	rows, err := a.db.Grant.Query().
		Where(grant.ResourceID(itemID), grant.TenantID(actor.TenantID)).
		Order(grant.ByCreatedAt(), grant.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list grants: %w", err)
	}
	out := make([]authz.Grant, 0, len(rows))
	for _, g := range rows {
		out = append(out, *grantFromEnt(g))
	}
	return out, nil
}

// ItemAccess returns an item's grants and whether it inherits. Requires CapShare.
func (a *Authorizer) ItemAccess(ctx context.Context, actor authz.Principal, itemID string) (*authz.ItemAccess, error) {
	grants, err := a.ListGrants(ctx, actor, itemID)
	if err != nil {
		return nil, err
	}
	item, err := a.db.DriveItem.Query().
		Where(driveitem.ID(itemID), driveitem.TenantID(actor.TenantID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: item", authz.ErrNotFound)
		}
		return nil, err
	}
	return &authz.ItemAccess{ItemID: itemID, InheritsPermissions: item.InheritPerms, Grants: grants}, nil
}

// SetInheritance stops or resumes an item inheriting its ancestors' grants.
func (a *Authorizer) SetInheritance(ctx context.Context, actor authz.Principal, itemID string, inherit bool) error {
	if _, err := a.require(ctx, actor, itemID, authz.CapManage); err != nil {
		return err
	}

	return a.db.WithTx(ctx, func(tx *ent.Tx) error {
		item, err := tx.DriveItem.Query().
			Where(driveitem.ID(itemID), driveitem.TenantID(actor.TenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item", authz.ErrNotFound)
			}
			return err
		}
		if item.ParentID == nil {
			return fmt.Errorf("%w: a drive root has nothing to inherit", authz.ErrInvalid)
		}
		if item.InheritPerms == inherit {
			return nil
		}
		if err := tx.DriveItem.UpdateOne(item).SetInheritPerms(inherit).Exec(ctx); err != nil {
			return err
		}
		if inherit {
			return nil
		}

		// Keep the actor's access: unless they own the drive, give them a direct
		// manager grant so restricting an item cannot lock them out of it.
		d, err := tx.Drive.Query().Where(drive.ID(item.DriveID)).Only(ctx)
		if err != nil {
			return err
		}
		if d.OwnerID != nil && *d.OwnerID == actor.UserID {
			return nil
		}
		managerCaps, _ := authz.RoleManager.Caps()
		_, err = upsertGrant(ctx, tx, item, authz.Subject{Type: authz.SubjectUser, ID: actor.UserID}, authz.RoleManager, managerCaps, nil, actor.UserID)
		return err
	})
}
