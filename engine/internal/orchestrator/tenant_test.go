package orchestrator_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
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
	to := orchestrator.NewTenantOrchestrator(d, tenants, idps, orchestrator.NewUserOrchestrator(users, fs), local.NewLocalUserStore(d))

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
	if ok, err := local.NewLocalUserStore(d).VerifyPassword(ctx, user.ID, "pw-12345678"); err != nil || !ok {
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
