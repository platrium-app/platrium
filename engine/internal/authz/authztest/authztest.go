// Package authztest holds the scenarios every authz.Authorizer must pass. An
// adapter (SQL today, OpenFGA later) implements World to build a small
// directory and drive tree in its own storage, then calls Run. The scenarios
// speak only the authz.Authorizer interface, so they say what is allowed
// without saying how it is stored.
package authztest

import (
	"context"
	"errors"
	"testing"
	"time"

	"platrium/internal/authz"
)

// World builds the things a scenario needs, in the adapter's own storage.
type World interface {
	Authorizer() authz.Authorizer
	NewTenant(t *testing.T) string
	NewUser(t *testing.T, tenantID, name string) string
	NewGroup(t *testing.T, tenantID, name string) string
	// NewPrivateDrive creates a user's private drive and returns its root.
	NewPrivateDrive(t *testing.T, tenantID, ownerID string) string
	// NewSharedDrive creates a shared drive whose only admin is adminID and
	// returns its root.
	NewSharedDrive(t *testing.T, tenantID, name, adminID string) string
	// NewFolder creates a folder under parentID.
	NewFolder(t *testing.T, tenantID, parentID string) string
}

// Run runs every scenario, each against a fresh World.
func Run(t *testing.T, newWorld func(t *testing.T) World) {
	for name, fn := range scenarios {
		t.Run(name, func(t *testing.T) { fn(t, newWorld(t)) })
	}
}

var scenarios = map[string]func(*testing.T, World){
	"NoSelfEdit":      noSelfEdit,
	"LastAdminIsKept": lastAdminIsKept,
	"RestrictionKeepsAdminsAndLeavesNoGrants": restrictionKeepsAdminsAndLeavesNoGrants,
	"GroupAdminCountsAndIsKept":               groupAdminCountsAndIsKept,
	"RoleMustBeOfferedByTheItem":              roleMustBeOffered,
	"NoEscalation":                            noEscalation,
	"InheritedAccessIsListed":                 inheritedAccessIsListed,
}

// cast is the people most scenarios need, all in one tenant.
type cast struct {
	w        World
	az       authz.Authorizer
	tenant   string
	alice    string // owns a private drive
	bob, dan string
	carol    string
}

func newCast(t *testing.T, w World) *cast {
	tn := w.NewTenant(t)
	return &cast{
		w: w, az: w.Authorizer(), tenant: tn,
		alice: w.NewUser(t, tn, "alice"), bob: w.NewUser(t, tn, "bob"),
		carol: w.NewUser(t, tn, "carol"), dan: w.NewUser(t, tn, "dan"),
	}
}

func (c *cast) as(t *testing.T, user string) authz.Principal {
	t.Helper()
	p, err := c.az.Principal(context.Background(), c.tenant, user)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func user(id string) authz.Subject  { return authz.Subject{Type: authz.SubjectUser, ID: id} }
func group(id string) authz.Subject { return authz.Subject{Type: authz.SubjectGroup, ID: id} }

func (c *cast) grant(t *testing.T, actor string, item string, s authz.Subject, role authz.Role) *authz.Grant {
	t.Helper()
	g, err := c.az.Grant(context.Background(), c.as(t, actor), authz.GrantInput{ItemID: item, Subject: s, Role: role})
	if err != nil {
		t.Fatalf("grant %s %s to %s/%s: %v", role, item, s.Type, s.ID, err)
	}
	return g
}

// grantTo shares with an end date, which a grant of the last admin must refuse.
func (c *cast) grantUntil(t *testing.T, actor, item string, s authz.Subject, role authz.Role, until time.Time) error {
	t.Helper()
	_, err := c.az.Grant(context.Background(), c.as(t, actor), authz.GrantInput{ItemID: item, Subject: s, Role: role, ExpiresAt: &until})
	return err
}

func (c *cast) grantOf(t *testing.T, actor, item string, s authz.Subject) *authz.Grant {
	t.Helper()
	grants, err := c.az.ListGrants(context.Background(), c.as(t, actor), item)
	if err != nil {
		t.Fatal(err)
	}
	for i := range grants {
		if grants[i].Subject == s {
			return &grants[i]
		}
	}
	t.Fatalf("no grant to %s/%s on %s", s.Type, s.ID, item)
	return nil
}

func (c *cast) revoke(t *testing.T, actor string, g *authz.Grant) error {
	t.Helper()
	return c.az.Revoke(context.Background(), c.as(t, actor), g.ID)
}

func refused(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s: want %v, got %v", what, want, err)
	}
}

// Nobody edits their own access: not by sharing with themselves, not by
// demoting or removing the grant they were given.
func noSelfEdit(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)

	// An owner cannot make themselves a viewer of their own folder.
	drive := w.NewPrivateDrive(t, c.tenant, c.alice)
	folder := w.NewFolder(t, c.tenant, drive)
	_, err := c.az.Grant(ctx, c.as(t, c.alice), authz.GrantInput{ItemID: folder, Subject: user(c.alice), Role: authz.RoleViewer})
	refused(t, "owner shares with self", err, authz.ErrInvalid)

	// A member who can share cannot change or remove their own grant.
	shared := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	c.grant(t, c.carol, shared, user(c.bob), authz.RoleDriveAdmin)
	_, err = c.az.Grant(ctx, c.as(t, c.bob), authz.GrantInput{ItemID: shared, Subject: user(c.bob), Role: authz.RoleViewer})
	refused(t, "admin demotes self", err, authz.ErrInvalid)
	refused(t, "admin removes self", c.revoke(t, c.bob, c.grantOf(t, c.carol, shared, user(c.bob))), authz.ErrInvalid)

	// Someone else still can, and the change takes effect.
	if err := c.revoke(t, c.carol, c.grantOf(t, c.carol, shared, user(c.bob))); err != nil {
		t.Fatalf("another admin removes bob: %v", err)
	}
}

// A shared drive never loses its last permanent admin, whatever the route:
// removing, demoting or giving the grant an end date.
func lastAdminIsKept(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	// bob's admin grant ends, so he is not a permanent admin; he can act, but
	// he cannot take carol's place.
	if err := c.grantUntil(t, c.carol, drive, user(c.bob), authz.RoleDriveAdmin, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	carolGrant := c.grantOf(t, c.carol, drive, user(c.carol))
	refused(t, "remove the last admin", c.revoke(t, c.bob, carolGrant), authz.ErrInvalid)
	_, err := c.az.Grant(ctx, c.as(t, c.bob), authz.GrantInput{ItemID: drive, Subject: user(c.carol), Role: authz.RoleViewer})
	refused(t, "demote the last admin", err, authz.ErrInvalid)
	refused(t, "end-date the last admin", c.grantUntil(t, c.bob, drive, user(c.carol), authz.RoleDriveAdmin, time.Now().Add(time.Hour)), authz.ErrInvalid)

	// With a second permanent admin the first can go.
	c.grant(t, c.carol, drive, user(c.dan), authz.RoleDriveAdmin)
	if err := c.revoke(t, c.dan, carolGrant); err != nil {
		t.Fatalf("handing over: %v", err)
	}
}

// Restricting an item cuts it off from the drive's members, but never from its
// admins, and it adds no grants. Resuming leaves nothing behind either: an admin
// who is later demoted must not keep access to something they once restricted.
func restrictionKeepsAdminsAndLeavesNoGrants(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	c.grant(t, c.carol, drive, user(c.dan), authz.RoleDriveAdmin)
	c.grant(t, c.carol, drive, user(c.bob), authz.RoleFullEditor)
	folder := w.NewFolder(t, c.tenant, drive)
	file := w.NewFolder(t, c.tenant, folder)

	caps := func(user string, item string) authz.Capability {
		t.Helper()
		got, err := c.az.Caps(ctx, c.as(t, user), item)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	grants := func(item string) int { return len(mustList(t, c, item)) }

	if err := c.az.SetInheritance(ctx, c.as(t, c.carol), folder, false); err != nil {
		t.Fatal(err)
	}
	if n := grants(folder); n != 0 {
		t.Fatalf("restricting must add no grants, found %d", n)
	}
	for _, item := range []string{folder, file} {
		if !caps(c.carol, item).Has(authz.CapManage) || !caps(c.dan, item).Has(authz.CapManage) {
			t.Fatalf("drive admins keep full access under a restriction (%s)", item)
		}
		if caps(c.bob, item) != 0 {
			t.Fatalf("a member loses a restricted item (%s), has %v", item, caps(c.bob, item))
		}
	}

	if err := c.az.SetInheritance(ctx, c.as(t, c.carol), folder, true); err != nil {
		t.Fatal(err)
	}
	if n := grants(folder); n != 0 {
		t.Fatalf("resuming must leave no grants, found %d", n)
	}
	if caps(c.bob, folder) == 0 {
		t.Fatal("members get the folder back when it inherits again")
	}

	// Dan demotes carol on the drive: she must not keep anything on the folder
	// she restricted and un-restricted.
	if _, err := c.az.Grant(ctx, c.as(t, c.dan), authz.GrantInput{ItemID: drive, Subject: user(c.carol), Role: authz.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if got := caps(c.carol, folder); got.Has(authz.CapManage) || got.Has(authz.CapEdit) {
		t.Fatalf("a demoted admin keeps nothing on the folder, has %v", got)
	}
}

func mustList(t *testing.T, c *cast, item string) []authz.Grant {
	t.Helper()
	grants, err := c.az.ListGrants(context.Background(), c.as(t, c.carol), item)
	if err != nil {
		t.Fatal(err)
	}
	return grants
}

// A group's admin grant counts as an admin, and when it is the only one it
// cannot be removed or demoted, even by a member of the group.
func groupAdminCountsAndIsKept(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	admins := w.NewGroup(t, c.tenant, "admins")
	if err := c.az.AddMember(ctx, c.tenant, admins, authz.MemberUser, c.bob); err != nil {
		t.Fatal(err)
	}
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	c.grant(t, c.carol, drive, group(admins), authz.RoleDriveAdmin)

	// With a group admin in place, carol may be removed by a member of it.
	if err := c.revoke(t, c.bob, c.grantOf(t, c.bob, drive, user(c.carol))); err != nil {
		t.Fatalf("a group admin keeps the drive managed: %v", err)
	}
	// Now the group is the only admin. Bob holds access through it, which is
	// not a grant to him, so only the last-admin rule stops him.
	refused(t, "remove the only admin group", c.revoke(t, c.bob, c.grantOf(t, c.bob, drive, group(admins))), authz.ErrInvalid)
	_, err := c.az.Grant(ctx, c.as(t, c.bob), authz.GrantInput{ItemID: drive, Subject: group(admins), Role: authz.RoleViewer})
	refused(t, "demote the only admin group", err, authz.ErrInvalid)
}

// A role is only offered where it makes sense: Drive Admin on a drive, never
// on a single folder.
func roleMustBeOffered(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	folder := w.NewFolder(t, c.tenant, drive)
	_, err := c.az.Grant(ctx, c.as(t, c.carol), authz.GrantInput{ItemID: folder, Subject: user(c.bob), Role: authz.RoleDriveAdmin})
	refused(t, "drive admin on a folder", err, authz.ErrInvalid)
	c.grant(t, c.carol, folder, user(c.bob), authz.RoleFullEditor)
}

// Nobody hands out more than they hold. Through the API only a viewer is short
// of what a role carries; the capability arithmetic is covered in rules_test.go.
func noEscalation(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	folder := w.NewFolder(t, c.tenant, drive)
	c.grant(t, c.carol, drive, user(c.bob), authz.RoleViewer)

	// A viewer cannot share at all.
	_, err := c.az.Grant(ctx, c.as(t, c.bob), authz.GrantInput{ItemID: folder, Subject: user(c.dan), Role: authz.RoleViewer})
	refused(t, "viewer shares", err, authz.ErrForbidden)

}

// An item's access list includes what reaches it from above, so a client can show
// everyone who can open it, each with where that comes from. It follows the same
// rules as the check: above a restriction only the drive's admins remain.
func inheritedAccessIsListed(t *testing.T, w World) {
	ctx := context.Background()
	c := newCast(t, w)
	drive := w.NewSharedDrive(t, c.tenant, "Finance", c.carol)
	c.grant(t, c.carol, drive, user(c.bob), authz.RoleFullEditor)
	folder := w.NewFolder(t, c.tenant, drive)
	file := w.NewFolder(t, c.tenant, folder)
	c.grant(t, c.carol, folder, user(c.dan), authz.RoleViewer)

	inherited := func(item string) map[string]authz.InheritedGrant {
		t.Helper()
		access, err := c.az.ItemAccess(ctx, c.as(t, c.carol), item)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]authz.InheritedGrant{}
		for _, g := range access.Inherited {
			out[g.Subject.ID] = g
		}
		return out
	}

	got := inherited(file)
	if len(got) != 3 {
		t.Fatalf("file should inherit carol and bob from the drive and dan from the folder, got %d", len(got))
	}
	if got[c.carol].Role != authz.RoleDriveAdmin || got[c.carol].From.Name != "Finance" || got[c.carol].From.ID != drive {
		t.Errorf("carol should come from the drive: %+v", got[c.carol])
	}
	if got[c.dan].From.ID != folder {
		t.Errorf("dan should come from the folder: %+v", got[c.dan])
	}

	// The drive's own list has nothing above it, and the folder's direct grants
	// are not repeated as inherited.
	if len(inherited(drive)) != 0 {
		t.Errorf("a drive root inherits nothing")
	}
	if _, repeated := inherited(folder)[c.dan]; repeated {
		t.Errorf("a direct grant is not also inherited")
	}

	// Restrict the file: members and the folder's viewer drop out of the list,
	// the drive's admin stays.
	if err := c.az.SetInheritance(ctx, c.as(t, c.carol), file, false); err != nil {
		t.Fatal(err)
	}
	got = inherited(file)
	if _, ok := got[c.carol]; !ok || len(got) != 1 {
		t.Fatalf("only the drive admin remains above a restriction, got %v", got)
	}
}
