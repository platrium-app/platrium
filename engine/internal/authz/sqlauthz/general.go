package sqlauthz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
)

// SetGeneralAccess sets an item's general access, replacing whatever it was.
// An item has at most one general-access grant, whose kind is the level's
// subject. Levels are defined in authz.Levels; nothing here names one.
func (a *Authorizer) SetGeneralAccess(ctx context.Context, actor authz.Principal, in authz.GeneralAccessInput) error {
	def, ok := authz.LevelOf(in.Level)
	if !ok {
		return fmt.Errorf("%w: unknown access level %q", authz.ErrInvalid, in.Level)
	}

	var caps authz.Capability
	if def.Stored() {
		c, err := authz.GrantCaps(in.Role, in.NoDownload)
		if err != nil {
			return fmt.Errorf("%w: role %q cannot be granted", authz.ErrInvalid, in.Role)
		}
		caps = authz.Normalize(c)
		if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
			return fmt.Errorf("%w: expiry must be in the future", authz.ErrInvalid)
		}
	}

	actorCaps, err := a.require(ctx, actor, in.ItemID, authz.CapShare)
	if err != nil {
		return err
	}

	// Every stored level keeps its grant under its own subject kind; setting one
	// level removes the others'.
	var kinds []string
	for _, l := range authz.Levels() {
		if l.Stored() {
			kinds = append(kinds, string(l.Subject))
		}
	}

	return a.changeTx(ctx, func(tx *ent.Tx) error {
		item, err := tx.DriveItem.Query().
			Where(driveitem.ID(in.ItemID), driveitem.TenantID(actor.TenantID)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: item", authz.ErrNotFound)
			}
			return err
		}
		d, err := a.lockDrive(ctx, tx, item.DriveID)
		if err != nil {
			return err
		}

		err = a.checkChange(ctx, tx, pending{
			op: authz.OpGeneralAccess, actor: actor, actorCaps: actorCaps, item: item, drive: d,
			after: func(before []authz.Grant) []authz.Grant {
				out := make([]authz.Grant, 0, len(before)+1)
				for _, g := range before {
					if !slices.Contains(kinds, string(g.Subject.Type)) {
						out = append(out, g)
					}
				}
				if def.Stored() {
					out = append(out, authz.Grant{Subject: def.SubjectFor(actor.TenantID), Role: in.Role, Caps: caps, ExpiresAt: in.ExpiresAt})
				}
				return out
			},
		})
		if err != nil {
			return err
		}

		if _, err := tx.Grant.Delete().Where(grant.ResourceID(item.ID), grant.SubjectTypeIn(kinds...)).Exec(ctx); err != nil {
			return err
		}
		if !def.Stored() {
			return nil
		}
		_, err = upsertGrant(ctx, tx, item, def.SubjectFor(actor.TenantID), in.Role, caps, in.ExpiresAt, actor.UserID)
		return err
	})
}

// GeneralAccessOptions lists the levels the actor can choose for an item.
func (a *Authorizer) GeneralAccessOptions(ctx context.Context, actor authz.Principal, itemID string) ([]authz.LevelDef, error) {
	if _, err := a.require(ctx, actor, itemID, authz.CapShare); err != nil {
		return nil, err
	}
	publicOK, err := a.publicAllowed(ctx, map[string]bool{}, actor.TenantID)
	if err != nil {
		return nil, err
	}
	var out []authz.LevelDef
	for _, d := range authz.Levels() {
		if d.RequiresPublicSharing && !publicOK {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}
