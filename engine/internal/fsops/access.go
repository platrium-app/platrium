package fsops

import (
	"context"
	"fmt"

	"platrium/internal/authz"
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
	if err := requireActor(p); err != nil {
		return err
	}
	caps, err := f.authz.Caps(ctx, p, itemID)
	if err != nil {
		return err
	}
	return denyUnless(caps, need, "item")
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
