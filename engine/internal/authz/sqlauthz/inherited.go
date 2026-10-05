package sqlauthz

import (
	"context"
	"fmt"
	"sort"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
)

// inheritedGrants lists the grants that reach an item from above, nearest
// ancestor first. It reads the same ancestor chain the capability check does, so
// the list and the check cannot disagree: above an unrestricted item every
// grant counts, and above a restriction only grants that carry MANAGE do.
//
// Expired grants are left out, since they no longer apply. The name of an
// ancestor the actor cannot open is withheld.
func (a *Authorizer) inheritedGrants(ctx context.Context, actor authz.Principal, itemID string) ([]authz.InheritedGrant, error) {
	chains, err := a.ancestorChains(ctx, []string{itemID})
	if err != nil {
		return nil, err
	}
	chain := chains[itemID]
	if len(chain) < 2 {
		return nil, nil
	}
	above := chain[1:] // the item itself comes first

	ids := make([]string, len(above))
	depth := make(map[string]int, len(above))
	cut := make(map[string]bool, len(above))
	for i, h := range above {
		ids[i], depth[h.id], cut[h.id] = h.id, i, h.cut
	}

	rows, err := a.db.Grant.Query().
		Where(grant.ResourceIDIn(ids...), grant.TenantID(actor.TenantID)).
		Order(grant.ByCreatedAt(), grant.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load inherited grants: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	publicOK, err := a.publicAllowed(ctx, map[string]bool{}, actor.TenantID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var keep []*authz.InheritedGrant
	for _, r := range rows {
		g := grantFromEnt(r)
		if g.ExpiresAt != nil && !g.ExpiresAt.After(now) {
			continue
		}
		if cut[g.ItemID] && !g.Caps.Has(authz.CapManage) {
			continue
		}
		if g.Subject.Type == authz.SubjectPublic && !publicOK {
			continue
		}
		keep = append(keep, &authz.InheritedGrant{Grant: *g, From: authz.ItemRef{ID: g.ItemID}})
	}
	if len(keep) == 0 {
		return nil, nil
	}

	// Say where each one comes from, naming only what the actor can open.
	var fromIDs []string
	seen := map[string]bool{}
	for _, k := range keep {
		if !seen[k.From.ID] {
			seen[k.From.ID] = true
			fromIDs = append(fromIDs, k.From.ID)
		}
	}
	visible, err := a.CapsMany(ctx, actor, fromIDs)
	if err != nil {
		return nil, err
	}
	items, err := a.db.DriveItem.Query().Where(driveitem.IDIn(fromIDs...), driveitem.TenantID(actor.TenantID)).All(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(items))
	for _, it := range items {
		names[it.ID] = it.Name
	}
	for _, k := range keep {
		if visible[k.From.ID].Has(authz.CapList) {
			k.From.Name = names[k.From.ID]
		} else {
			k.From = authz.ItemRef{Name: "A folder you can't open"}
		}
	}

	sort.SliceStable(keep, func(i, j int) bool { return depth[keep[i].ItemID] < depth[keep[j].ItemID] })
	out := make([]authz.InheritedGrant, len(keep))
	for i, k := range keep {
		out[i] = *k
	}
	return out, nil
}
