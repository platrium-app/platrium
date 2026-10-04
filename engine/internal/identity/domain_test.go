package identity_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/identity"
)

func TestDomains(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	acme, _ := e.tenantWithIdp(t, "acme", false)
	other, _ := e.tenantWithIdp(t, "other", false)

	if err := e.domains.AddDomainToTenant(ctx, acme.ID, "Acme.com"); err != nil {
		t.Fatal(err)
	}
	if err := e.domains.AddDomainToTenant(ctx, other.ID, "acme.com"); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("domain claimed by another tenant must conflict, got %v", err)
	}
	if err := e.domains.RemoveDomainFromTenant(ctx, acme.ID, "acme.com"); err == nil {
		t.Error("removing the only domain must fail")
	}
	if err := e.domains.AddDomainToTenant(ctx, acme.ID, "acme.org"); err != nil {
		t.Fatal(err)
	}
	if err := e.domains.RemoveDomainFromTenant(ctx, acme.ID, "ACME.com"); err != nil {
		t.Fatalf("removing one of two domains: %v", err)
	}
	if err := e.domains.RemoveDomainFromTenant(ctx, acme.ID, "acme.org"); err == nil {
		t.Error("removing the last remaining domain must fail")
	}
}
