package sqlauthz_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/grant"
	enttenant "platrium/internal/infra/db/ent/tenant"
)

func (e *env) setGeneral(t *testing.T, actor authz.Principal, itemID string, level authz.GeneralAccessLevel, role authz.Role) {
	t.Helper()
	if err := e.az.SetGeneralAccess(context.Background(), actor, authz.GeneralAccessInput{ItemID: itemID, Level: level, Role: role}); err != nil {
		t.Fatalf("set %s %s: %v", level, role, err)
	}
}

func (e *env) generalOf(t *testing.T, actor authz.Principal, itemID string) authz.GeneralAccessLevel {
	t.Helper()
	grants, err := e.az.ListGrants(context.Background(), actor, itemID)
	if err != nil {
		t.Fatal(err)
	}
	level, _ := authz.GeneralAccessOf(grants)
	return level
}

func TestGeneralAccessLevels(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)

	if got := e.generalOf(t, s.pa, s.docs); got != authz.AccessRestricted {
		t.Fatalf("starts restricted, got %s", got)
	}

	// Organization-wide: members of the tenant can open it, anonymous callers cannot.
	e.setGeneral(t, s.pa, s.docs, authz.AccessTenant, authz.RoleViewer)
	if got := e.generalOf(t, s.pa, s.docs); got != authz.AccessTenant {
		t.Fatalf("level = %s", got)
	}
	if !e.caps(t, s.pb, s.spec).Has(authz.CapView) {
		t.Error("tenant members must see it")
	}
	if e.caps(t, authz.Anonymous(), s.spec) != 0 {
		t.Error("anonymous callers must not")
	}

	// Public replaces the tenant grant instead of adding to it.
	e.setGeneral(t, s.pa, s.docs, authz.AccessPublic, authz.RoleCommenter)
	if got := e.generalOf(t, s.pa, s.docs); got != authz.AccessPublic {
		t.Fatalf("level = %s", got)
	}
	if got := e.caps(t, authz.Anonymous(), s.spec); !got.Has(authz.CapComment) {
		t.Errorf("anonymous caps = %s", got)
	}
	grants, _ := e.az.ListGrants(context.Background(), s.pa, s.docs)
	if len(grants) != 1 {
		t.Errorf("exactly one general-access grant, got %d", len(grants))
	}

	// Back to restricted removes everything general, and keeps named grants.
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	e.setGeneral(t, s.pa, s.docs, authz.AccessRestricted, "")
	if got := e.generalOf(t, s.pa, s.docs); got != authz.AccessRestricted {
		t.Fatalf("level = %s", got)
	}
	if e.caps(t, authz.Anonymous(), s.spec) != 0 || e.caps(t, s.pc, s.spec) != 0 {
		t.Error("general access must be gone")
	}
	if !e.caps(t, s.pb, s.spec).Has(authz.CapView) {
		t.Error("the named grant for bob must survive")
	}
}

func TestGeneralAccessRoles(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	cases := []struct {
		level authz.GeneralAccessLevel
		role  authz.Role
		ok    bool
	}{
		{authz.AccessTenant, authz.RoleViewer, true},
		{authz.AccessTenant, authz.RoleCommenter, true},
		{authz.AccessTenant, authz.RoleRestrictedEditor, true},
		{authz.AccessTenant, authz.RoleFullEditor, true},
		{authz.AccessTenant, authz.RoleDriveAdmin, false}, // managing access is never general
		{authz.AccessPublic, authz.RoleViewer, true},
		{authz.AccessPublic, authz.RoleCommenter, true},
		{authz.AccessPublic, authz.RoleRestrictedEditor, false}, // anonymous visitors can never write
		{authz.AccessPublic, authz.RoleFullEditor, false},
		{authz.AccessPublic, authz.RoleDriveAdmin, false},
		{authz.AccessPublic, authz.RoleOwner, false},
		{authz.AccessPublic, "", false},
		{"EVERYONE", authz.RoleViewer, false},
	}
	for _, c := range cases {
		err := e.az.SetGeneralAccess(ctx, s.pa, authz.GeneralAccessInput{ItemID: s.docs, Level: c.level, Role: c.role})
		if (err == nil) != c.ok {
			t.Errorf("%s/%s: err = %v, want ok = %v", c.level, c.role, err, c.ok)
		} else if err != nil && !errors.Is(err, authz.ErrInvalid) {
			t.Errorf("%s/%s: must be ErrInvalid, got %v", c.level, c.role, err)
		}
	}
}

func TestGeneralAccessRequiresShare(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleFullEditor) // cannot share
	in := authz.GeneralAccessInput{ItemID: s.docs, Level: authz.AccessPublic, Role: authz.RoleViewer}

	if err := e.az.SetGeneralAccess(ctx, s.pb, in); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("a content manager cannot publish: %v", err)
	}
	if err := e.az.SetGeneralAccess(ctx, s.pc, in); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("an invisible item reads as missing: %v", err)
	}
	if err := e.az.SetGeneralAccess(ctx, authz.Anonymous(), in); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("anonymous: %v", err)
	}
	if e.caps(t, authz.Anonymous(), s.docs) != 0 {
		t.Error("refused calls must change nothing")
	}
}

func TestGeneralAccessKeepsNoDownloadAndExpiry(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	soon := time.Now().Add(time.Hour)
	if err := e.az.SetGeneralAccess(ctx, s.pa, authz.GeneralAccessInput{
		ItemID: s.docs, Level: authz.AccessPublic, Role: authz.RoleViewer, NoDownload: true, ExpiresAt: &soon,
	}); err != nil {
		t.Fatal(err)
	}

	got := e.caps(t, authz.Anonymous(), s.spec)
	if !got.Has(authz.CapView) || got.Has(authz.CapDownload) {
		t.Fatalf("a view-only public link, got %s", got)
	}
	grants, _ := e.az.ListGrants(ctx, s.pa, s.docs)
	if _, g := authz.GeneralAccessOf(grants); g == nil || g.ExpiresAt == nil {
		t.Fatalf("expiry was lost: %+v", g)
	}

	// Once it expires the link stops working.
	for _, g := range grants {
		e.expire(t, g.ID)
	}
	if e.caps(t, authz.Anonymous(), s.spec) != 0 {
		t.Error("an expired link must stop working")
	}
}

// The tenant can forbid public sharing. That must block new public grants and
// also end public access that already exists, immediately and reversibly.
func TestTenantPolicyControlsPublicSharing(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.setGeneral(t, s.pa, s.docs, authz.AccessPublic, authz.RoleViewer)
	if e.caps(t, authz.Anonymous(), s.spec) == 0 {
		t.Fatal("setup")
	}

	setPolicy := func(allowed bool) {
		t.Helper()
		if err := e.db.Tenant.Update().Where(enttenant.ID(s.tn.id)).SetAllowPublicSharing(allowed).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	setPolicy(false)
	if got := e.caps(t, authz.Anonymous(), s.spec); got != 0 {
		t.Fatalf("existing public access must end when the tenant forbids it, got %s", got)
	}
	// Signed-in users do not get the public grant either.
	if got := e.caps(t, s.pb, s.spec); got != 0 {
		t.Fatalf("signed-in callers are covered by the same switch, got %s", got)
	}

	// New public grants are refused, by either route; tenant-wide still works.
	if err := e.az.SetGeneralAccess(ctx, s.pa, authz.GeneralAccessInput{ItemID: s.private, Level: authz.AccessPublic, Role: authz.RoleViewer}); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("general access: %v", err)
	}
	if _, err := e.az.Grant(ctx, s.pa, authz.GrantInput{ItemID: s.private, Subject: publicSubject(), Role: authz.RoleViewer}); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("explicit public grant: %v", err)
	}
	e.setGeneral(t, s.pa, s.private, authz.AccessTenant, authz.RoleViewer)

	// The grants were never touched, so turning the policy back on restores access.
	if n, _ := e.db.Grant.Query().Where(grant.ResourceID(s.docs), grant.SubjectType("PUBLIC")).Count(ctx); n != 1 {
		t.Fatalf("the grant must remain, found %d", n)
	}
	setPolicy(true)
	if e.caps(t, authz.Anonymous(), s.spec) == 0 {
		t.Error("access returns with the policy")
	}
}

func TestPublicAccessNeverCrossesTenants(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	other := e.tenant(t, "other")
	outsider := e.principal(t, other, e.user(t, other, "outsider"))
	e.setGeneral(t, s.pa, s.docs, authz.AccessPublic, authz.RoleViewer)

	// A signed-in user of another tenant is scoped to their own tenant, so the
	// item is invisible to them; they must open it anonymously, like anyone.
	if got := e.caps(t, outsider, s.spec); got != 0 {
		t.Fatalf("got %s", got)
	}
	if got := e.caps(t, authz.Anonymous(), s.spec); !got.Has(authz.CapView) {
		t.Fatalf("anonymous access works, got %s", got)
	}
}
