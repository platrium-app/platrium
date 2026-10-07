package sqlauthz_test

import (
	"context"
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

	// The engine only expands groups. Whether the user exists or may sign in is
	// package actor's question, so a stranger is a principal with no groups.
	for name, in := range map[string][2]string{"unknown user": {acme.id, "missing"}, "user of another tenant": {other.id, alice}} {
		p, err := e.az.Principal(ctx, in[0], in[1])
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if len(p.GroupIDs) != 0 {
			t.Errorf("%s must hold no groups, got %v", name, p.GroupIDs)
		}
	}
	if !authz.Anonymous().IsAnonymous() {
		t.Error("Anonymous must be anonymous")
	}
}
