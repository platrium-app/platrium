package authz

import (
	"fmt"
	"time"
)

// Rules for changing who can access an item.
//
// An item's access is the set of grants on it, and every write (share, revoke,
// set general access, restrict, create a drive) is a change to that set. A
// writer builds the set as it would be afterwards, hands both to CheckChange,
// and writes only if it passes. The rules are plain functions over plain
// types: no database, no engine. Every Authorizer implementation, SQL or
// OpenFGA, runs the same ones, so what is allowed never depends on where the
// grants are stored. authztest holds the scenarios an adapter must pass.
//
// Adding a rule is one entry in rules and a case in authztest. Adding a writer
// means building a Change and calling CheckChange; nothing else is needed for
// it to be covered.

// ChangeOp says which write produced a Change.
type ChangeOp string

const (
	OpGrant         ChangeOp = "GRANT"          // share an item with a subject
	OpRevoke        ChangeOp = "REVOKE"         // remove a grant
	OpGeneralAccess ChangeOp = "GENERAL_ACCESS" // set restricted, organization or public
	// OpInheritance restricts or un-restricts an item. It changes no grants:
	// a drive's admins keep access through a restriction, so nobody needs a
	// grant added to stay in.
	OpInheritance ChangeOp = "INHERITANCE"
	// OpInitial is trusted server code granting the first access to something
	// it just created, with no acting user.
	OpInitial ChangeOp = "INITIAL"
)

// requested reports whether the grants in a change were asked for by the actor,
// as opposed to added by trusted server code.
func (o ChangeOp) requested() bool { return o != OpInitial }

// ItemFacts is what the rules need to know about the item beyond its grants.
// The adapter fills it in; the rules never look anything up.
type ItemFacts struct {
	ID string
	// IsDriveRoot is true for the item that is a drive itself.
	IsDriveRoot bool
	// SharedDrive is true when the item is in a shared (tenant-owned) drive.
	// Private drives have an owner who always has access, so they cannot be
	// orphaned.
	SharedDrive bool
	// Roles are the roles the item can be shared with.
	Roles RoleContext
}

// Change is a proposed edit to the grants on one item.
type Change struct {
	Op        ChangeOp
	Actor     Principal  // zero for OpInitial
	ActorCaps Capability // what the actor holds on the item, before the change
	Item      ItemFacts
	// PublicSharingAllowed is whether the actor's tenant permits public links.
	PublicSharingAllowed bool
	// Before and After are the grants placed directly on the item, as they are
	// now and as they would be after the change. A grant new to After may have
	// no ID.
	Before, After []Grant
}

// Rule is one named invariant over a Change.
type Rule struct {
	Name  string
	Check func(Change) error
}

var rules = []Rule{
	{"no-self-edit", noSelfEdit},
	{"role-offered", roleOffered},
	{"public-allowed", publicAllowed},
	{"no-escalation", noEscalation},
	{"never-orphan", neverOrphan},
}

// Rules lists the rules CheckChange applies, in order.
func Rules() []Rule { return append([]Rule(nil), rules...) }

// CheckChange returns nil if the change is allowed, or the first rule's error:
// ErrInvalid when the change makes no sense, ErrForbidden when it is not the
// actor's to make.
func CheckChange(c Change) error {
	for _, r := range rules {
		if err := r.Check(c); err != nil {
			return err
		}
	}
	return nil
}

// noSelfEdit: nobody changes their own access to an item, up or down. You
// cannot make yourself a viewer of your own file, and you cannot leave a drive
// you were put in; someone else has to do it.
func noSelfEdit(c Change) error {
	if !c.Op.requested() || c.Actor.IsAnonymous() {
		return nil
	}
	me := Subject{Type: SubjectUser, ID: c.Actor.UserID}
	if !sameGrant(find(c.Before, me), find(c.After, me)) {
		return fmt.Errorf("%w: you cannot change your own access", ErrInvalid)
	}
	return nil
}

// roleOffered: a grant may only carry a role its kind and its item offer, such
// as Drive Admin on a shared drive's root and never on a single file, and only
// carry an expiry if its general-access level supports one.
func roleOffered(c Change) error {
	if !c.Op.requested() {
		return nil
	}
	for _, g := range changed(c.Before, c.After) {
		if def, ok := levelOfSubject(g.Subject.Type); ok {
			if !def.Offers(g.Role) {
				return fmt.Errorf("%w: role %q is not available for %s access", ErrInvalid, g.Role, def.Level)
			}
			if g.ExpiresAt != nil && !def.SupportsExpiry {
				return fmt.Errorf("%w: %s access cannot expire", ErrInvalid, def.Level)
			}
			continue
		}
		if !c.Item.Roles.Offers(g.Role) {
			return fmt.Errorf("%w: role %q is not available for this item", ErrInvalid, g.Role)
		}
	}
	return nil
}

// publicAllowed: a level that needs public sharing is refused when the tenant
// has turned it off.
func publicAllowed(c Change) error {
	if !c.Op.requested() || c.PublicSharingAllowed {
		return nil
	}
	for _, g := range changed(c.Before, c.After) {
		if def, ok := levelOfSubject(g.Subject.Type); ok && def.RequiresPublicSharing {
			return fmt.Errorf("%w: your organization does not allow public sharing", ErrForbidden)
		}
	}
	return nil
}

// noEscalation: nobody hands out more than they hold.
func noEscalation(c Change) error {
	if !c.Op.requested() {
		return nil
	}
	for _, g := range changed(c.Before, c.After) {
		if !g.Caps.SubsetOf(c.ActorCaps) {
			return fmt.Errorf("%w: cannot grant capabilities you do not hold", ErrForbidden)
		}
	}
	return nil
}

// neverOrphan: a shared drive's root must keep a permanent grant that can
// manage it, held by a user or a group. A shared drive has no owner, so losing
// its last manager would leave only a tenant admin able to recover it.
// Removing, demoting or putting an end date on that grant is a change that
// fails here. A change to a drive that already had no manager is let through,
// so a broken one can be repaired. Items inside a drive need nothing of their
// own: the drive's managers keep access even through a restriction.
func neverOrphan(c Change) error {
	if !c.Item.SharedDrive || !c.Item.IsDriveRoot {
		return nil
	}
	if hasManager(c.After) || !hasManager(c.Before) {
		return nil
	}
	return fmt.Errorf("%w: this needs at least one admin; make someone else an admin first", ErrInvalid)
}

// hasManager reports whether a grant set has a permanent grant of CapManage to
// a user or a group.
func hasManager(grants []Grant) bool {
	for _, g := range grants {
		if (g.Subject.Type == SubjectUser || g.Subject.Type == SubjectGroup) &&
			g.Caps.Has(CapManage) && g.ExpiresAt == nil {
			return true
		}
	}
	return false
}

func levelOfSubject(t SubjectType) (LevelDef, bool) {
	for _, d := range levels {
		if d.Stored() && d.Subject == t {
			return d, true
		}
	}
	return LevelDef{}, false
}

func find(grants []Grant, s Subject) *Grant {
	for i := range grants {
		if grants[i].Subject == s {
			return &grants[i]
		}
	}
	return nil
}

// changed returns the grants in after that are new or differ from before.
func changed(before, after []Grant) []Grant {
	var out []Grant
	for _, g := range after {
		if !sameGrant(find(before, g.Subject), &g) {
			out = append(out, g)
		}
	}
	return out
}

// sameGrant compares what a grant gives, not its identity.
func sameGrant(a, b *Grant) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Role == b.Role && a.Caps == b.Caps && sameTime(a.ExpiresAt, b.ExpiresAt)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
