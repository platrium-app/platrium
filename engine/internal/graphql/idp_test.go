package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/oidc"
	"platrium/internal/identity"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/orchestrator"
	"platrium/internal/secrets"
)

const idpFields = `id name type isLocal enabled jitUsers defaultRole allowedEmailDomains userCount
  config { __typename ... on OidcConfig { issuer clientId scopes emailClaim nameClaim pictureClaim requireEmailVerified extraAuthParams { name value } redirectUri } }`

type idpEnv struct {
	*harness
	do    func(userID, query string, vars ...client.Option) (map[string]any, error)
	super string // a SUPER_ADMIN of the tenant, the only role holding idp.manage
	store *oidc.Store
}

func newIdpEnv(t *testing.T) *idpEnv {
	t.Helper()
	h := newHarness(t)
	do := h.adminClient(t)

	idps := auth.NewIdpStore(h.db)
	store := oidc.NewStore(h.db, idps, secrets.New("test-key"))
	ep, err := oidc.NewEndpoints("https://files.example.com")
	if err != nil {
		t.Fatal(err)
	}
	h.r.IdpAdmin = orchestrator.NewIdpAdmin(identity.NewUserStore(h.db), idps, store, oidc.NewClient(ep, nil), ep)

	local := h.db.IdpProvider.Query().Where(idpprovider.TenantID(h.tenant)).OnlyX(context.Background())
	sue := h.db.User.Create().SetTenantID(h.tenant).SetIdpID(local.ID).SetExternalID("sue").
		SetEmail("sue@acme.com").SetDisplayName("sue").SetRole(identity.RoleSuperAdmin).SaveX(context.Background())
	return &idpEnv{harness: h, do: do, super: sue.ID, store: store}
}

func (e *idpEnv) create(t *testing.T, extra string) map[string]any {
	t.Helper()
	resp, err := e.do(e.super, fmt.Sprintf(`mutation { createIdentityProvider(input: {
		name: "Auth0", jitUsers: true, defaultRole: "MEMBER", allowedEmailDomains: ["Acme.com"],
		config: { oidc: { issuer: "https://acme.auth0.com/", clientId: "cid", clientSecret: "shh" %s } } }) { %s } }`, extra, idpFields))
	if err != nil {
		t.Fatal(err)
	}
	return resp["createIdentityProvider"].(map[string]any)
}

func TestIdpAdminIsGatedByPermission(t *testing.T) {
	e := newIdpEnv(t)

	_, err := e.do("", `query { identityProviders { id } }`)
	mustFail(t, "no session", err, "UNAUTHENTICATED")
	for who, id := range map[string]string{"member": e.bob, "admin": e.admin} {
		_, err = e.do(id, `query { identityProviders { id } }`)
		mustFail(t, who+" listing", err, "FORBIDDEN")
		_, err = e.do(id, `mutation { createIdentityProvider(input: {name: "X", jitUsers: false, defaultRole: "MEMBER", config: {oidc: {issuer: "https://x.example", clientId: "c", clientSecret: "s"}}}) { id } }`)
		mustFail(t, who+" creating", err, "FORBIDDEN")
		_, err = e.do(id, `mutation { testOidcDiscovery(issuer: "https://x.example") { ok } }`)
		mustFail(t, who+" testing", err, "FORBIDDEN")
	}
	if n := e.db.IdpProvider.Query().CountX(context.Background()); n != 1 {
		t.Errorf("providers = %d, a refused request must change nothing", n)
	}

	resp, err := e.do(e.super, `query { me { permissions } }`)
	if err != nil || !strings.Contains(fmt.Sprint(resp), "IDP_MANAGE") {
		t.Errorf("super admin lacks IDP_MANAGE: %v %v", resp, err)
	}
}

func TestCreateAndReadOIDCProvider(t *testing.T) {
	e := newIdpEnv(t)
	p := e.create(t, "")

	cfg := p["config"].(map[string]any)
	if p["type"] != "OIDC" || p["isLocal"] != false || p["enabled"] != true || p["jitUsers"] != true || p["defaultRole"] != "MEMBER" || p["userCount"].(float64) != 0 {
		t.Errorf("provider = %v", p)
	}
	if d := p["allowedEmailDomains"].([]any); len(d) != 1 || d[0] != "acme.com" {
		t.Errorf("domains = %v", d)
	}
	if cfg["__typename"] != "OidcConfig" || cfg["issuer"] != "https://acme.auth0.com/" || cfg["clientId"] != "cid" ||
		cfg["emailClaim"] != "email" || cfg["nameClaim"] != "name" || cfg["pictureClaim"] != "picture" || cfg["requireEmailVerified"] != true {
		t.Errorf("config = %v", cfg)
	}
	if want := "https://files.example.com/api/auth/oidc/" + p["id"].(string) + "/callback"; cfg["redirectUri"] != want {
		t.Errorf("redirectUri = %v, want %s", cfg["redirectUri"], want)
	}

	// The secret is stored sealed and has no field to be read back through.
	row := e.db.IdpOIDCConfig.Query().OnlyX(context.Background())
	if !strings.HasPrefix(row.ClientSecret, "enc:v1:") || strings.Contains(row.ClientSecret, "shh") {
		t.Errorf("stored secret = %q", row.ClientSecret)
	}
	_, err := e.do(e.super, `query { identityProviders { config { ... on OidcConfig { clientSecret } } } }`)
	if err == nil {
		t.Error("the client secret is readable through the API")
	}
}

func TestListShowsBuiltInFirstWithUserCounts(t *testing.T) {
	e := newIdpEnv(t)
	e.create(t, "")
	resp, err := e.do(e.super, `query { identityProviders { `+idpFields+` } }`)
	if err != nil {
		t.Fatal(err)
	}
	list := resp["identityProviders"].([]any)
	if len(list) != 2 {
		t.Fatalf("providers = %d", len(list))
	}
	local := list[0].(map[string]any)
	if local["isLocal"] != true || local["type"] != "LOCAL" || local["config"] != nil || local["userCount"].(float64) != 5 {
		t.Errorf("built-in = %v", local)
	}
}

func TestConfigInputMustNameExactlyOneProtocol(t *testing.T) {
	e := newIdpEnv(t)
	for name, cfg := range map[string]string{"empty": `{}`, "null member": `{oidc: null}`} {
		_, err := e.do(e.super, `mutation { createIdentityProvider(input: {name: "X", jitUsers: false, defaultRole: "MEMBER", config: `+cfg+`}) { id } }`)
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if n := e.db.IdpProvider.Query().CountX(context.Background()); n != 1 {
		t.Errorf("providers = %d", n)
	}
}

func TestCreateRejectsUnsafeOrInvalidInput(t *testing.T) {
	e := newIdpEnv(t)
	mut := func(role, issuer string) string {
		return fmt.Sprintf(`mutation { createIdentityProvider(input: {name: "X", jitUsers: true, defaultRole: %q,
			config: {oidc: {issuer: %q, clientId: "c", clientSecret: "s"}}}) { id } }`, role, issuer)
	}
	for name, q := range map[string]string{
		"admin default role":       mut("ADMIN", "https://idp.example"),
		"super admin default role": mut("SUPER_ADMIN", "https://idp.example"),
		"unknown role":             mut("WIZARD", "https://idp.example"),
		"plain http issuer":        mut("MEMBER", "http://idp.example"),
		"not a url":                mut("MEMBER", "idp.example"),
	} {
		_, err := e.do(e.super, q)
		mustFail(t, name, err, "BAD_REQUEST")
	}
	_, err := e.do(e.super, `mutation { createIdentityProvider(input: {name: "X", jitUsers: true, defaultRole: "MEMBER",
		config: {oidc: {issuer: "https://idp.example", clientId: "c", clientSecret: "s", extraAuthParams: [{name: "a", value: "1"}, {name: "a", value: "2"}]}}}) { id } }`)
	mustFail(t, "repeated parameter", err, "BAD_REQUEST")

	if n := e.db.IdpProvider.Query().CountX(context.Background()); n != 1 {
		t.Errorf("providers = %d, rejected requests must change nothing", n)
	}
}

func TestBuiltInProviderIsReadOnlyAndPermanent(t *testing.T) {
	e := newIdpEnv(t)
	ctx := context.Background()
	local := e.db.IdpProvider.Query().OnlyX(ctx)

	for name, q := range map[string]string{
		"rename":  fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {name: "Renamed"}) { id } }`, local.ID),
		"policy":  fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {jitUsers: true}) { id } }`, local.ID),
		"disable": fmt.Sprintf(`mutation { setIdentityProviderEnabled(id: %q, enabled: false) { id } }`, local.ID),
		"enable":  fmt.Sprintf(`mutation { setIdentityProviderEnabled(id: %q, enabled: true) { id } }`, local.ID),
		"delete":  fmt.Sprintf(`mutation { deleteIdentityProvider(id: %q) }`, local.ID),
	} {
		_, err := e.do(e.super, q)
		mustFail(t, name, err, "BAD_REQUEST")
	}
	after := e.db.IdpProvider.Query().OnlyX(ctx)
	if after.Name != local.Name || !after.Enabled || after.JitUsers || after.UpdatedAt != local.UpdatedAt {
		t.Errorf("the built-in provider changed: %+v", after)
	}
}

func TestUpdateCommonSettings(t *testing.T) {
	e := newIdpEnv(t)
	id := e.create(t, "")["id"].(string)

	resp, err := e.do(e.super, fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {name: "Okta", jitUsers: false, allowedEmailDomains: ["Corp.acme.com", "acme.com"]}) { name jitUsers defaultRole allowedEmailDomains } }`, id))
	if err != nil {
		t.Fatal(err)
	}
	got := resp["updateIdentityProvider"].(map[string]any)
	if got["name"] != "Okta" || got["jitUsers"] != false || got["defaultRole"] != "MEMBER" || fmt.Sprint(got["allowedEmailDomains"]) != "[corp.acme.com acme.com]" {
		t.Errorf("updated = %v", got)
	}

	_, err = e.do(e.super, fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {defaultRole: "ADMIN"}) { id } }`, id))
	mustFail(t, "admin default role", err, "BAD_REQUEST")
	_, err = e.do(e.super, fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {allowedEmailDomains: ["@acme.com"]}) { id } }`, id))
	mustFail(t, "bad domain", err, "BAD_REQUEST")
	if row := e.db.IdpProvider.GetX(context.Background(), id); row.DefaultRole != "MEMBER" || row.Name != "Okta" {
		t.Errorf("a rejected update changed the provider: %+v", row)
	}
}

func TestUpdateOIDCConfigKeepsOrRotatesTheSecret(t *testing.T) {
	e := newIdpEnv(t)
	ctx := context.Background()
	id := e.create(t, "")["id"].(string)
	before := e.db.IdpOIDCConfig.Query().OnlyX(ctx).ClientSecret

	resp, err := e.do(e.super, fmt.Sprintf(`mutation { updateOidcConfig(id: %q, input: {clientId: "cid-2", scopes: ["groups"], nameClaim: "nickname", extraAuthParams: [{name: "audience", value: "https://api"}]}) { config { ... on OidcConfig { clientId scopes nameClaim extraAuthParams { name value } issuer } } } }`, id))
	if err != nil {
		t.Fatal(err)
	}
	cfg := resp["updateOidcConfig"].(map[string]any)["config"].(map[string]any)
	if cfg["clientId"] != "cid-2" || cfg["nameClaim"] != "nickname" || cfg["issuer"] != "https://acme.auth0.com/" || fmt.Sprint(cfg["scopes"]) != "[groups]" {
		t.Errorf("config = %v", cfg)
	}
	if e.db.IdpOIDCConfig.Query().OnlyX(ctx).ClientSecret != before {
		t.Error("omitting the secret changed it")
	}
	c, _ := e.store.Get(ctx, id)
	if c.ClientSecret != "shh" {
		t.Errorf("secret = %q, want it kept", c.ClientSecret)
	}

	if _, err := e.do(e.super, fmt.Sprintf(`mutation { updateOidcConfig(id: %q, input: {clientSecret: "rotated"}) { id } }`, id)); err != nil {
		t.Fatal(err)
	}
	if c, _ = e.store.Get(ctx, id); c.ClientSecret != "rotated" {
		t.Errorf("secret = %q, want rotated", c.ClientSecret)
	}
	_, err = e.do(e.super, fmt.Sprintf(`mutation { updateOidcConfig(id: %q, input: {clientSecret: ""}) { id } }`, id))
	mustFail(t, "empty secret", err, "BAD_REQUEST")

	// The issuer is fixed at creation: the input has no way to name one.
	_, err = e.do(e.super, fmt.Sprintf(`mutation { updateOidcConfig(id: %q, input: {issuer: "https://other.example"}) { id } }`, id))
	if err == nil {
		t.Error("the issuer of an existing provider can be changed")
	}
}

func TestDisableAndDeleteGuards(t *testing.T) {
	e := newIdpEnv(t)
	ctx := context.Background()
	id := e.create(t, "")["id"].(string)

	resp, err := e.do(e.super, fmt.Sprintf(`mutation { setIdentityProviderEnabled(id: %q, enabled: false) { enabled } }`, id))
	if err != nil || resp["setIdentityProviderEnabled"].(map[string]any)["enabled"] != false {
		t.Fatalf("disable: %v %v", resp, err)
	}

	// A provider with users is not deleted.
	e.db.User.Create().SetTenantID(e.tenant).SetIdpID(id).SetExternalID("sub-1").SetEmail("x@acme.com").SetDisplayName("X").SaveX(ctx)
	_, err = e.do(e.super, fmt.Sprintf(`mutation { deleteIdentityProvider(id: %q) }`, id))
	mustFail(t, "delete with users", err, "CONFLICT")
	if e.db.IdpProvider.Query().Where(idpprovider.ID(id)).CountX(ctx) != 1 {
		t.Fatal("a provider with users was deleted")
	}
}

func TestDeleteWithoutUsersRemovesConfigToo(t *testing.T) {
	e := newIdpEnv(t)
	ctx := context.Background()
	id := e.create(t, "")["id"].(string)

	resp, err := e.do(e.super, fmt.Sprintf(`mutation { deleteIdentityProvider(id: %q) }`, id))
	if err != nil || resp["deleteIdentityProvider"] != true {
		t.Fatalf("delete: %v %v", resp, err)
	}
	if e.db.IdpProvider.Query().CountX(ctx) != 1 || e.db.IdpOIDCConfig.Query().CountX(ctx) != 0 {
		t.Error("provider or its config survived")
	}
	_, err = e.do(e.super, fmt.Sprintf(`mutation { deleteIdentityProvider(id: %q) }`, id))
	mustFail(t, "delete again", err, "NOT_FOUND")
}

func TestOtherTenantsProvidersAreInvisible(t *testing.T) {
	e := newIdpEnv(t)
	ctx := context.Background()
	other := e.db.Tenant.Create().SetAlias("other").SetName("Other").SaveX(ctx)
	foreign := e.db.IdpProvider.Create().SetTenantID(other.ID).SetType("OIDC").SetName("Theirs").SaveX(ctx)

	for name, q := range map[string]string{
		"read":   fmt.Sprintf(`query { identityProvider(id: %q) { id } }`, foreign.ID),
		"update": fmt.Sprintf(`mutation { updateIdentityProvider(id: %q, input: {name: "Mine now"}) { id } }`, foreign.ID),
		"enable": fmt.Sprintf(`mutation { setIdentityProviderEnabled(id: %q, enabled: false) { id } }`, foreign.ID),
		"delete": fmt.Sprintf(`mutation { deleteIdentityProvider(id: %q) }`, foreign.ID),
		"oidc":   fmt.Sprintf(`mutation { updateOidcConfig(id: %q, input: {clientId: "x"}) { id } }`, foreign.ID),
	} {
		_, err := e.do(e.super, q)
		mustFail(t, name, err, "NOT_FOUND")
	}
	resp, _ := e.do(e.super, `query { identityProviders { name } }`)
	if strings.Contains(fmt.Sprint(resp), "Theirs") {
		t.Error("another tenant's provider is listed")
	}
	if got := e.db.IdpProvider.GetX(ctx, foreign.ID); got.Name != "Theirs" || !got.Enabled {
		t.Errorf("another tenant's provider changed: %+v", got)
	}
}

func TestTestOidcDiscovery(t *testing.T) {
	e := newIdpEnv(t)

	var base string
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"issuer": base, "authorization_endpoint": base + "/auth", "token_endpoint": base + "/token",
			"jwks_uri": base + "/jwks", "scopes_supported": []string{"openid", "email"},
			"code_challenge_methods_supported": []string{"plain"},
		})
	}))
	defer good.Close()
	base = good.URL

	resp, err := e.do(e.super, fmt.Sprintf(`mutation { testOidcDiscovery(issuer: %q) { ok message authorizationEndpoint tokenEndpoint jwksUri scopesSupported supportsPkce } }`, good.URL))
	if err != nil {
		t.Fatal(err)
	}
	r := resp["testOidcDiscovery"].(map[string]any)
	if r["ok"] != true || r["tokenEndpoint"] != good.URL+"/token" || r["authorizationEndpoint"] != good.URL+"/auth" || r["supportsPkce"] != false {
		t.Errorf("result = %v", r)
	}

	// A URL that is not a provider is a result, not an error, and says nothing
	// about what the server saw there.
	secret := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal-token-abc123", http.StatusInternalServerError)
	}))
	defer secret.Close()
	resp, err = e.do(e.super, fmt.Sprintf(`mutation { testOidcDiscovery(issuer: %q) { ok message } }`, secret.URL))
	if err != nil {
		t.Fatal(err)
	}
	r = resp["testOidcDiscovery"].(map[string]any)
	if r["ok"] != false || r["message"] == nil || strings.Contains(fmt.Sprint(r["message"]), "abc123") {
		t.Errorf("result = %v", r)
	}

	_, err = e.do(e.super, `mutation { testOidcDiscovery(issuer: "ftp://idp.example") { ok } }`)
	mustFail(t, "bad scheme", err, "BAD_REQUEST")
}
