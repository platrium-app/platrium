package restapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"platrium/internal/auth/actor"
	"platrium/internal/auth/session"
	"platrium/internal/auth/token"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/kvstore"
	"platrium/internal/restapi"
)

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

type env struct {
	t        *testing.T
	srv      *httptest.Server
	db       *db.DB
	tokens   *token.Store
	tenantID string
	userID   string
}

// newEnv serves the real REST handlers behind the real session and bearer middleware.
func newEnv(t *testing.T) *env {
	t.Helper()
	t.Setenv("PLATRIUM_SECRET_KEY", "test-secret-key")
	ctx := context.Background()
	d := dbtest.New(t)
	kv, err := kvstore.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kv.Close() })

	devices := identity.NewEntDeviceStore(d)
	tokens := token.NewStore(d, devices, 0)
	sm := session.NewManager()
	users := identity.NewUserStore(d)
	api := restapi.NewRestAPI(nil, actor.NewResolver(sqlauthz.New(d), users), nil, nil, nil, nil, users, nil, sm, tokens, token.NewCodeStore(kv), devices)

	e := &env{t: t, db: d, tokens: tokens}
	err = d.WithTx(ctx, func(tx *ent.Tx) error {
		tn, err := tx.Tenant.Create().SetAlias("acme").SetName("acme").Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("l").Save(ctx)
		if err != nil {
			return err
		}
		u, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID("u").SetEmail("u@x.com").SetDisplayName("u").Save(ctx)
		e.tenantID, e.userID = tn.ID, u.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Route("/api", func(r chi.Router) {
		r.Use(sm.LoadAndSave)
		r.Use(session.Middleware(sm))
		r.Use(session.Bearer(tokens))
		// Stand-in for the login handler: creates the browser session.
		r.Post("/test-login", func(w http.ResponseWriter, r *http.Request) {
			session.PutSession(sm, r, &session.PlatriumSession{UserID: e.userID, TenantID: e.tenantID, Email: "u@x.com", IssuedAt: time.Now().UTC()})
		})
		restapi.HandlerFromMux(restapi.NewStrictHandler(api, nil), r)
	})
	e.srv = httptest.NewServer(r)
	t.Cleanup(e.srv.Close)
	return e
}

// client is one caller: a cookie jar-less browser or a bearer-token client.
type client struct {
	e      *env
	cookie string
	bearer string
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+"/api"+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if sc := resp.Header.Get("Set-Cookie"); sc != "" && path == "/test-login" {
		c.cookie = sc
	}
	return resp.StatusCode, out
}

func (e *env) browser() *client {
	c := &client{e: e}
	c.do("POST", "/test-login", nil)
	return c
}

// authorize runs the browser half of the flow and returns the code.
func (e *env) authorize(b *client, extra map[string]any) string {
	e.t.Helper()
	body := map[string]any{
		"redirect_uri": "platrium://callback", "code_challenge": token.Challenge(verifier),
		"state": "xyz", "name": "Platrium for iPhone",
	}
	for k, v := range extra {
		body[k] = v
	}
	status, out := b.do("POST", "/auth/authorize", body)
	if status != 200 {
		e.t.Fatalf("authorize: %d %v", status, out)
	}
	u, err := url.Parse(out["redirect_to"].(string))
	if err != nil || u.Query().Get("state") != "xyz" {
		e.t.Fatalf("redirect_to = %v", out["redirect_to"])
	}
	return u.Query().Get("code")
}

func TestDeviceFlow(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	code := e.authorize(b, map[string]any{"platform": "ios", "app_version": "1.2"})
	status, tok := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier})
	if status != 200 || tok["device_id"] == nil {
		t.Fatalf("token: %d %v", status, tok)
	}
	secret := tok["token"].(string)

	app := &client{e: e, bearer: secret}
	status, me := app.do("GET", "/auth/me", nil)
	if status != 200 || me["auth_kind"] != "DEVICE" || me["email"] != "u@x.com" || me["device_id"] != tok["device_id"] {
		t.Fatalf("me: %d %v", status, me)
	}

	// The code is single-use.
	if status, _ := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier}); status != 400 {
		t.Fatalf("code reuse: %d", status)
	}

	// Push registration.
	if status, _ := app.do("PUT", "/auth/device/push", map[string]any{"transport": "APNS", "token": "abc"}); status != 204 {
		t.Fatalf("set push: %d", status)
	}
	if status, _ := app.do("PUT", "/auth/device/push", map[string]any{"transport": "SMS", "token": "abc"}); status == 204 {
		t.Fatal("accepted unknown transport")
	}
	// Listed as a device, marked current from its own point of view.
	status, list := app.do("GET", "/auth/clients", nil)
	clients := list["clients"].([]any)
	if status != 200 || len(clients) != 1 {
		t.Fatalf("list: %d %v", status, list)
	}
	c := clients[0].(map[string]any)
	if c["kind"] != "DEVICE" || c["platform"] != "IOS" || c["current"] != true || c["app_version"] != "1.2" {
		t.Fatalf("client = %v", c)
	}

	// Logout revokes the token and its device.
	if status, _ := app.do("POST", "/auth/logout", nil); status != 204 {
		t.Fatalf("logout: %d", status)
	}
	if status, _ := app.do("GET", "/auth/me", nil); status != 401 {
		t.Fatalf("token survived logout: %d", status)
	}
	_, list = b.do("GET", "/auth/clients", nil)
	if n := len(list["clients"].([]any)); n != 0 {
		t.Fatalf("device survived logout: %d", n)
	}
}

func TestAppFlowAndRevokeFromBrowser(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	code := e.authorize(b, map[string]any{"redirect_uri": "http://127.0.0.1:41234/cb", "name": "Platrium CLI"})
	status, tok := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier})
	if status != 200 || tok["device_id"] != nil {
		t.Fatalf("token: %d %v", status, tok)
	}
	cli := &client{e: e, bearer: tok["token"].(string)}
	if _, me := cli.do("GET", "/auth/me", nil); me["auth_kind"] != "APP" {
		t.Fatalf("me = %v", me)
	}
	// Apps have no device, so no push.
	if status, _ := cli.do("PUT", "/auth/device/push", map[string]any{"transport": "FCM", "token": "t"}); status != 403 {
		t.Fatalf("app push: %d", status)
	}

	// The user revokes it from the browser's Apps page.
	if status, _ := b.do("DELETE", "/auth/clients/"+tok["id"].(string), nil); status != 204 {
		t.Fatalf("revoke: %d", status)
	}
	if status, _ := cli.do("GET", "/auth/me", nil); status != 401 {
		t.Fatalf("revoked token still works: %d", status)
	}
	if status, _ := b.do("DELETE", "/auth/clients/"+tok["id"].(string), nil); status != 404 {
		t.Fatalf("second revoke: %d", status)
	}
}

func TestAuthorizeValidation(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	good := map[string]any{"redirect_uri": "platrium://callback", "code_challenge": token.Challenge(verifier), "name": "x"}
	with := func(k string, v any) map[string]any {
		m := map[string]any{}
		for kk, vv := range good {
			m[kk] = vv
		}
		m[k] = v
		return m
	}
	for name, body := range map[string]map[string]any{
		"web redirect":    with("redirect_uri", "https://evil.example/cb"),
		"plain http host": with("redirect_uri", "http://evil.example/cb"),
		"js redirect":     with("redirect_uri", "javascript:alert(1)"),
		"short challenge": with("code_challenge", "abc"),
		"blank name":      with("name", "  "),
		"bad platform":    with("platform", "ios; drop"),
	} {
		if status, _ := b.do("POST", "/auth/authorize", body); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
	if status, _ := (&client{e: e}).do("POST", "/auth/authorize", good); status != 401 {
		t.Errorf("anonymous authorize: %d", status)
	}
}

func TestTokenCannotMintTokens(t *testing.T) {
	e := newEnv(t)
	iss, err := e.tokens.Issue(context.Background(), token.IssueReq{TenantID: e.tenantID, UserID: e.userID, Name: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	cli := &client{e: e, bearer: iss.Secret}
	body := map[string]any{"redirect_uri": "platrium://callback", "code_challenge": token.Challenge(verifier), "name": "x"}
	if status, _ := cli.do("POST", "/auth/authorize", body); status != 403 {
		t.Errorf("authorize via token: %d", status)
	}
	if status, _ := cli.do("POST", "/auth/clients", map[string]any{"name": "x"}); status != 403 {
		t.Errorf("create via token: %d", status)
	}
}

func TestWrongVerifier(t *testing.T) {
	e := newEnv(t)
	code := e.authorize(e.browser(), nil)
	status, _ := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": "wrongwrongwrongwrongwrongwrongwrongwrongwrong"})
	if status != 400 {
		t.Fatalf("wrong verifier: %d", status)
	}
}

func TestCreateAppTokenAndBrowserLogout(t *testing.T) {
	e := newEnv(t)
	b := e.browser()

	status, tok := b.do("POST", "/auth/clients", map[string]any{"name": "CI", "expires_in_days": 30})
	if status != 200 || tok["expires_at"] == nil {
		t.Fatalf("create: %d %v", status, tok)
	}
	if status, _ := b.do("POST", "/auth/clients", map[string]any{"name": "CI", "expires_in_days": 0}); status != 400 {
		t.Fatalf("zero days: %d", status)
	}
	ci := &client{e: e, bearer: tok["token"].(string)}
	if status, _ := ci.do("GET", "/auth/me", nil); status != 200 {
		t.Fatalf("CI token: %d", status)
	}

	// Browser logout destroys the cookie session but leaves tokens alone.
	if status, _ := b.do("POST", "/auth/logout", nil); status != 204 {
		t.Fatalf("logout: %d", status)
	}
	if status, _ := b.do("GET", "/auth/me", nil); status != 401 {
		t.Fatalf("session survived logout: %d", status)
	}
	if status, _ := ci.do("GET", "/auth/me", nil); status != 200 {
		t.Fatalf("browser logout killed the CI token: %d", status)
	}
}

func TestDisabledUserLosesBrowserAndTokenAccess(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	b := e.browser()

	code := e.authorize(b, map[string]any{"platform": "ios", "app_version": "1.2"})
	_, tok := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier})
	app := &client{e: e, bearer: tok["token"].(string)}

	for name, c := range map[string]*client{"browser": b, "device": app} {
		if status, _ := c.do("GET", "/auth/me", nil); status != 200 {
			t.Fatalf("%s must be signed in before the user is disabled: %d", name, status)
		}
	}
	if _, err := identity.NewUserStore(e.db).SetDisabled(ctx, e.tenantID, e.userID, true); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]*client{"browser": b, "device": app} {
		if status, _ := c.do("GET", "/auth/me", nil); status != 401 {
			t.Errorf("disabled user's %s must be rejected: %d", name, status)
		}
	}

	if _, err := identity.NewUserStore(e.db).SetDisabled(ctx, e.tenantID, e.userID, false); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]*client{"browser": b, "device": app} {
		if status, _ := c.do("GET", "/auth/me", nil); status != 200 {
			t.Errorf("re-enabled user's %s must work again: %d", name, status)
		}
	}
}

// Signing a user out everywhere voids the cookies and tokens issued before it,
// including their ability to mint new tokens, and leaves later sign-ins alone.
func TestRevokedSessionsAreDeadEverywhere(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	b := e.browser()
	code := e.authorize(b, map[string]any{"platform": "ios", "app_version": "1.2"})
	_, tok := (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier})
	app := &client{e: e, bearer: tok["token"].(string)}

	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		return identity.NewUserStore(e.db).RevokeSessionsTx(ctx, tx, e.tenantID, e.userID)
	}); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]*client{"browser": b, "device": app} {
		if status, _ := c.do("GET", "/auth/me", nil); status != 401 {
			t.Errorf("revoked %s must be rejected: %d", name, status)
		}
	}
	// The old cookie must not be able to use the client endpoints either: it
	// could otherwise list devices or mint a fresh token that outlives the reset.
	if status, _ := b.do("GET", "/auth/clients", nil); status != 401 {
		t.Errorf("revoked cookie listing clients: %d", status)
	}
	body := map[string]any{"redirect_uri": "platrium://callback", "code_challenge": token.Challenge(verifier), "name": "x"}
	if status, _ := b.do("POST", "/auth/authorize", body); status != 401 {
		t.Errorf("revoked cookie minting a token: %d", status)
	}

	// Signing in again works, and so does a token issued after the revocation.
	b2 := e.browser()
	if status, _ := b2.do("GET", "/auth/me", nil); status != 200 {
		t.Errorf("a new sign-in must work: %d", status)
	}
	code = e.authorize(b2, map[string]any{"platform": "ios", "app_version": "1.2"})
	_, tok = (&client{e: e}).do("POST", "/auth/token", map[string]any{"code": code, "code_verifier": verifier})
	if status, _ := (&client{e: e, bearer: tok["token"].(string)}).do("GET", "/auth/me", nil); status != 200 {
		t.Errorf("a new token must work: %d", status)
	}
	if status, _ := app.do("GET", "/auth/me", nil); status != 401 {
		t.Errorf("the old token stays dead: %d", status)
	}
}

// A disabled user's old cookie cannot reach the client endpoints.
func TestDisabledCookieCannotUseClientEndpoints(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	b := e.browser()
	if _, err := identity.NewUserStore(e.db).SetDisabled(ctx, e.tenantID, e.userID, true); err != nil {
		t.Fatal(err)
	}
	if status, _ := b.do("GET", "/auth/clients", nil); status != 401 {
		t.Errorf("disabled cookie listing clients: %d", status)
	}
	body := map[string]any{"redirect_uri": "platrium://callback", "code_challenge": token.Challenge(verifier), "name": "x"}
	if status, _ := b.do("POST", "/auth/authorize", body); status != 401 {
		t.Errorf("disabled cookie minting a token: %d", status)
	}
}
