package oidc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"platrium/internal/apperr"
	"platrium/internal/auth"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/secrets"
)

type storeEnv struct {
	db     *db.DB
	store  *Store
	idps   *auth.IdpStore
	tenant *identity.Tenant
}

func newStoreEnv(t *testing.T) *storeEnv {
	t.Helper()
	d := dbtest.New(t)
	idps := auth.NewIdpStore(d)
	e := &storeEnv{db: d, idps: idps, store: NewStore(d, idps, secrets.New("test-key"))}
	err := d.WithTx(context.Background(), func(tx *ent.Tx) error {
		var err error
		e.tenant, err = identity.NewTenantStore(d).CreateTenantTx(context.Background(), tx, identity.CreateTenantParams{Alias: "acme", Name: "Acme"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *storeEnv) params() CreateParams {
	return CreateParams{
		TenantID: e.tenant.ID,
		Name:     "Acme SSO",
		ProvisioningPolicy: auth.ProvisioningPolicy{
			JITUsers: true, DefaultRole: "MEMBER", AllowedEmailDomains: []string{"Acme.com"},
		},
		Config: Config{
			Issuer: "https://acme.auth0.com/", ClientID: "cid", ClientSecret: "sh-its-a-secret",
			Scopes: []string{"openid", "groups", "groups"}, RequireEmailVerified: true,
		},
	}
}

func TestCreateAndGet(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()

	idp, cfg, err := e.store.Create(ctx, e.params())
	if err != nil {
		t.Fatal(err)
	}
	if idp.Type != auth.IdpTypeOIDC || !idp.Enabled || !idp.JITUsers || idp.DefaultRole != "MEMBER" {
		t.Errorf("idp = %+v", idp)
	}
	if got := idp.AllowedEmailDomains; len(got) != 1 || got[0] != "acme.com" {
		t.Errorf("domains = %v, want lowercased", got)
	}
	// Optional inputs are filled with the OIDC standards; openid is implied.
	if cfg.EmailClaim != "email" || cfg.NameClaim != "name" || cfg.PictureClaim != "picture" {
		t.Errorf("claims = %+v", cfg)
	}
	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "groups" {
		t.Errorf("scopes = %v", cfg.Scopes)
	}

	got, err := e.store.Get(ctx, idp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientSecret != "sh-its-a-secret" || got.Issuer != "https://acme.auth0.com/" || got.ClientID != "cid" || got.TenantID != e.tenant.ID || got.IdpID != idp.ID {
		t.Errorf("Get = %+v", got)
	}
}

func TestSecretIsSealedAtRest(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()
	idp, _, err := e.store.Create(ctx, e.params())
	if err != nil {
		t.Fatal(err)
	}

	row := e.db.IdpOIDCConfig.Query().OnlyX(ctx)
	if !strings.HasPrefix(row.ClientSecret, "enc:v1:") || strings.Contains(row.ClientSecret, "sh-its-a-secret") {
		t.Fatalf("client_secret stored as %q", row.ClientSecret)
	}

	// Another key can not read it.
	other := NewStore(e.db, e.idps, secrets.New("different-key"))
	if _, err := other.Get(ctx, idp.ID); err == nil {
		t.Error("opened under a different key")
	}

	// A sealed secret copied onto another provider does not open there.
	idp2, _, err := e.store.Create(ctx, e.params())
	if err != nil {
		t.Fatal(err)
	}
	e.db.IdpOIDCConfig.Update().Where().SetClientSecret(row.ClientSecret).ExecX(ctx)
	if _, err := e.store.Get(ctx, idp2.ID); err == nil {
		t.Error("a sealed secret opened on a provider it was not sealed for")
	}
}

func TestCreateValidation(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()

	cases := map[string]func(p *CreateParams){
		"plain http issuer":        func(p *CreateParams) { p.Issuer = "http://idp.acme.com" },
		"issuer with query":        func(p *CreateParams) { p.Issuer = "https://idp.acme.com/?x=1" },
		"not a url":                func(p *CreateParams) { p.Issuer = "idp.acme.com" },
		"no client id":             func(p *CreateParams) { p.ClientID = " " },
		"no client secret":         func(p *CreateParams) { p.ClientSecret = "" },
		"no name":                  func(p *CreateParams) { p.Name = " " },
		"admin default role":       func(p *CreateParams) { p.DefaultRole = "ADMIN" },
		"super admin default role": func(p *CreateParams) { p.DefaultRole = "SUPER_ADMIN" },
		"unknown default role":     func(p *CreateParams) { p.DefaultRole = "WIZARD" },
		"empty default role":       func(p *CreateParams) { p.DefaultRole = "" },
		"bad domain":               func(p *CreateParams) { p.AllowedEmailDomains = []string{"@acme.com"} },
		"bad scope":                func(p *CreateParams) { p.Scopes = []string{"a b"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := e.params()
			mutate(&p)
			if _, _, err := e.store.Create(ctx, p); !errors.Is(err, apperr.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
	if n := e.db.IdpProvider.Query().CountX(ctx); n != 0 {
		t.Errorf("%d providers were left behind by rejected requests", n)
	}
}

func TestLoopbackHTTPIssuerAllowedForDevelopment(t *testing.T) {
	e := newStoreEnv(t)
	p := e.params()
	p.Issuer = "http://localhost:9000/application/o/platrium/"
	if _, _, err := e.store.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func TestCreateIsAtomic(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()
	p := e.params()
	p.TenantID = "no-such-tenant"
	if _, _, err := e.store.Create(ctx, p); err == nil {
		t.Fatal("created a provider in a missing tenant")
	}
	if e.db.IdpProvider.Query().CountX(ctx) != 0 || e.db.IdpOIDCConfig.Query().CountX(ctx) != 0 {
		t.Error("a failed create left rows behind")
	}
}

func TestGetUnknown(t *testing.T) {
	e := newStoreEnv(t)
	if _, err := e.store.Get(context.Background(), "nope"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeletingProviderDeletesItsConfig(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()
	idp, _, err := e.store.Create(ctx, e.params())
	if err != nil {
		t.Fatal(err)
	}
	e.db.IdpProvider.DeleteOneID(idp.ID).ExecX(ctx)
	if n := e.db.IdpOIDCConfig.Query().CountX(ctx); n != 0 {
		t.Fatalf("%d configs outlived their provider", n)
	}
}

func TestOneConfigPerProvider(t *testing.T) {
	e := newStoreEnv(t)
	ctx := context.Background()
	idp, _, err := e.store.Create(ctx, e.params())
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.db.IdpOIDCConfig.Create().SetTenantID(e.tenant.ID).SetIdpID(idp.ID).
		SetIssuer("https://x").SetClientID("c").SetClientSecret("s").Save(ctx)
	if err == nil {
		t.Fatal("a second config for one provider was accepted")
	}
}
