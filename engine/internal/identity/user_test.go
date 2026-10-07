package identity_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/authz"
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

func TestUserSearch(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	other, otherIdp := e.tenantWithIdp(t, "other", false)

	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		for _, u := range []identity.CreateUserParams{
			{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "1", Email: "ana@acme.com", DisplayName: "Ana Alvarez"},
			{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "2", Email: "ben@acme.com", DisplayName: "Ben Anders"},
			{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "3", Email: "cy@acme.com", DisplayName: "Cy"},
			{TenantID: other.ID, IdpID: otherIdp.ID, ExternalID: "4", Email: "ana@other.com", DisplayName: "Ana Other"},
		} {
			if _, err := e.users.CreateUserTx(ctx, tx, u); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	names := func(q string, limit int) []string {
		got, err := e.users.Search(ctx, acme.ID, q, limit)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, u := range got {
			out = append(out, u.DisplayName)
		}
		return out
	}
	if got := names("AN", 10); len(got) != 2 || got[0] != "Ana Alvarez" || got[1] != "Ben Anders" {
		t.Errorf("name match, case-insensitive, ordered: %v", got)
	}
	if got := names("cy@acme", 10); len(got) != 1 || got[0] != "Cy" {
		t.Errorf("email match: %v", got)
	}
	if got := names("ana", 1); len(got) != 1 {
		t.Errorf("limit: %v", got)
	}
	if got := names("other", 10); len(got) != 0 {
		t.Errorf("another tenant's people are invisible: %v", got)
	}
	if got := names("  ", 10); len(got) != 0 {
		t.Errorf("a blank query finds nothing: %v", got)
	}
	// Wildcards in a query are literal, never a way to list everyone.
	if got := names("%", 10); len(got) != 0 {
		t.Errorf("percent sign must not match everything: %v", got)
	}
	if got := names("_", 10); len(got) != 0 {
		t.Errorf("underscore must not match everything: %v", got)
	}
}

func TestUserSetDisabled(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)
	other, _ := e.tenantWithIdp(t, "other", false)
	var u *identity.User
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		u, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: acme.ID, IdpID: acmeIdp.ID, ExternalID: "a", Email: "a@x.com", DisplayName: "A"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if u.Disabled() {
		t.Fatal("a new user is active")
	}

	got, err := e.users.SetDisabled(ctx, acme.ID, u.ID, true)
	if err != nil || !got.Disabled() {
		t.Fatalf("disable: %+v %v", got, err)
	}
	// Disabling twice keeps the original time, so it records when it happened.
	again, err := e.users.SetDisabled(ctx, acme.ID, u.ID, true)
	if err != nil || again.DisabledAt == nil || !again.DisabledAt.Equal(*got.DisabledAt) {
		t.Fatalf("second disable must not move disabled_at: %+v %v", again, err)
	}
	got, err = e.users.SetDisabled(ctx, acme.ID, u.ID, false)
	if err != nil || got.Disabled() {
		t.Fatalf("enable: %+v %v", got, err)
	}

	// Tenant isolation: another tenant's admin cannot touch this user.
	if _, err := e.users.SetDisabled(ctx, other.ID, u.ID, true); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("cross-tenant disable must be not found, got %v", err)
	}
	if _, err := e.users.SetDisabled(ctx, acme.ID, "missing", true); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
}

func TestUserPermissions(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	home, homeIdp := e.tenantWithIdp(t, "home", true)
	acme, acmeIdp := e.tenantWithIdp(t, "acme", false)

	mk := func(tenantID, idpID, name, role string) *identity.User {
		t.Helper()
		var u *identity.User
		if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
			var err error
			u, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: tenantID, IdpID: idpID, ExternalID: name, Email: name + "@x.com", DisplayName: name, Role: role})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return u
	}
	clusterAdmin := mk(home.ID, homeIdp.ID, "root", identity.RoleSuperAdmin)
	orgAdmin := mk(acme.ID, acmeIdp.ID, "boss", identity.RoleSuperAdmin)
	member := mk(acme.ID, acmeIdp.ID, "bob", identity.RoleMember)

	perms := func(tenantID, userID string) authz.PermissionSet {
		t.Helper()
		p, _, err := e.users.Access(ctx, tenantID, userID)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if !perms(home.ID, clusterAdmin.ID).Has(authz.PermTenantsManage) {
		t.Error("the native tenant's super admin administers the cluster")
	}
	if p := perms(acme.ID, orgAdmin.ID); p.Has(authz.PermTenantsManage) || !p.Has(authz.PermUsersCreate) {
		t.Errorf("an org super admin administers only their org: %v", p.Sorted())
	}
	if p := perms(acme.ID, member.ID); p.Len() != 0 {
		t.Errorf("member: %v", p.Sorted())
	}
	// Tenant isolation: the same user ID under another tenant is nobody.
	if p := perms(home.ID, orgAdmin.ID); p.Len() != 0 {
		t.Errorf("cross-tenant lookup must grant nothing: %v", p.Sorted())
	}

	// A disabled user holds nothing, whatever their role.
	if _, err := e.users.SetDisabled(ctx, acme.ID, orgAdmin.ID, true); err != nil {
		t.Fatal(err)
	}
	if p := perms(acme.ID, orgAdmin.ID); p.Len() != 0 {
		t.Errorf("a disabled admin must hold nothing: %v", p.Sorted())
	}
}
