package identity_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"platrium/internal/identity"
	"platrium/internal/infra/db/ent"
)

func TestPolicyGroups(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	other, otherIdp := e.tenantWithIdp(t, "other", false)
	groups := identity.NewGroupStore(e.db)
	policies := identity.NewPolicyStore(e.db)

	var g1, g2, foreign *identity.Group
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if g1, err = groups.CreateGroupTx(ctx, tx, acme.ID, acmeIdp.ID, "g1", "G1"); err != nil {
			return err
		}
		if g2, err = groups.CreateGroupTx(ctx, tx, acme.ID, acmeIdp.ID, "g2", "G2"); err != nil {
			return err
		}
		foreign, err = groups.CreateGroupTx(ctx, tx, other.ID, otherIdp.ID, "f", "F")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	const policy = identity.PolicySharedDriveCreators

	if got, _ := policies.Groups(ctx, acme.ID, policy); len(got) != 0 {
		t.Fatalf("starts empty: %v", got)
	}

	// Replacing is idempotent and tolerates duplicates.
	for i := 0; i < 2; i++ {
		if err := policies.SetGroups(ctx, acme.ID, policy, []string{g1.ID, g2.ID, g1.ID}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{g1.ID, g2.ID}
	slices.Sort(want)
	if got, _ := policies.Groups(ctx, acme.ID, policy); !slices.Equal(got, want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}

	if ok, _ := policies.AppliesTo(ctx, acme.ID, policy, []string{"x", g2.ID}); !ok {
		t.Error("applies to a member of g2")
	}
	if ok, _ := policies.AppliesTo(ctx, acme.ID, policy, []string{"x"}); ok {
		t.Error("does not apply to other groups")
	}
	if ok, _ := policies.AppliesTo(ctx, acme.ID, policy, nil); ok {
		t.Error("does not apply to no groups")
	}
	if ok, _ := policies.AppliesTo(ctx, acme.ID, "another_policy", []string{g1.ID}); ok {
		t.Error("policies are separate")
	}

	// Shrinking removes what was dropped.
	if err := policies.SetGroups(ctx, acme.ID, policy, []string{g2.ID}); err != nil {
		t.Fatal(err)
	}
	if got, _ := policies.Groups(ctx, acme.ID, policy); !slices.Equal(got, []string{g2.ID}) {
		t.Fatalf("after shrinking: %v", got)
	}

	// A group from another tenant is refused and changes nothing.
	if err := policies.SetGroups(ctx, acme.ID, policy, []string{g1.ID, foreign.ID}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("cross-tenant group: %v", err)
	}
	if got, _ := policies.Groups(ctx, acme.ID, policy); !slices.Equal(got, []string{g2.ID}) {
		t.Fatalf("a refused change must leave the policy alone: %v", got)
	}

	if err := policies.SetGroups(ctx, acme.ID, policy, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := policies.Groups(ctx, acme.ID, policy); len(got) != 0 {
		t.Fatalf("cleared: %v", got)
	}
	// Other tenants never see this tenant's policy.
	if got, _ := policies.Groups(ctx, other.ID, policy); len(got) != 0 {
		t.Fatalf("isolation: %v", got)
	}
}
