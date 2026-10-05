package sqlauthz_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"platrium/internal/authz"
)

func TestPrincipal(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme := e.tenant(t, "acme")
	other := e.tenant(t, "other")
	alice := e.user(t, acme, "alice")
	g1, g2 := e.group(t, acme, "g1"), e.group(t, acme, "g2")
	for _, g := range []string{g1, g2} {
		if err := e.az.AddMember(ctx, acme.id, g, authz.MemberUser, alice); err != nil {
			t.Fatal(err)
		}
	}

	p, err := e.az.Principal(ctx, acme.id, alice)
	if err != nil {
		t.Fatal(err)
	}
	if p.TenantID != acme.id || p.UserID != alice || p.IsAnonymous() {
		t.Fatalf("unexpected principal: %+v", p)
	}
	want := []string{g1, g2}
	slices.Sort(want)
	if !slices.Equal(p.GroupIDs, want) {
		t.Fatalf("groups = %v, want %v", p.GroupIDs, want)
	}

	if _, err := e.az.Principal(ctx, acme.id, "missing"); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	// A user from another tenant is not a principal of this one.
	if _, err := e.az.Principal(ctx, other.id, alice); !errors.Is(err, authz.ErrNotFound) {
		t.Errorf("cross-tenant user: %v", err)
	}
	if !authz.Anonymous().IsAnonymous() {
		t.Error("Anonymous must be anonymous")
	}
}
