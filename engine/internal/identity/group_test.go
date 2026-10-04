package identity_test

import (
	"context"
	"testing"

	"platrium/internal/identity"
	"platrium/internal/infra/db/ent"
)

func TestGroupCreateAndGetByIDs(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	other, otherIdp := e.tenantWithIdp(t, "other", false)
	groups := identity.NewGroupStore(e.db)

	var a, o *identity.Group
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if a, err = groups.CreateGroupTx(ctx, tx, acme.ID, acmeIdp.ID, "design", "Design"); err != nil {
			return err
		}
		o, err = groups.CreateGroupTx(ctx, tx, other.ID, otherIdp.ID, "design", "Design")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := groups.GetByIDs(ctx, acme.ID, []string{a.ID, o.ID})
	if err != nil || len(got) != 1 || got[a.ID].Name != "Design" {
		t.Fatalf("only the tenant's own groups are returned: %+v %v", got, err)
	}

	// An IdP from another tenant cannot back a group.
	err = e.db.WithTx(ctx, func(tx *ent.Tx) error {
		_, err := groups.CreateGroupTx(ctx, tx, acme.ID, otherIdp.ID, "x", "X")
		return err
	})
	if err == nil {
		t.Error("a cross-tenant idp must be rejected")
	}
}

func TestTenantGetTenant(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, _ := e.tenantWithIdp(t, "acme", false)

	if got, err := e.tenants.GetTenant(ctx, acme.ID); err != nil || got.Alias != "acme" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := e.tenants.GetTenant(ctx, "missing"); err == nil {
		t.Fatal("expected not found")
	}
}
