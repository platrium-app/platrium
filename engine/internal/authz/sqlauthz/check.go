package sqlauthz

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"platrium/internal/authz"
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
//     ancestor, stopping the climb at the first item that does not inherit.
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
	}
	return out, nil
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
			owners[d.ID] = d.OwnerID
		}
	}

	pending := make([]string, 0, len(items))
	for _, it := range items {
		if !p.IsAnonymous() && owners[it.DriveID] == p.UserID {
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
		for _, id := range chain {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				resourceIDs = append(resourceIDs, id)
			}
		}
	}

	byResource := map[string]authz.Capability{}
	now := time.Now().UTC()
	for chunk := range slices.Chunk(resourceIDs, grantChunk) {
		grants, err := a.db.Grant.Query().
			Where(grant.ResourceIDIn(chunk...), subjectMatches(p), notExpired(now)).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to load grants: %w", err)
		}
		for _, g := range grants {
			byResource[g.ResourceID] |= authz.Capability(g.Caps)
		}
	}

	for origin, chain := range chains {
		var caps authz.Capability
		for _, id := range chain {
			caps |= byResource[id]
		}
		out[origin] = caps
	}
	return nil
}

// ancestorChains returns, for each item, its own ID followed by its ancestors'
// IDs up to the first item that does not inherit (or the drive root).
//
// This is the only raw SQL on the check path: a recursive CTE, identical on
// Postgres, MySQL 8+, MariaDB 10.2+ and SQLite. One statement walks every
// item's chain at once, tenant-scoped at every hop.
func (a *Authorizer) ancestorChains(ctx context.Context, ids []string) (map[string][]string, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	query := a.db.Rebind(`
WITH RECURSIVE anc(origin, id, parent_id, tenant_id, inherit, depth) AS (
	SELECT id, id, parent_id, tenant_id, inherit_perms, 0 FROM drive_items WHERE id IN (` + placeholders + `)
	UNION ALL
	SELECT a.origin, p.id, p.parent_id, p.tenant_id, p.inherit_perms, a.depth + 1
	FROM drive_items p
	JOIN anc a ON p.id = a.parent_id AND p.tenant_id = a.tenant_id
	WHERE a.inherit = ? AND a.depth < ?
)
SELECT origin, id FROM anc ORDER BY origin, depth`)

	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, true, maxAncestorDepth)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to walk ancestors: %w", err)
	}
	defer rows.Close()

	chains := make(map[string][]string, len(ids))
	for rows.Next() {
		var origin, id string
		if err := rows.Scan(&origin, &id); err != nil {
			return nil, fmt.Errorf("failed to scan ancestor: %w", err)
		}
		chains[origin] = append(chains[origin], id)
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
