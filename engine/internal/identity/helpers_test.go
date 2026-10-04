package identity_test

import (
	"context"
	"testing"

	"platrium/internal/auth"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

type env struct {
	db      *db.DB
	tenants *identity.TenantStore
	users   *identity.UserStore
	idps    *auth.IdpStore
	domains *identity.DomainStore
}

func newEnv(t *testing.T) *env {
	d := dbtest.New(t)
	return &env{
		db:      d,
		tenants: identity.NewTenantStore(d),
		users:   identity.NewUserStore(d),
		idps:    auth.NewIdpStore(d),
		domains: identity.NewDomainStore(d),
	}
}

// tenantWithIdp creates a tenant and its local IdP.
func (e *env) tenantWithIdp(t *testing.T, alias string, native bool) (*identity.Tenant, *auth.IdpProvider) {
	t.Helper()
	ctx := context.Background()
	var (
		tn  *identity.Tenant
		idp *auth.IdpProvider
	)
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if tn, err = e.tenants.CreateTenantTx(ctx, tx, identity.CreateTenantParams{Alias: alias, Name: alias, IsNative: native}); err != nil {
			return err
		}
		idp, err = e.idps.CreateLocalIdpTx(ctx, tx, tn.ID, "")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tn, idp
}
