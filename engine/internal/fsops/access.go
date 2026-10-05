package fsops

import (
	"context"
	"fmt"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/driveitem"
)

// The capability each operation needs. Operations are enforced here, in the
// store layer, so no resolver or handler can skip a check.
//
//	list a folder, breadcrumbs ... LIST on the folder
//	item metadata, file metadata . VIEW
//	download                       DOWNLOAD
//	upload, new folder ......... CREATE on the parent folder
//	rename ..................... EDIT
//	move ....................... MOVE on the item and CREATE on the destination
//	copy ....................... DOWNLOAD on the source and CREATE on the destination

// Require checks that p holds need on an item. An item p cannot see at all is
// ErrNotFound, so existence never leaks; one they can see but may not use is
// ErrForbidden.
func (f *FSOps) Require(ctx context.Context, p authz.Principal, itemID string, need authz.Capability) error {
	_, err := f.requireCaps(ctx, p, itemID, need)
	return err
}

// requireCaps is Require that also returns the capabilities the actor holds.
func (f *FSOps) requireCaps(ctx context.Context, p authz.Principal, itemID string, need authz.Capability) (authz.Capability, error) {
	if err := requireActor(p); err != nil {
		return 0, err
	}
	caps, err := f.authz.Caps(ctx, p, itemID)
	if err != nil {
		return 0, err
	}
	return caps, denyUnless(caps, need, "item")
}

// requireActor rejects callers without an identity. Anonymous access (public
// links) goes through its own path, not these methods.
func requireActor(p authz.Principal) error {
	if p.IsAnonymous() || p.TenantID == "" {
		return fmt.Errorf("%w: sign in required", ErrForbidden)
	}
	return nil
}

// denyUnless turns a capability set into the right error for what was asked.
func denyUnless(caps, need authz.Capability, what string) error {
	if caps == 0 {
		return fmt.Errorf("%w: %s not found or access denied", ErrNotFound, what)
	}
	if !caps.Has(need) {
		return fmt.Errorf("%w: %s requires %s", ErrForbidden, what, need)
	}
	return nil
}

// readAccess authorizes a read and tells it which tenant to run in. Signed-in
// callers run in their own tenant. Anonymous callers (someone opening a public
// link) have none, so the item's tenant is used. That is safe because
// anonymous callers are only ever granted capabilities through PUBLIC grants
// on that very item, and every query that follows stays scoped to the tenant.
//
// Only read operations call this. Writes always require a signed-in actor.
func (f *FSOps) readAccess(ctx context.Context, p authz.Principal, itemID string, need authz.Capability) (tenantID string, caps authz.Capability, err error) {
	if p.IsAnonymous() {
		row, err := f.db.DriveItem.Query().Where(driveitem.ID(itemID)).Select(driveitem.FieldTenantID).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return "", 0, fmt.Errorf("%w: item not found or access denied", ErrNotFound)
			}
			return "", 0, err
		}
		tenantID = row.TenantID
	} else {
		if err := requireActor(p); err != nil {
			return "", 0, err
		}
		tenantID = p.TenantID
	}

	caps, err = f.authz.Caps(ctx, p, itemID)
	if err != nil {
		return "", 0, err
	}
	return tenantID, caps, denyUnless(caps, need, "item")
}
