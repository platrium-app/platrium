package sqlauthz

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/grant"
	"platrium/internal/infra/db/ent/predicate"
)

const (
	defaultSharedPage = 50
	maxSharedPage     = 200
)

// SharedWithMe lists items shared directly with the principal or one of their
// groups. These are entry points: opening one shows its contents, so the
// result is never expanded into every file beneath a shared folder. That keeps
// the query a plain indexed lookup however large the shared trees are.
//
// Items in the principal's own drives are excluded, tenant-wide and public
// grants are not listed (they are discoverable through browsing), and an item
// shared several ways appears once with the union of the capabilities.
// An item already reachable through a shared ancestor still appears on its own.
func (a *Authorizer) SharedWithMe(ctx context.Context, p authz.Principal, limit int, after string) ([]authz.SharedItem, error) {
	if p.IsAnonymous() {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultSharedPage
	}
	limit = min(limit, maxSharedPage)

	base := []predicate.Grant{
		grant.TenantID(p.TenantID),
		directlyShared(p),
		notExpired(time.Now().UTC()),
		grant.HasDriveWith(drive.OwnerIDNEQ(p.UserID)),
	}

	q := a.db.Grant.Query().Where(base...)
	if after != "" {
		q = q.Where(grant.ResourceIDGT(after))
	}
	ids, err := q.Unique(true).
		Order(grant.ByResourceID()).
		Limit(limit).
		Select(grant.FieldResourceID).
		Strings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list shared items: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	grants, err := a.db.Grant.Query().
		Where(append(base, grant.ResourceIDIn(ids...))...).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load shared grants: %w", err)
	}
	caps := make(map[string]authz.Capability, len(ids))
	for _, g := range grants {
		caps[g.ResourceID] |= authz.Capability(g.Caps)
	}

	out := make([]authz.SharedItem, 0, len(ids))
	for _, id := range ids {
		role, _ := authz.Describe(caps[id])
		out = append(out, authz.SharedItem{ItemID: id, Caps: caps[id], Role: role})
	}
	return out, nil
}

// directlyShared matches grants to the user or to one of their groups.
func directlyShared(p authz.Principal) predicate.Grant {
	preds := []predicate.Grant{
		grant.And(grant.SubjectType(string(authz.SubjectUser)), grant.SubjectID(p.UserID)),
	}
	if len(p.GroupIDs) > 0 {
		preds = append(preds, grant.And(grant.SubjectType(string(authz.SubjectGroup)), grant.SubjectIDIn(p.GroupIDs...)))
	}
	return grant.Or(preds...)
}
