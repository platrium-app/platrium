package sqlauthz_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent/groupclosure"
)

// groupsOf returns the group IDs a user belongs to, from a fresh resolution.
func (e *env) groupsOf(t *testing.T, tn tenant, userID string) []string {
	t.Helper()
	g := e.principal(t, tn, userID).GroupIDs
	slices.Sort(g)
	return g
}

func sorted(ids ...string) []string {
	slices.Sort(ids)
	return ids
}

func TestAddAndRemoveDirectMember(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	tn := e.tenant(t, "acme")
	alice := e.user(t, tn, "alice")
	g := e.group(t, tn, "g")

	if err := e.az.AddMember(ctx, tn.id, g, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
	if got := e.groupsOf(t, tn, alice); !slices.Equal(got, []string{g}) {
		t.Fatalf("groups = %v", got)
	}

	// Idempotent.
	if err := e.az.AddMember(ctx, tn.id, g, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.db.GroupClosure.Query().Where(groupclosure.GroupID(g)).Count(ctx); n != 1 {
		t.Fatalf("closure rows = %d", n)
	}

	if err := e.az.RemoveMember(ctx, tn.id, g, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
	if got := e.groupsOf(t, tn, alice); len(got) != 0 {
		t.Fatalf("groups after removal = %v", got)
	}
	// Removing a membership that is not there is a no-op.
	if err := e.az.RemoveMember(ctx, tn.id, g, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
}

func TestNestedGroupsFlatten(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	tn := e.tenant(t, "acme")
	alice, bob := e.user(t, tn, "alice"), e.user(t, tn, "bob")
	company, eng, backend := e.group(t, tn, "company"), e.group(t, tn, "eng"), e.group(t, tn, "backend")

	// Build bottom-up and top-down in different orders: the result must not depend on it.
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(e.az.AddMember(ctx, tn.id, backend, authz.MemberUser, alice))
	must(e.az.AddMember(ctx, tn.id, eng, authz.MemberGroup, backend))
	must(e.az.AddMember(ctx, tn.id, company, authz.MemberGroup, eng))
	must(e.az.AddMember(ctx, tn.id, eng, authz.MemberUser, bob))

	if got, want := e.groupsOf(t, tn, alice), sorted(backend, eng, company); !slices.Equal(got, want) {
		t.Errorf("alice groups = %v, want %v", got, want)
	}
	if got, want := e.groupsOf(t, tn, bob), sorted(eng, company); !slices.Equal(got, want) {
		t.Errorf("bob groups = %v, want %v", got, want)
	}

	// Detaching a subgroup removes its users from the ancestors, not from the subgroup.
	must(e.az.RemoveMember(ctx, tn.id, eng, authz.MemberGroup, backend))
	if got, want := e.groupsOf(t, tn, alice), []string{backend}; !slices.Equal(got, want) {
		t.Errorf("alice after detach = %v, want %v", got, want)
	}
	if got, want := e.groupsOf(t, tn, bob), sorted(eng, company); !slices.Equal(got, want) {
		t.Errorf("bob is unaffected = %v, want %v", got, want)
	}
}

func TestUserKeepsMembershipThroughAnotherPath(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	tn := e.tenant(t, "acme")
	alice := e.user(t, tn, "alice")
	top, left, right := e.group(t, tn, "top"), e.group(t, tn, "left"), e.group(t, tn, "right")

	for _, g := range []string{left, right} {
		if err := e.az.AddMember(ctx, tn.id, g, authz.MemberUser, alice); err != nil {
			t.Fatal(err)
		}
		if err := e.az.AddMember(ctx, tn.id, top, authz.MemberGroup, g); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.az.RemoveMember(ctx, tn.id, left, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
	if got, want := e.groupsOf(t, tn, alice), sorted(right, top); !slices.Equal(got, want) {
		t.Fatalf("alice still reaches top through right: %v, want %v", got, want)
	}
}

func TestGroupCyclesAreRejected(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	tn := e.tenant(t, "acme")
	a, b, c := e.group(t, tn, "a"), e.group(t, tn, "b"), e.group(t, tn, "c")
	if err := e.az.AddMember(ctx, tn.id, a, authz.MemberGroup, b); err != nil { // b in a
		t.Fatal(err)
	}
	if err := e.az.AddMember(ctx, tn.id, b, authz.MemberGroup, c); err != nil { // c in b
		t.Fatal(err)
	}

	for name, args := range map[string][2]string{
		"into itself":         {a, a},
		"a parent into child": {b, a}, // a in b, but b is in a
		"a deep ancestor":     {c, a}, // a in c, but c is under a
	} {
		if err := e.az.AddMember(ctx, tn.id, args[0], authz.MemberGroup, args[1]); !errors.Is(err, authz.ErrInvalid) {
			t.Errorf("%s must be rejected as a cycle: %v", name, err)
		}
	}
}

func TestMembershipValidation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, other := e.tenant(t, "acme"), e.tenant(t, "other")
	g := e.group(t, acme, "g")
	stranger := e.user(t, other, "stranger")
	foreign := e.group(t, other, "foreign")

	cases := []struct {
		name string
		call func() error
		want error
	}{
		{"user from another tenant", func() error { return e.az.AddMember(ctx, acme.id, g, authz.MemberUser, stranger) }, authz.ErrNotFound},
		{"group from another tenant", func() error { return e.az.AddMember(ctx, acme.id, g, authz.MemberGroup, foreign) }, authz.ErrNotFound},
		{"unknown group", func() error { return e.az.AddMember(ctx, acme.id, "nope", authz.MemberUser, stranger) }, authz.ErrNotFound},
		{"group of another tenant", func() error { return e.az.AddMember(ctx, other.id, g, authz.MemberUser, stranger) }, authz.ErrNotFound},
		{"unknown member type", func() error { return e.az.AddMember(ctx, acme.id, g, "ROBOT", "x") }, authz.ErrInvalid},
		{"unknown tenant", func() error { return e.az.AddMember(ctx, "nope", g, authz.MemberUser, stranger) }, authz.ErrNotFound},
		{"remove from unknown group", func() error { return e.az.RemoveMember(ctx, acme.id, "nope", authz.MemberUser, stranger) }, authz.ErrNotFound},
	}
	for _, c := range cases {
		if err := c.call(); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
	if n, _ := e.db.GroupClosure.Query().Count(ctx); n != 0 {
		t.Errorf("rejected changes must leave no closure rows, found %d", n)
	}
}

// The closure is derived data; RebuildClosure must be able to repair it.
func TestRebuildClosureRepairsDrift(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	tn := e.tenant(t, "acme")
	alice := e.user(t, tn, "alice")
	eng, backend := e.group(t, tn, "eng"), e.group(t, tn, "backend")
	if err := e.az.AddMember(ctx, tn.id, backend, authz.MemberUser, alice); err != nil {
		t.Fatal(err)
	}
	if err := e.az.AddMember(ctx, tn.id, eng, authz.MemberGroup, backend); err != nil {
		t.Fatal(err)
	}
	want := e.groupsOf(t, tn, alice)

	// Corrupt: drop everything, then add a bogus row.
	if _, err := e.db.GroupClosure.Delete().Exec(ctx); err != nil {
		t.Fatal(err)
	}
	other := e.user(t, tn, "bob")
	if err := e.db.GroupClosure.Create().SetTenantID(tn.id).SetGroupID(eng).SetUserID(other).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if err := e.az.RebuildClosure(ctx, tn.id); err != nil {
		t.Fatal(err)
	}
	if got := e.groupsOf(t, tn, alice); !slices.Equal(got, want) {
		t.Errorf("alice groups = %v, want %v", got, want)
	}
	if got := e.groupsOf(t, tn, other); len(got) != 0 {
		t.Errorf("the bogus row must be removed, bob groups = %v", got)
	}
}
