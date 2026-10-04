package identity_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/identity"
	"platrium/internal/infra/db/ent"
)

func TestUserCreateAndLookup(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	_, otherIdp := e.tenantWithIdp(t, "other", false)

	var u *identity.User
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		u, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{
			TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "a@acme.com", Email: "a@acme.com", DisplayName: "A", Role: identity.RoleSuperAdmin,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != identity.RoleSuperAdmin || u.TenantID != acme.ID {
		t.Fatalf("unexpected user: %+v", u)
	}

	got, tenantID, err := e.users.GetUserByExternalId(ctx, acmeIdp.ID, "a@acme.com")
	if err != nil || got.ID != u.ID || tenantID != acme.ID {
		t.Fatalf("lookup: %+v %q %v", got, tenantID, err)
	}
	if _, _, err := e.users.GetUserByExternalId(ctx, acmeIdp.ID, "missing"); err == nil {
		t.Fatal("expected not found")
	}

	try := func(p identity.CreateUserParams) error {
		return e.db.WithTx(ctx, func(tx *ent.Tx) error { _, err := e.users.CreateUserTx(ctx, tx, p); return err })
	}
	if err := try(identity.CreateUserParams{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "a@acme.com"}); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("duplicate (idp, externalId) must conflict, got %v", err)
	}
	// Tenant isolation: another tenant's IdP can't back this tenant's user.
	if err := try(identity.CreateUserParams{TenantID: acme.ID, IdpID: otherIdp.ID, ExternalID: "x"}); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("cross-tenant idp must be rejected, got %v", err)
	}
	if err := try(identity.CreateUserParams{TenantID: "missing", IdpID: acmeIdp.ID, ExternalID: "y"}); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("unknown tenant must be not found, got %v", err)
	}
}

func TestUserGetByIDs(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	other, otherIdp := e.tenantWithIdp(t, "other", false)

	var a, o *identity.User
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if a, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "a", Email: "a@x.com", DisplayName: "A"}); err != nil {
			return err
		}
		o, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: other.ID, IdpID: otherIdp.ID, ExternalID: "o", Email: "o@x.com", DisplayName: "O"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := e.users.GetByIDs(ctx, acme.ID, []string{a.ID, o.ID, "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[a.ID] == nil || got[a.ID].DisplayName != "A" {
		t.Fatalf("only the tenant's own users are returned, got %+v", got)
	}
	if got, err := e.users.GetByIDs(ctx, acme.ID, nil); err != nil || len(got) != 0 {
		t.Errorf("empty input: %v %v", got, err)
	}
}
