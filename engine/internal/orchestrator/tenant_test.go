package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/orchestrator"
)

func TestProvisionNewTenant(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)

	tenants := identity.NewTenantStore(d)
	users := identity.NewUserStore(d)
	idps := auth.NewIdpStore(d)
	az := sqlauthz.New(d)
	fs := fsops.NewFSOps(d, nil, az)
	localStore := local.NewLocalUserStore(d, orchestrator.NewUserOrchestrator(users, fs))
	to := orchestrator.NewTenantOrchestrator(d, tenants, idps, localStore)

	tn, err := to.ProvisionNewTenant(ctx, "Home", "home", "admin@home.org", "pw-12345678", true)
	if err != nil {
		t.Fatal(err)
	}
	if !tn.IsNative || tn.Alias != "home" {
		t.Fatalf("unexpected tenant: %+v", tn)
	}

	// Tenant, local IdP, super admin and personal drive all exist.
	cfg, err := tenants.GetPublicTenantAuthConfig(ctx, "home")
	if err != nil || len(cfg.Providers) != 1 {
		t.Fatalf("auth config: %+v %v", cfg, err)
	}
	user, tenantID, err := users.GetUserByExternalId(ctx, cfg.Providers[0].ID, "admin@home.org")
	if err != nil || tenantID != tn.ID || user.Role != identity.RoleSuperAdmin {
		t.Fatalf("admin: %+v %q %v", user, tenantID, err)
	}
	if ok, err := localStore.VerifyPassword(ctx, user.ID, "pw-12345678"); err != nil || !ok {
		t.Fatalf("the admin password must work immediately after provisioning: %v %v", ok, err)
	}
	principal, err := az.Principal(ctx, tn.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	drives, err := fs.GetUserDrives(ctx, principal)
	if err != nil || len(drives) != 1 || drives[0].Type != fsops.DriveTypePrivate {
		t.Fatalf("drives: %+v %v", drives, err)
	}
	if n, _ := d.DriveItem.Query().Count(ctx); n != 1 {
		t.Fatalf("expected a root folder item, got %d", n)
	}

	// A failure rolls back everything: a second native tenant must leave no trace.
	if _, err := to.ProvisionNewTenant(ctx, "Again", "again", "x@again.org", "pw-12345678", true); err == nil {
		t.Fatal("second native tenant must fail")
	}
	if n, _ := tenants.GetTenantCount(ctx); n != 1 {
		t.Fatalf("tenant count after rollback = %d", n)
	}
	if n, _ := d.User.Query().Count(ctx); n != 1 {
		t.Fatalf("user count after rollback = %d", n)
	}
	if n, _ := d.LocalCredential.Query().Count(ctx); n != 1 {
		t.Fatalf("credential count after rollback = %d", n)
	}

	// The conflict reason names the real cause.
	if _, err := to.ProvisionNewTenant(ctx, "Home 2", "HOME", "x@home.org", "pw-12345678", false); !errors.Is(err, identity.ErrConflict) || !strings.Contains(err.Error(), "alias 'home' already exists") {
		t.Fatalf("duplicate alias error: %v", err)
	}
}

func TestProvisionNewTenantStoresTheLoginInLowercase(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	users := identity.NewUserStore(d)
	fs := fsops.NewFSOps(d, nil, sqlauthz.New(d))
	to := orchestrator.NewTenantOrchestrator(d, identity.NewTenantStore(d), auth.NewIdpStore(d), local.NewLocalUserStore(d, orchestrator.NewUserOrchestrator(users, fs)))

	tn, err := to.ProvisionNewTenant(ctx, "Home", "home", "  Admin@Home.ORG ", "pw-12345678", true)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := identity.NewTenantStore(d).GetPublicTenantAuthConfig(ctx, tn.Alias)
	if err != nil {
		t.Fatal(err)
	}
	// The login handler looks users up with local.NormalizeLogin(whatever was typed).
	u, _, err := users.GetUserByExternalId(ctx, cfg.Providers[0].ID, local.NormalizeLogin("ADMIN@home.org"))
	if err != nil || u.Email != "admin@home.org" {
		t.Fatalf("lookup by any casing must find the admin: %+v %v", u, err)
	}
}

// The first administrator is held to the same rules as any local user, so the
// setup flow cannot create an account an admin could not.
func TestProvisionNewTenantValidatesTheAdmin(t *testing.T) {
	ctx := context.Background()
	d := dbtest.New(t)
	users := identity.NewUserStore(d)
	fs := fsops.NewFSOps(d, nil, sqlauthz.New(d))
	to := orchestrator.NewTenantOrchestrator(d, identity.NewTenantStore(d), auth.NewIdpStore(d), local.NewLocalUserStore(d, orchestrator.NewUserOrchestrator(users, fs)))

	for name, c := range map[string]struct{ email, password string }{
		"short password": {"admin@home.org", "1"},
		"long password":  {"admin@home.org", strings.Repeat("x", 73)},
		"not an email":   {"admin", "pw-12345678"},
		"name and email": {"Admin <admin@home.org>", "pw-12345678"},
	} {
		if _, err := to.ProvisionNewTenant(ctx, "Home", "home", c.email, c.password, true); !errors.Is(err, authz.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if n, _ := d.Tenant.Query().Count(ctx); n != 0 {
		t.Errorf("a refused setup must leave nothing behind, found %d tenants", n)
	}
	// The email is stored in its normal form.
	if _, err := to.ProvisionNewTenant(ctx, "Home", "home", "  Admin@Home.ORG ", "pw-12345678", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := users.GetUserByExternalId(ctx, mustLocalIdp(t, d), "admin@home.org"); err != nil {
		t.Errorf("the admin must be stored under the normalized email: %v", err)
	}
}

func mustLocalIdp(t *testing.T, d *db.DB) string {
	t.Helper()
	id, err := d.IdpProvider.Query().OnlyID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return id
}
