package auth_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/apperr"
	"platrium/internal/auth"
	"platrium/internal/infra/db/ent"
)

// The built-in provider's rules live in IdpStore alone: whatever calls it
// (an API, a CLI, SCIM) gets the same answer.
func TestBuiltInProviderCannotBeCreatedEditedDisabledOrDeleted(t *testing.T) {
	e := newMenv(t, jit)
	ctx := context.Background()
	name, on := "Renamed", true

	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		_, err := e.idps.CreateTx(ctx, tx, auth.CreateIdpParams{TenantID: e.tenant.ID, Type: auth.IdpTypeLocal, Name: "Second", ProvisioningPolicy: jit})
		return err
	})
	if !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("create: err = %v, want ErrInvalid", err)
	}
	if _, err := e.idps.Update(ctx, e.tenant.ID, e.local.ID, auth.UpdateIdpParams{Name: &name}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("update: err = %v", err)
	}
	if _, err := e.idps.Update(ctx, e.tenant.ID, e.local.ID, auth.UpdateIdpParams{JITUsers: &on}); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("update policy: err = %v", err)
	}
	for _, enabled := range []bool{false, true} {
		if _, err := e.idps.SetEnabled(ctx, e.tenant.ID, e.local.ID, enabled); !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("SetEnabled(%v): err = %v", enabled, err)
		}
	}
	if err := e.idps.Delete(ctx, e.tenant.ID, e.local.ID); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("delete: err = %v", err)
	}

	got, err := e.idps.GetInTenant(ctx, e.tenant.ID, e.local.ID)
	if err != nil || got.Name != e.local.Name || !got.Enabled || got.JITUsers {
		t.Errorf("built-in = %+v, %v", got, err)
	}
	if n := e.db.IdpProvider.Query().CountX(ctx); n != 2 {
		t.Errorf("providers = %d, want the 2 from setup", n)
	}
}

func TestStoreOperationsStayInTheTenant(t *testing.T) {
	e := newMenv(t, jit)
	ctx := context.Background()
	name := "x"

	if _, err := e.idps.GetInTenant(ctx, "other-tenant", e.oidc.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := e.idps.Update(ctx, "other-tenant", e.oidc.ID, auth.UpdateIdpParams{Name: &name}); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if _, err := e.idps.SetEnabled(ctx, "other-tenant", e.oidc.ID, false); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("enable: %v", err)
	}
	if err := e.idps.Delete(ctx, "other-tenant", e.oidc.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if got, _ := e.idps.GetInTenant(ctx, e.tenant.ID, e.oidc.ID); got == nil || !got.Enabled || got.Name != "Okta" {
		t.Errorf("provider changed: %+v", got)
	}
}

func TestDeleteRefusesAProviderWithUsers(t *testing.T) {
	e := newMenv(t, jit)
	ctx := context.Background()
	if _, err := e.login(e.handoff("sub-1", "alice@acme.com")); err != nil {
		t.Fatal(err)
	}
	if err := e.idps.Delete(ctx, e.tenant.ID, e.oidc.ID); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	counts, _ := e.idps.ListWithCounts(ctx, e.tenant.ID)
	if len(counts) != 2 || counts[0].ID != e.local.ID || counts[1].UserCount != 1 {
		t.Errorf("counts = %+v", counts)
	}
}
