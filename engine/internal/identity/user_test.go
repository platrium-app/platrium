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
