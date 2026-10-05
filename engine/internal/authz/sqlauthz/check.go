package sqlauthz

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
	"platrium/internal/infra/db/ent/predicate"
)

const (
	// itemChunk and grantChunk bound the number of bound parameters per
	// statement, well under every backend's limit.
	itemChunk  = 500
	grantChunk = 1000
	// maxAncestorDepth backstops the recursive walk. The tree is acyclic by
	// construction, so this only limits damage from corruption.
	maxAncestorDepth = 1024
)

// Check reports whether p holds every capability in need on the item.
func (a *Authorizer) Check(ctx context.Context, p authz.Principal, itemID string, need authz.Capability) (bool, error) {
	caps, err := a.Caps(ctx, p, itemID)
	if err != nil {
		return false, err
	}
	return caps != 0 && caps.Has(need), nil
}

// Caps returns the capabilities p holds on an item.
func (a *Authorizer) Caps(ctx context.Context, p authz.Principal, itemID string) (authz.Capability, error) {
	m, err := a.CapsMany(ctx, p, []string{itemID})
	if err != nil {
		return 0, err
	}
	return m[itemID], nil
}

// CapsMany returns the capabilities p holds on each item, in as few round
// trips as possible: one ancestor walk covers every item in a chunk, and one
// grant query covers every ancestor found.
//
// How an item's capabilities are decided:
//  1. The item must exist and, for a signed-in principal, be in their tenant.
//  2. The drive owner holds every capability.
//  3. Otherwise, the union of every unexpired grant that matches the principal
//     (their user, their groups, their tenant, or PUBLIC) on the item or on any
//     ancestor, up to the first item that does not inherit.
//  4. Past that point, a restriction hides ancestors' grants from everyone
//     except managers: a grant that carries MANAGE (a Drive Admin) still applies
//     in full. Restricting an item cuts out members, never the drive's admins.
//
// Items that do not exist or are not visible yield no capabilities and no
// error, so callers cannot probe for existence.
func (a *Authorizer) CapsMany(ctx context.Context, p authz.Principal, itemIDs []string) (map[string]authz.Capability, error) {
	out := make(map[string]authz.Capability, len(itemIDs))
	ids := make([]string, 0, len(itemIDs))
	for _, id := range itemIDs {
		if _, seen := out[id]; !seen {
			out[id] = 0
			ids = append(ids, id)
		}
	}

	for chunk := range slices.Chunk(ids, itemChunk) {
		if err := a.capsChunk(ctx, p, chunk, out); err != nil {
			return nil, err
		}
		if err := a.withholdPrivateRootSharing(ctx, chunk, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// withholdPrivateRootSharing removes the capabilities that open a private
// drive to others, from everyone, owner included. A private drive is the
// owner's alone; its contents can be shared, but the drive itself cannot.
//
// This is the one place the rule lives. Every sharing operation requires SHARE
// or MANAGE on its item, so enforcing it here covers granting, revoking,
// listing, general access and inheritance, and clients that offer sharing only
// where SHARE is held never show it for a private drive.
func (a *Authorizer) withholdPrivateRootSharing(ctx context.Context, ids []string, out map[string]authz.Capability) error {
	const sharing = authz.CapShare | authz.CapManage
	var candidates []string
	for _, id := range ids {
		if out[id]&sharing != 0 {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	roots, err := a.db.DriveItem.Query().
		Where(
			driveitem.IDIn(candidates...),
			driveitem.ParentIDIsNil(),
			driveitem.HasDriveWith(drive.TypeEQ(drive.TypePRIVATE)),
		).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to find private drive roots: %w", err)
	}
	for _, id := range roots {
		out[id] = out[id].Without(sharing)
	}
	return nil
}

func (a *Authorizer) capsChunk(ctx context.Context, p authz.Principal, ids []string, out map[string]authz.Capability) error {
	q := a.db.DriveItem.Query().Where(driveitem.IDIn(ids...))
	if p.TenantID != "" {
		q = q.Where(driveitem.TenantID(p.TenantID)) // tenant isolation
	}
	items, err := q.All(ctx)
	if err != nil {
		return fmt.Errorf("failed to load items: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	// Drive owners hold every capability on their drive.
	owners := map[string]string{}
	if !p.IsAnonymous() {
		driveIDs := make([]string, 0, len(items))
		for _, it := range items {
			driveIDs = append(driveIDs, it.DriveID)
		}
		drives, err := a.db.Drive.Query().Where(drive.IDIn(driveIDs...)).All(ctx)
		if err != nil {
			return fmt.Errorf("failed to load drives: %w", err)
		}
		for _, d := range drives {
			// Only user-owned drives have an implicit owner. A tenant-owned
			// drive grants access through grants alone.
			if d.OwnerID != nil {
				owners[d.ID] = *d.OwnerID
			}
		}
	}

	pending := make([]string, 0, len(items))
	for _, it := range items {
		if owner, ok := owners[it.DriveID]; ok && !p.IsAnonymous() && owner == p.UserID {
			out[it.ID] = authz.AllCaps
			continue
		}
		pending = append(pending, it.ID)
	}
	if len(pending) == 0 {
		return nil
	}

	chains, err := a.ancestorChains(ctx, pending)
	if err != nil {
		return err
	}

	// One grant lookup for every ancestor of every pending item.
	seen := map[string]struct{}{}
	var resourceIDs []string
	for _, chain := range chains {
		for _, h := range chain {
			if _, ok := seen[h.id]; !ok {
				seen[h.id] = struct{}{}
				resourceIDs = append(resourceIDs, h.id)
			}
		}
	}

	byResource := map[string]authz.Capability{}
	managerOnly := map[string]authz.Capability{} // the same, counting only grants that carry MANAGE
	publicOK := map[string]bool{}                // tenant -> allows public sharing, looked up only if a public grant turns up
	now := time.Now().UTC()
	for chunk := range slices.Chunk(resourceIDs, grantChunk) {
		grants, err := a.db.Grant.Query().
			Where(grant.ResourceIDIn(chunk...), subjectMatches(p), notExpired(now)).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to load grants: %w", err)
		}
		for _, g := range grants {
			if g.SubjectType == string(authz.SubjectPublic) {
				ok, err := a.publicAllowed(ctx, publicOK, g.TenantID)
				if err != nil {
					return err
				}
				if !ok {
					continue // the tenant turned public sharing off
				}
			}
			caps := authz.Capability(g.Caps)
			byResource[g.ResourceID] |= caps
			if caps.Has(authz.CapManage) {
				managerOnly[g.ResourceID] |= caps
			}
		}
	}

	for origin, chain := range chains {
		var caps authz.Capability
		for _, h := range chain {
			if h.cut {
				caps |= managerOnly[h.id]
			} else {
				caps |= byResource[h.id]
			}
		}
		out[origin] = caps
	}
	return nil
}

// hop is one item on an ancestor chain.
type hop struct {
	id string
	// cut is true when a restriction (an item that does not inherit) lies
	// between this ancestor and the item the chain starts from. Only managers'
	// grants count on a cut hop.
	cut bool
}

// ancestorChains returns, for each item, its own ID followed by its ancestors'
// IDs up to the drive root. Each ancestor is marked cut if a restriction lies
// between it and the item.
//
// This is the only raw SQL on the check path: a recursive CTE, identical on
// Postgres, MySQL 8+, MariaDB 10.2+ and SQLite. One statement walks every
// item's chain at once, tenant-scoped at every hop. cut is carried as 0 or 1
// so the same text works on every dialect.
func (a *Authorizer) ancestorChains(ctx context.Context, ids []string) (map[string][]hop, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	query := a.db.Rebind(`
WITH RECURSIVE anc(origin, id, parent_id, tenant_id, inherit, cut, depth) AS (
	SELECT id, id, parent_id, tenant_id, inherit_perms, 0, 0 FROM drive_items WHERE id IN (` + placeholders + `)
	UNION ALL
	SELECT a.origin, p.id, p.parent_id, p.tenant_id, p.inherit_perms,
		CASE WHEN a.cut = 1 OR a.inherit = ? THEN 1 ELSE 0 END, a.depth + 1
	FROM drive_items p
	JOIN anc a ON p.id = a.parent_id AND p.tenant_id = a.tenant_id
	WHERE a.depth < ?
)
SELECT origin, id, cut FROM anc ORDER BY origin, depth`)

	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, false, maxAncestorDepth)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to walk ancestors: %w", err)
	}
	defer rows.Close()

	chains := make(map[string][]hop, len(ids))
	for rows.Next() {
		var origin, id string
		var cut int
		if err := rows.Scan(&origin, &id, &cut); err != nil {
			return nil, fmt.Errorf("failed to scan ancestor: %w", err)
		}
		chains[origin] = append(chains[origin], hop{id: id, cut: cut == 1})
	}
	return chains, rows.Err()
}

// subjectMatches selects grants that apply to p: PUBLIC always; and, for a
// signed-in principal, their user, their groups and their tenant.
func subjectMatches(p authz.Principal) predicate.Grant {
	preds := []predicate.Grant{grant.SubjectType(string(authz.SubjectPublic))}
	if p.UserID != "" {
		preds = append(preds, grant.And(grant.SubjectType(string(authz.SubjectUser)), grant.SubjectID(p.UserID)))
	}
	if len(p.GroupIDs) > 0 {
		preds = append(preds, grant.And(grant.SubjectType(string(authz.SubjectGroup)), grant.SubjectIDIn(p.GroupIDs...)))
	}
	if p.UserID != "" && p.TenantID != "" {
		preds = append(preds, grant.And(grant.SubjectType(string(authz.SubjectTenant)), grant.SubjectID(p.TenantID)))
	}
	return grant.Or(preds...)
}

func notExpired(now time.Time) predicate.Grant {
	return grant.Or(grant.ExpiresAtIsNil(), grant.ExpiresAtGT(now))
}

// publicAllowed reports whether a tenant currently allows public sharing,
// caching the answer for the duration of one evaluation. Turning the setting
// off ends all public access at once, without touching the grants.
func (a *Authorizer) publicAllowed(ctx context.Context, cache map[string]bool, tenantID string) (bool, error) {
	if ok, seen := cache[tenantID]; seen {
		return ok, nil
	}
	t, err := a.db.Tenant.Get(ctx, tenantID)
	if err != nil {
		if ent.IsNotFound(err) {
			cache[tenantID] = false
			return false, nil
		}
		return false, fmt.Errorf("failed to load tenant: %w", err)
	}
	cache[tenantID] = t.AllowPublicSharing
	return t.AllowPublicSharing, nil
}
