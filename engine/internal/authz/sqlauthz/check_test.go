package sqlauthz_test

import (
	"context"
	"fmt"
	"testing"

	"platrium/internal/authz"
)

// scene is a drive owned by alice with a small tree:
//
//	root
//	├── docs
//	│   ├── specs
//	│   │   └── spec.txt
//	│   └── notes.txt
//	└── private
type scene struct {
	tn                              tenant
	alice, bob, carol               string
	pa, pb, pc                      authz.Principal
	drive, docs, specs, spec, notes string
	private                         string
}

func newScene(t *testing.T, e *env) scene {
	t.Helper()
	var s scene
	s.tn = e.tenant(t, "acme")
	s.alice, s.bob, s.carol = e.user(t, s.tn, "alice"), e.user(t, s.tn, "bob"), e.user(t, s.tn, "carol")
	s.drive = e.drive(t, s.tn, s.alice)
	s.docs = e.folder(t, s.tn, s.drive, s.drive, "docs")
	s.specs = e.folder(t, s.tn, s.drive, s.docs, "specs")
	s.spec = e.file(t, s.tn, s.drive, s.specs, "spec.txt")
	s.notes = e.file(t, s.tn, s.drive, s.docs, "notes.txt")
	s.private = e.folder(t, s.tn, s.drive, s.drive, "private")
	s.pa, s.pb, s.pc = e.principal(t, s.tn, s.alice), e.principal(t, s.tn, s.bob), e.principal(t, s.tn, s.carol)
	return s
}

func TestOwnerHoldsEverything(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	for _, id := range []string{s.drive, s.docs, s.spec, s.private} {
		if got := e.caps(t, s.pa, id); got != authz.AllCaps {
			t.Errorf("owner caps on %s = %s", id, got)
		}
	}
}

func TestNoGrantsNoAccess(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	if got := e.caps(t, s.pb, s.docs); got != 0 {
		t.Fatalf("bob has %s with no grants", got)
	}
	ok, err := e.az.Check(ctx, s.pb, s.docs, authz.CapView)
	if err != nil || ok {
		t.Fatalf("Check = %v, %v", ok, err)
	}
	// A missing item looks exactly like a forbidden one: no error, no access.
	ok, err = e.az.Check(ctx, s.pa, "does-not-exist", authz.CapList)
	if err != nil || ok {
		t.Fatalf("missing item: %v, %v", ok, err)
	}
}

func TestGrantAppliesToSubtree(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)

	viewer, _ := authz.RoleViewer.Caps()
	for _, id := range []string{s.docs, s.specs, s.spec, s.notes} {
		if got := e.caps(t, s.pb, id); got != viewer {
			t.Errorf("bob on %s = %s, want %s", id, got, viewer)
		}
	}
	for _, id := range []string{s.drive, s.private} {
		if got := e.caps(t, s.pb, id); got != 0 {
			t.Errorf("a grant on docs must not reach %s, got %s", id, got)
		}
	}
	if got := e.caps(t, s.pc, s.docs); got != 0 {
		t.Errorf("carol was not granted anything, got %s", got)
	}
}

func TestNoDownloadGrant(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	if _, err := e.az.Grant(ctx, s.pa, authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: authz.RoleViewer, NoDownload: true}); err != nil {
		t.Fatal(err)
	}

	got := e.caps(t, s.pb, s.spec)
	if !got.Has(authz.CapView) || got.Has(authz.CapDownload) {
		t.Fatalf("view-only caps = %s", got)
	}
}

func TestCapabilitiesFromSeveralGrantsAreUnioned(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	team := e.group(t, s.tn, "team")
	if err := e.az.AddMember(ctx, s.tn.id, team, authz.MemberUser, s.bob); err != nil {
		t.Fatal(err)
	}
	pb := e.principal(t, s.tn, s.bob) // re-resolve to pick up the group

	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	e.share(t, s.pa, s.specs, groupSubject(team), authz.RoleFullEditor)

	if got := e.caps(t, pb, s.spec); !got.Has(authz.CapEdit) || !got.Has(authz.CapDownload) {
		t.Fatalf("union on spec = %s", got)
	}
	if got := e.caps(t, pb, s.notes); got.Has(authz.CapEdit) {
		t.Fatalf("the editor grant is on specs only, got %s on notes", got)
	}
}

func TestGroupGrantReachesNestedMembers(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	eng, backend := e.group(t, s.tn, "eng"), e.group(t, s.tn, "backend")
	if err := e.az.AddMember(ctx, s.tn.id, eng, authz.MemberGroup, backend); err != nil {
		t.Fatal(err)
	}
	if err := e.az.AddMember(ctx, s.tn.id, backend, authz.MemberUser, s.carol); err != nil {
		t.Fatal(err)
	}
	e.share(t, s.pa, s.docs, groupSubject(eng), authz.RoleRestrictedEditor)

	pc := e.principal(t, s.tn, s.carol)
	if got := e.caps(t, pc, s.spec); !got.Has(authz.CapCreate) {
		t.Fatalf("carol reaches the grant through backend -> eng, got %s", got)
	}

	if err := e.az.RemoveMember(ctx, s.tn.id, backend, authz.MemberUser, s.carol); err != nil {
		t.Fatal(err)
	}
	pc = e.principal(t, s.tn, s.carol)
	if got := e.caps(t, pc, s.spec); got != 0 {
		t.Fatalf("access must end with membership, got %s", got)
	}
}

func TestTenantWideGrant(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	other := e.tenant(t, "other")
	mallory := e.principal(t, other, e.user(t, other, "mallory"))

	e.share(t, s.pa, s.docs, tenantSubject(s.tn), authz.RoleViewer)

	for _, p := range []authz.Principal{s.pb, s.pc} {
		if got := e.caps(t, p, s.spec); !got.Has(authz.CapView) {
			t.Errorf("every member of the tenant is covered, got %s", got)
		}
	}
	if got := e.caps(t, mallory, s.spec); got != 0 {
		t.Errorf("another tenant's user must see nothing, got %s", got)
	}
}

func TestPublicGrant(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, publicSubject(), authz.RoleViewer)

	if got := e.caps(t, authz.Anonymous(), s.spec); !got.Has(authz.CapView) {
		t.Fatalf("anonymous caps on a public subtree = %s", got)
	}
	if got := e.caps(t, authz.Anonymous(), s.private); got != 0 {
		t.Fatalf("public docs must not expose private, got %s", got)
	}
	// Anonymous callers only ever match PUBLIC grants.
	e.share(t, s.pa, s.private, userSubject(s.bob), authz.RoleViewer)
	e.share(t, s.pa, s.private, tenantSubject(s.tn), authz.RoleViewer)
	if got := e.caps(t, authz.Anonymous(), s.private); got != 0 {
		t.Fatalf("user and tenant grants must not apply to anonymous, got %s", got)
	}
}

func TestBreakingInheritance(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleFullEditor)

	if err := e.az.SetInheritance(ctx, s.pa, s.specs, false); err != nil {
		t.Fatal(err)
	}
	if got := e.caps(t, s.pb, s.docs); !got.Has(authz.CapEdit) {
		t.Fatalf("docs is unaffected, got %s", got)
	}
	for _, id := range []string{s.specs, s.spec} {
		if got := e.caps(t, s.pb, id); got != 0 {
			t.Errorf("bob's inherited access must stop at the restricted folder, got %s on %s", got, id)
		}
	}
	if got := e.caps(t, s.pa, s.spec); got != authz.AllCaps {
		t.Errorf("the owner is never locked out, got %s", got)
	}

	// A direct grant at the boundary opens it up again, for the whole subtree below.
	e.share(t, s.pa, s.specs, userSubject(s.bob), authz.RoleViewer)
	if got := e.caps(t, s.pb, s.spec); !got.Has(authz.CapView) || got.Has(authz.CapEdit) {
		t.Fatalf("direct viewer grant only, got %s", got)
	}

	// Resuming inheritance restores the original behavior.
	if err := e.az.SetInheritance(ctx, s.pa, s.specs, true); err != nil {
		t.Fatal(err)
	}
	if got := e.caps(t, s.pb, s.spec); !got.Has(authz.CapEdit) {
		t.Fatalf("inheritance resumed, got %s", got)
	}
}

func TestExpiredGrantsAreIgnored(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	g := e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	if e.caps(t, s.pb, s.docs) == 0 {
		t.Fatal("setup: the grant should work")
	}

	e.expire(t, g.ID)
	if got := e.caps(t, s.pb, s.docs); got != 0 {
		t.Fatalf("expired grant still applies: %s", got)
	}
}

func TestTenantIsolation(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	evil := e.tenant(t, "evil")
	eve := e.principal(t, evil, e.user(t, evil, "eve"))

	e.share(t, s.pa, s.docs, tenantSubject(s.tn), authz.RoleDriveAdmin)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleDriveAdmin)

	ids := []string{s.drive, s.docs, s.spec, s.private}
	got, err := e.az.CapsMany(context.Background(), eve, ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if c, ok := got[id]; !ok || c != 0 {
			t.Errorf("another tenant's principal holds %s on %s", c, id)
		}
	}
}

func TestCapsManyMatchesCaps(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.specs, userSubject(s.bob), authz.RoleFullEditor)
	e.share(t, s.pa, s.notes, userSubject(s.bob), authz.RoleViewer)

	ids := []string{s.drive, s.docs, s.specs, s.spec, s.notes, s.private, "missing", s.spec} // includes a dup
	many, err := e.az.CapsMany(ctx, s.pb, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(many) != 7 {
		t.Fatalf("every distinct id gets an entry, got %d: %v", len(many), many)
	}
	for _, id := range ids {
		single, err := e.az.Caps(ctx, s.pb, id)
		if err != nil || single != many[id] {
			t.Errorf("%s: Caps = %s, CapsMany = %s (%v)", id, single, many[id], err)
		}
	}
}

func TestCapsManyAcrossChunks(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)

	var ids []string
	for i := 0; i < 620; i++ { // more than one chunk
		ids = append(ids, e.file(t, s.tn, s.drive, s.docs, fmt.Sprintf("f%d", i)))
	}
	got, err := e.az.CapsMany(ctx, s.pb, ids)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !got[id].Has(authz.CapView) {
			t.Fatalf("%s: %s", id, got[id])
		}
	}
}

func TestDeepTree(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	parent := s.private
	for i := 0; i < 40; i++ {
		parent = e.folder(t, s.tn, s.drive, parent, fmt.Sprintf("d%d", i))
	}
	e.share(t, s.pa, s.private, userSubject(s.bob), authz.RoleViewer)

	if got := e.caps(t, s.pb, parent); !got.Has(authz.CapView) {
		t.Fatalf("a grant 40 levels up still applies, got %s", got)
	}
}

// Capabilities this build does not know about must pass through untouched, so a
// rolling upgrade cannot corrupt grants a newer node wrote.
func TestUnknownCapabilityBitsSurvive(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	future := authz.Capability(1) << 30
	if err := e.db.Grant.Create().SetTenantID(s.tn.id).SetDriveID(s.drive).SetResourceID(s.docs).
		SetSubjectType("USER").SetSubjectID(s.bob).SetRole("CUSTOM").SetCaps(int64(authz.CapView | authz.CapList | future)).
		Exec(ctx); err != nil {
		t.Fatal(err)
	}

	got := e.caps(t, s.pb, s.docs)
	if got.Unknown() != future || !got.Has(authz.CapView) {
		t.Fatalf("caps = %s", got)
	}
}
