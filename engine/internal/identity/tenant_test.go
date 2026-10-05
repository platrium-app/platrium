package identity_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/identity"
	"platrium/internal/infra/db/ent"
)

func TestTenantCreateAndLookup(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	tn, idp := e.tenantWithIdp(t, "  ACME ", false)
	if tn.Alias != "acme" || tn.IsNative || tn.CreatedAt.IsZero() {
		t.Fatalf("unexpected tenant: %+v", tn)
	}

	if has, _ := e.tenants.HasNativeTenant(ctx); has {
		t.Fatal("no native tenant yet")
	}
	native, _ := e.tenantWithIdp(t, "home", true)
	if has, _ := e.tenants.HasNativeTenant(ctx); !has {
		t.Fatal("native tenant should exist")
	}
	if n, _ := e.tenants.GetTenantCount(ctx); n != 2 {
		t.Fatalf("count = %d", n)
	}

	cfg, err := e.tenants.GetPublicTenantAuthConfig(ctx, "Acme") // lookup is case-insensitive
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TenantID != tn.ID || len(cfg.Providers) != 1 || cfg.Providers[0].ID != idp.ID || cfg.Providers[0].Type != "LOCAL" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg, err = e.tenants.GetPublicTenantAuthConfig(ctx, ""); err != nil || cfg.TenantID != native.ID {
		t.Fatalf("native lookup: %+v %v", cfg, err)
	}
	if _, err := e.tenants.GetPublicTenantAuthConfig(ctx, "nope"); err == nil {
		t.Fatal("expected not found")
	}
}

func TestTenantConstraints(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	e.tenantWithIdp(t, "acme", true)

	create := func(alias string, native bool) error {
		return e.db.WithTx(ctx, func(tx *ent.Tx) error {
			_, err := e.tenants.CreateTenantTx(ctx, tx, identity.CreateTenantParams{Alias: alias, Name: alias, IsNative: native})
			return err
		})
	}
	if err := create("ACME", false); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("duplicate alias (case-insensitive) must conflict, got %v", err)
	}
	if err := create("other", true); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("second native tenant must conflict, got %v", err)
	}
	if reason := e.tenants.DescribeConflict(ctx, "acme", false); reason != "a tenant with alias 'acme' already exists" {
		t.Errorf("alias conflict reason: %q", reason)
	}
	if reason := e.tenants.DescribeConflict(ctx, "other", true); reason != "a native cluster tenant already exists" {
		t.Errorf("native conflict reason: %q", reason)
	}
	if err := create("other", false); err != nil {
		t.Errorf("non-native tenant: %v", err)
	}
}
