package sqlauthz_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/grant"
)

func TestGrantRequiresShare(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	in := authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.carol), Role: authz.RoleViewer}

	// Bob can see docs but is only a content manager: no SHARE.
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleContentManager)
	if _, err := e.az.Grant(ctx, s.pb, in); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("a content manager cannot share: %v", err)
	}
	// Carol cannot even see the item: it does not exist as far as she knows.
	if _, err := e.az.Grant(ctx, s.pc, in); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("an invisible item must read as not found: %v", err)
	}
	if _, err := e.az.Grant(ctx, authz.Anonymous(), in); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("anonymous cannot share: %v", err)
	}
}

func TestManagersCanShareFurther(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleManager)

	g := e.share(t, s.pb, s.docs, userSubject(s.carol), authz.RoleEditor)
	if g.CreatedBy != s.bob || g.Role != authz.RoleEditor {
		t.Fatalf("unexpected grant: %+v", g)
	}
	if got := e.caps(t, s.pc, s.spec); !got.Has(authz.CapEdit) {
		t.Fatalf("carol caps = %s", got)
	}
}

// Nobody can hand out more than they hold.
func TestCannotGrantMoreThanYouHold(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	// A custom grant that can share but not edit.
	limited := authz.Normalize(authz.CapView | authz.CapShare)
	if err := e.db.Grant.Create().SetTenantID(s.tn.id).SetDriveID(s.drive).SetResourceID(s.docs).
		SetSubjectType("USER").SetSubjectID(s.bob).SetRole("CUSTOM").SetCaps(int64(limited)).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	_, err := e.az.Grant(ctx, s.pb, authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.carol), Role: authz.RoleEditor})
	if !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("escalation must be refused: %v", err)
	}
}

func TestGrantReplacesEarlierGrant(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	first := e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleEditor)
	soon := time.Now().Add(time.Hour)
	second, err := e.az.Grant(ctx, s.pa, authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: authz.RoleViewer, ExpiresAt: &soon})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Role != authz.RoleViewer || second.ExpiresAt == nil {
		t.Fatalf("sharing again must replace the grant: %+v -> %+v", first, second)
	}
	if n, _ := e.db.Grant.Query().Where(grant.ResourceID(s.docs)).Count(ctx); n != 1 {
		t.Fatalf("expected one grant, got %d", n)
	}
	if got := e.caps(t, s.pb, s.docs); got.Has(authz.CapEdit) {
		t.Fatalf("the downgrade must take effect, got %s", got)
	}

	third := e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer) // no expiry now
	if third.ExpiresAt != nil {
		t.Fatalf("re-sharing without an expiry clears it: %+v", third)
	}
}

func TestGrantValidation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	other := e.tenant(t, "other")
	stranger := e.user(t, other, "stranger")
	foreignGroup := e.group(t, other, "foreign")
	past := time.Now().Add(-time.Hour)

	cases := []struct {
		name string
		in   authz.GrantInput
		want error
	}{
		{"owner is not grantable", authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: authz.RoleOwner}, authz.ErrInvalid},
		{"custom is not grantable", authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: authz.RoleCustom}, authz.ErrInvalid},
		{"unknown role", authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: "BOSS"}, authz.ErrInvalid},
		{"expiry in the past", authz.GrantInput{ItemID: s.docs, Subject: userSubject(s.bob), Role: authz.RoleViewer, ExpiresAt: &past}, authz.ErrInvalid},
		{"unknown subject type", authz.GrantInput{ItemID: s.docs, Subject: authz.Subject{Type: "ROBOT", ID: "x"}, Role: authz.RoleViewer}, authz.ErrInvalid},
		{"user from another tenant", authz.GrantInput{ItemID: s.docs, Subject: userSubject(stranger), Role: authz.RoleViewer}, authz.ErrNotFound},
		{"group from another tenant", authz.GrantInput{ItemID: s.docs, Subject: groupSubject(foreignGroup), Role: authz.RoleViewer}, authz.ErrNotFound},
		{"missing user", authz.GrantInput{ItemID: s.docs, Subject: userSubject("nobody"), Role: authz.RoleViewer}, authz.ErrNotFound},
		{"another tenant as subject", authz.GrantInput{ItemID: s.docs, Subject: authz.Subject{Type: authz.SubjectTenant, ID: other.id}, Role: authz.RoleViewer}, authz.ErrInvalid},
		{"public with a made-up id", authz.GrantInput{ItemID: s.docs, Subject: authz.Subject{Type: authz.SubjectPublic, ID: "everyone"}, Role: authz.RoleViewer}, authz.ErrInvalid},
	}
	for _, c := range cases {
		if _, err := e.az.Grant(ctx, s.pa, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if n, _ := e.db.Grant.Query().Count(ctx); n != 0 {
		t.Errorf("rejected grants must leave nothing behind, found %d", n)
	}
}

func TestRevoke(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	other := e.tenant(t, "other")
	outsider := e.principal(t, other, e.user(t, other, "outsider"))

	g := e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	if e.caps(t, s.pb, s.docs) == 0 {
		t.Fatal("setup")
	}

	if err := e.az.Revoke(ctx, s.pb, g.ID); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("a viewer cannot revoke: %v", err)
	}
	if err := e.az.Revoke(ctx, outsider, g.ID); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("another tenant cannot see the grant: %v", err)
	}
	if err := e.az.Revoke(ctx, s.pa, "missing"); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("missing grant: %v", err)
	}

	if err := e.az.Revoke(ctx, s.pa, g.ID); err != nil {
		t.Fatal(err)
	}
	if got := e.caps(t, s.pb, s.docs); got != 0 {
		t.Fatalf("revocation must take effect immediately, got %s", got)
	}
}

func TestListGrants(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	e.share(t, s.pa, s.docs, tenantSubject(s.tn), authz.RoleViewer)
	e.share(t, s.pa, s.private, userSubject(s.carol), authz.RoleViewer)

	grants, err := e.az.ListGrants(ctx, s.pa, s.docs)
	if err != nil || len(grants) != 2 {
		t.Fatalf("grants on docs: %v %v", grants, err)
	}
	for _, g := range grants {
		if g.ItemID != s.docs || g.DriveID != s.drive || g.TenantID != s.tn.id {
			t.Errorf("unexpected grant: %+v", g)
		}
	}
	if _, err := e.az.ListGrants(ctx, s.pb, s.docs); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("only sharers see who has access: %v", err)
	}
}

func TestSetInheritance(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleEditor)    // can share, cannot manage
	e.share(t, s.pa, s.docs, userSubject(s.carol), authz.RoleManager) // can manage

	if err := e.az.SetInheritance(ctx, s.pb, s.specs, false); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("an editor cannot restrict: %v", err)
	}
	if err := e.az.SetInheritance(ctx, s.pa, s.drive, false); !errors.Is(err, authz.ErrInvalid) {
		t.Errorf("a drive root has nothing to inherit: %v", err)
	}

	// A manager who restricts an item keeps access to it through a direct grant.
	if err := e.az.SetInheritance(ctx, s.pc, s.specs, false); err != nil {
		t.Fatal(err)
	}
	if got := e.caps(t, s.pc, s.spec); !got.Has(authz.CapManage) {
		t.Fatalf("the manager must not lock themselves out, got %s", got)
	}
	if got := e.caps(t, s.pb, s.spec); got != 0 {
		t.Fatalf("bob loses inherited access, got %s", got)
	}

	// The owner needs no compensating grant.
	if err := e.az.SetInheritance(ctx, s.pa, s.private, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.db.Grant.Query().Where(grant.ResourceID(s.private)).Count(ctx); n != 0 {
		t.Errorf("owners get no extra grant, found %d", n)
	}
	// Setting the current value again is a no-op.
	if err := e.az.SetInheritance(ctx, s.pa, s.private, false); err != nil {
		t.Errorf("idempotent: %v", err)
	}
}
