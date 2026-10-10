package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"platrium/internal/secrets"
)

const testAppURL = "https://files.example.com"

func testSealer() *secrets.Sealer { return secrets.New("test-key").Sealer(secrets.PurposeAuthFlow) }

func newTestFlow(t *testing.T) (*fakeIdP, *Client, *Config) {
	t.Helper()
	idp := newFakeIdP(t)
	ep, err := NewEndpoints(testAppURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		IdpID: "idp_1", TenantID: "t1",
		Issuer: idp.issuer(), ClientID: idp.clientID, ClientSecret: idp.clientSecret,
		EmailClaim: "email", NameClaim: "name", PictureClaim: "picture",
		RequireEmailVerified: true,
	}
	return idp, NewClient(ep, testSealer()), cfg
}

// start begins a sign-in and plays the user approving it at the provider. It
// returns what the callback carries (code, state) and what the browser's cookie
// carries (sealed).
func start(t *testing.T, idp *fakeIdP, c *Client, cfg *Config, returnTo string) (code, state, sealed string) {
	t.Helper()
	st, err := c.Begin(context.Background(), cfg, returnTo)
	if err != nil {
		t.Fatal(err)
	}
	code, state = idp.authorize(st.AuthURL)
	if state != st.State {
		t.Fatalf("state = %q, Started.State = %q", state, st.State)
	}
	return code, state, st.Sealed
}

// signIn runs a full successful round trip and returns the result.
func signIn(t *testing.T, idp *fakeIdP, c *Client, cfg *Config, returnTo string) (*Result, error) {
	t.Helper()
	code, state, sealed := start(t, idp, c, cfg, returnTo)
	return c.Complete(context.Background(), cfg, state, code, "", sealed)
}

func TestFlowHappyPath(t *testing.T) {
	idp, c, cfg := newTestFlow(t)

	res, err := signIn(t, idp, c, cfg, "/drives/123")
	if err != nil {
		t.Fatal(err)
	}
	h := res.Handoff
	if h.IdpProviderID != "idp_1" || h.SubjectID != "user-1" || h.Email != "alice@acme.com" || !h.EmailVerified ||
		h.DisplayName != "Alice" || h.AvatarURL != "https://img.example/alice.png" {
		t.Errorf("handoff = %+v", h)
	}
	if res.ReturnTo != "/drives/123" {
		t.Errorf("ReturnTo = %q", res.ReturnTo)
	}
}

func TestBeginCarriesRegisteredRedirectScopesAndExtras(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	cfg.Scopes = []string{"groups"}
	cfg.ExtraAuthParams = map[string]string{"audience": "https://api.example"}

	st, err := c.Begin(context.Background(), cfg, "/")
	if err != nil {
		t.Fatal(err)
	}
	q := mustQuery(t, st.AuthURL)
	if got, want := q.Get("redirect_uri"), testAppURL+"/api/auth/oidc/idp_1/callback"; got != want {
		t.Errorf("redirect_uri = %q, want %q", got, want)
	}
	if q.Get("scope") != "openid groups" {
		t.Errorf("scope = %q", q.Get("scope"))
	}
	if q.Get("audience") != "https://api.example" {
		t.Errorf("audience = %q", q.Get("audience"))
	}
	if !strings.HasPrefix(st.AuthURL, idp.issuer()+"/authorize?") {
		t.Errorf("url = %s", st.AuthURL)
	}
}

func TestExtraParamsCanNotOverrideTheFlow(t *testing.T) {
	_, c, cfg := newTestFlow(t)
	cfg.ExtraAuthParams = map[string]string{
		"redirect_uri": "https://evil.example/cb", "state": "fixed", "nonce": "fixed",
		"code_challenge": "fixed", "scope": "admin", "audience": "https://api.example",
	}
	st, err := c.Begin(context.Background(), cfg, "/")
	if err != nil {
		t.Fatal(err)
	}
	q := mustQuery(t, st.AuthURL)
	if q.Get("redirect_uri") != testAppURL+"/api/auth/oidc/idp_1/callback" || q.Get("state") == "fixed" ||
		q.Get("nonce") == "fixed" || q.Get("code_challenge") == "fixed" || q.Get("scope") != "openid" {
		t.Errorf("flow parameters were overridden: %s", st.AuthURL)
	}
	if q.Get("audience") != "https://api.example" {
		t.Errorf("audience = %q", q.Get("audience"))
	}
}

// The state names the sign-in in the URL but proves nothing; what is sealed is
// invisible to anyone but this server.
func TestSealedStateRevealsNothing(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	_, _, sealed := start(t, idp, c, cfg, "/secret/path")
	for _, leak := range []string{"secret/path", cfg.IdpID} {
		if strings.Contains(sealed, leak) {
			t.Errorf("the sealed cookie contains %q", leak)
		}
	}
	if len(sealed) > 1000 {
		t.Errorf("the cookie is %d bytes, too close to the 4 KB limit", len(sealed))
	}
}

// The provider's code is single use, which is what stops a replay: the server
// holds nothing to burn.
func TestReplayedCallbackIsRefusedByTheProvider(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	code, state, sealed := start(t, idp, c, cfg, "/")
	if _, err := c.Complete(context.Background(), cfg, state, code, "", sealed); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Complete(context.Background(), cfg, state, code, "", sealed); err == nil {
		t.Fatal("a code was accepted twice")
	}
}

func TestFlowRefusals(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	code, state, sealed := start(t, idp, c, cfg, "/")
	_, otherState, otherSealed := start(t, idp, c, cfg, "/")
	forged := sealed[:len(sealed)-4] + "AAAA"

	foreign := *cfg
	foreign.IdpID = "idp_other"

	for name, tc := range map[string]struct {
		cfg                 *Config
		state, code, sealed string
		wantInError         string
	}{
		"no state":              {cfg, "", code, sealed, "no state"},
		"no cookie":             {cfg, state, code, "", "no sign-in cookie"},
		"another sign-in's":     {cfg, state, code, otherSealed, "does not match"},
		"altered cookie":        {cfg, state, code, forged, "does not match"},
		"state of another flow": {cfg, otherState, code, sealed, "does not match"},
		"a different provider":  {&foreign, state, code, sealed, "different provider"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.Complete(context.Background(), tc.cfg, tc.state, tc.code, "", tc.sealed)
			if !errors.Is(err, ErrFlowInvalid) || !strings.Contains(err.Error(), tc.wantInError) {
				t.Fatalf("err = %v, want ErrFlowInvalid mentioning %q", err, tc.wantInError)
			}
		})
	}
}

func TestSealedStateFromAnotherKeyIsRefused(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	code, state, sealed := start(t, idp, c, cfg, "/")

	otherKey := NewClient(c.endpoints, secrets.New("another-key").Sealer(secrets.PurposeAuthFlow))
	if _, err := otherKey.Complete(context.Background(), cfg, state, code, "", sealed); !errors.Is(err, ErrFlowInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestExpiredFlow(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	code, state, sealed := start(t, idp, c, cfg, "/")

	c.now = func() time.Time { return time.Now().Add(flowTTL + time.Minute) }
	_, err := c.Complete(context.Background(), cfg, state, code, "", sealed)
	if !errors.Is(err, ErrFlowInvalid) || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("err = %v", err)
	}
}

// Two sign-ins open at once finish in either order: each carries its own state.
func TestSignInsDoNotInterfere(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	type flow struct{ code, state, sealed string }
	var flows [2]flow
	for i := range flows {
		code, state, sealed := start(t, idp, c, cfg, fmt.Sprintf("/tab%d", i))
		flows[i] = flow{code, state, sealed}
	}
	for _, i := range []int{1, 0} {
		f := flows[i]
		res, err := c.Complete(context.Background(), cfg, f.state, f.code, "", f.sealed)
		if err != nil || res.ReturnTo != fmt.Sprintf("/tab%d", i) {
			t.Fatalf("sign-in %d: res = %+v, err = %v", i, res, err)
		}
	}
}

func TestProviderError(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	_, state, sealed := start(t, idp, c, cfg, "/")
	if _, err := c.Complete(context.Background(), cfg, state, "", "access_denied", sealed); !errors.Is(err, ErrProviderDenied) {
		t.Fatalf("err = %v, want ErrProviderDenied", err)
	}
}

func TestIDTokenIsVerified(t *testing.T) {
	cases := map[string]func(cl map[string]any){
		"wrong nonce":    func(cl map[string]any) { cl["nonce"] = "someone-elses" },
		"wrong audience": func(cl map[string]any) { cl["aud"] = "another-client" },
		"expired":        func(cl map[string]any) { cl["exp"] = time.Now().Add(-time.Hour).Unix() },
		"wrong issuer":   func(cl map[string]any) { cl["iss"] = "https://evil.example" },
	}
	for name, tweak := range cases {
		t.Run(name, func(t *testing.T) {
			idp, c, cfg := newTestFlow(t)
			idp.tweak = tweak
			if _, err := signIn(t, idp, c, cfg, "/"); !errors.Is(err, ErrRejected) {
				t.Fatalf("err = %v, want ErrRejected", err)
			}
		})
	}
}

func TestBadClientSecretFailsExchange(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	cfg.ClientSecret = "wrong"
	if _, err := signIn(t, idp, c, cfg, "/"); err == nil {
		t.Fatal("exchange with a wrong client secret succeeded")
	}
}

func TestEmailRules(t *testing.T) {
	t.Run("unverified is rejected when required", func(t *testing.T) {
		idp, c, cfg := newTestFlow(t)
		idp.tweak = func(cl map[string]any) { cl["email_verified"] = false }
		if _, err := signIn(t, idp, c, cfg, "/"); !errors.Is(err, ErrRejected) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unverified is accepted when not required", func(t *testing.T) {
		idp, c, cfg := newTestFlow(t)
		cfg.RequireEmailVerified = false
		idp.tweak = func(cl map[string]any) { cl["email_verified"] = false }
		res, err := signIn(t, idp, c, cfg, "/")
		if err != nil || res.Handoff.EmailVerified {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})
	t.Run("verified as a string", func(t *testing.T) {
		idp, c, cfg := newTestFlow(t)
		idp.tweak = func(cl map[string]any) { cl["email_verified"] = "true" }
		if _, err := signIn(t, idp, c, cfg, "/"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("no email is rejected", func(t *testing.T) {
		idp, c, cfg := newTestFlow(t)
		idp.tweak = func(cl map[string]any) { delete(cl, "email") }
		if _, err := signIn(t, idp, c, cfg, "/"); !errors.Is(err, ErrRejected) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestUserinfoFillsMissingClaims(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	idp.tweak = func(cl map[string]any) { delete(cl, "email"); delete(cl, "name") }
	idp.userinfo = map[string]any{"sub": "user-1", "email": "alice@acme.com", "name": "Alice From Userinfo"}

	res, err := signIn(t, idp, c, cfg, "/")
	if err != nil {
		t.Fatal(err)
	}
	if res.Handoff.Email != "alice@acme.com" {
		t.Errorf("email = %q", res.Handoff.Email)
	}
}

func TestUserinfoForAnotherSubjectIsIgnored(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	idp.tweak = func(cl map[string]any) { delete(cl, "email") }
	idp.userinfo = map[string]any{"sub": "someone-else", "email": "mallory@evil.com"}

	if _, err := signIn(t, idp, c, cfg, "/"); !errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected (no email)", err)
	}
}

func TestCustomClaimMapping(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	cfg.EmailClaim, cfg.NameClaim = "mail", "preferred_name"
	idp.tweak = func(cl map[string]any) {
		cl["mail"], cl["preferred_name"] = "al@acme.com", "Al"
	}
	res, err := signIn(t, idp, c, cfg, "/")
	if err != nil {
		t.Fatal(err)
	}
	h := res.Handoff
	if h.Email != "al@acme.com" || h.DisplayName != "Al" {
		t.Errorf("handoff = %+v", h)
	}
}

func TestDisplayNameFallsBackToEmail(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	idp.tweak = func(cl map[string]any) { delete(cl, "name") }
	res, err := signIn(t, idp, c, cfg, "/")
	if err != nil || res.Handoff.DisplayName != "alice@acme.com" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

func TestDiscoveryRejectsADifferentIssuer(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	cfg.Issuer = idp.issuer() + "/tenant-b" // not the one the provider reports
	if _, err := c.Begin(context.Background(), cfg, "/"); err == nil {
		t.Fatal("discovery accepted an issuer the provider does not report")
	}
}

// Providers disagree about a trailing slash (Auth0 reports one), and people
// leave it off. Either way works, and tokens are still held to exactly what the
// provider reports.
func TestTrailingSlashOnTheIssuerDoesNotMatter(t *testing.T) {
	for name, tc := range map[string]struct{ reported, configured string }{
		"provider reports a slash, configured without": {"/", ""},
		"provider reports none, configured with one":   {"", "/"},
		"both with a slash":                            {"/", "/"},
	} {
		t.Run(name, func(t *testing.T) {
			idp, c, cfg := newTestFlow(t)
			idp.reportedSuffix = tc.reported
			cfg.Issuer = idp.issuer() + tc.configured
			if _, err := signIn(t, idp, c, cfg, "/"); err != nil {
				t.Fatal(err)
			}
			if _, err := c.Discover(context.Background(), cfg.Issuer); err != nil {
				t.Fatalf("Discover: %v", err)
			}
		})
	}
}

func TestIDTokenIssuerMustStillMatchWhatTheProviderReports(t *testing.T) {
	idp, c, cfg := newTestFlow(t)
	idp.reportedSuffix = "/"
	cfg.Issuer = idp.issuer() // configured without the slash
	// The provider reports a slash in discovery, but this token drops it.
	idp.tweak = func(cl map[string]any) { cl["iss"] = idp.issuer() }
	if _, err := signIn(t, idp, c, cfg, "/"); !errors.Is(err, ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
}

func TestSafeReturnTo(t *testing.T) {
	for in, want := range map[string]string{
		"":                                       "/",
		"/home":                                  "/home",
		"/folder/1?x=2#y":                        "/folder/1?x=2#y",
		"//evil.com":                             "/",
		"https://evil.com":                       "/",
		"/\\evil.com":                            "/",
		"evil.com":                               "/",
		"/ok\r\nSet-Cookie: a=b":                 "/",
		"/" + strings.Repeat("a", maxReturnTo):   "/", // it rides in a cookie
		"/" + strings.Repeat("a", maxReturnTo-1): "/" + strings.Repeat("a", maxReturnTo-1),
	} {
		if got := SafeReturnTo(in); got != want {
			t.Errorf("SafeReturnTo(%.40q) = %.40q, want %.40q", in, got, want)
		}
	}
}

func TestEndpoints(t *testing.T) {
	ep, err := NewEndpoints(" https://files.example.com/ ")
	if err != nil {
		t.Fatal(err)
	}
	got := ep.Callback("idp 1")
	if got != "https://files.example.com/api/auth/oidc/idp%201/callback" {
		t.Errorf("Callback = %q", got)
	}
	if !ep.Secure() {
		t.Error("an https installation must be Secure")
	}
	if plain, _ := NewEndpoints("http://localhost:3000"); plain.Secure() {
		t.Error("http is not Secure")
	}
	for _, bad := range []string{"", "ftp://x", "files.example.com", "https://x?y=1"} {
		if _, err := NewEndpoints(bad); err == nil {
			t.Errorf("NewEndpoints(%q) accepted", bad)
		}
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
