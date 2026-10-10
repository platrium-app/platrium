package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"platrium/internal/apperr"
	"platrium/internal/auth"
)

type recordingManager struct {
	got []auth.IdpAuthHandoff
	err error
}

func (m *recordingManager) HandleFederatedLogin(_ context.Context, h auth.IdpAuthHandoff) error {
	m.got = append(m.got, h)
	return m.err
}

type handlerEnv struct {
	idp     *fakeIdP
	store   *storeEnv
	idpID   string
	manager *recordingManager
	server  *httptest.Server
}

func newHandlerEnv(t *testing.T) *handlerEnv {
	t.Helper()
	e := &handlerEnv{idp: newFakeIdP(t), store: newStoreEnv(t), manager: &recordingManager{}}

	p := e.store.params()
	p.Issuer, p.ClientID, p.ClientSecret = e.idp.issuer(), e.idp.clientID, e.idp.clientSecret
	idp, _, err := e.store.store.Create(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	e.idpID = idp.ID

	ep, _ := NewEndpoints(testAppURL)
	h := NewHandler(e.store.idps, e.store.store, NewClient(ep, testSealer()), e.manager)

	r := chi.NewRouter()
	r.Get("/api/auth/start", func(w http.ResponseWriter, r *http.Request) {
		loc, err := h.Begin(w, r, r.URL.Query().Get("idp"), r.URL.Query().Get("return_to"))
		if errors.Is(err, apperr.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, loc, http.StatusFound)
	})
	r.Get("/api/auth/oidc/{idpId}/callback", h.Callback)
	e.server = httptest.NewServer(r)
	t.Cleanup(e.server.Close)
	return e
}

// browser is an HTTP client with its own cookie jar that does not follow redirects.
func (e *handlerEnv) browser() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func (e *handlerEnv) get(t *testing.T, b *http.Client, path string) *http.Response {
	t.Helper()
	resp, err := b.Get(e.server.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// begin starts a sign-in in the browser and returns the provider's code and state.
func (e *handlerEnv) begin(t *testing.T, b *http.Client, returnTo string) (code, state string) {
	t.Helper()
	resp := e.get(t, b, "/api/auth/start?idp="+e.idpID+"&return_to="+url.QueryEscape(returnTo))
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("start: status %d", resp.StatusCode)
	}
	return e.idp.authorize(resp.Header.Get("Location"))
}

func (e *handlerEnv) callback(code, state string) string {
	return "/api/auth/oidc/" + e.idpID + "/callback?" + url.Values{"code": {code}, "state": {state}}.Encode()
}

func TestCallbackSignsTheBrowserIn(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	code, state := e.begin(t, b, "/drives/1")

	resp := e.get(t, b, e.callback(code, state))
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/drives/1" {
		t.Fatalf("status %d, Location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if len(e.manager.got) != 1 || e.manager.got[0].SubjectID != "user-1" || e.manager.got[0].IdpProviderID != e.idpID {
		t.Fatalf("handoffs = %+v", e.manager.got)
	}
}

func TestCallbackFromAnotherBrowserIsRefused(t *testing.T) {
	e := newHandlerEnv(t)
	code, state := e.begin(t, e.browser(), "/")

	// An attacker feeds their own code and state to a victim: the victim's
	// browser never started this sign-in.
	resp := e.get(t, e.browser(), e.callback(code, state))
	if got := resp.Header.Get("Location"); got != "/login?error=sso_expired" {
		t.Fatalf("Location = %q", got)
	}
	if len(e.manager.got) != 0 {
		t.Fatal("a sign-in this browser did not start was accepted")
	}
}

func TestCallbackReplayIsRefused(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	code, state := e.begin(t, b, "/")
	e.get(t, b, e.callback(code, state))

	resp := e.get(t, b, e.callback(code, state))
	if got := resp.Header.Get("Location"); got != "/login?error=sso_expired" {
		t.Fatalf("Location = %q", got)
	}
	if len(e.manager.got) != 1 {
		t.Fatalf("handoffs = %d, want 1", len(e.manager.got))
	}
}

func TestCallbackProviderDenied(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	_, state := e.begin(t, b, "/")
	resp := e.get(t, b, "/api/auth/oidc/"+e.idpID+"/callback?error=access_denied&state="+state)
	if got := resp.Header.Get("Location"); got != "/login?error=sso_denied" {
		t.Fatalf("Location = %q", got)
	}
}

func TestCallbackLoginFailure(t *testing.T) {
	e := newHandlerEnv(t)
	e.manager.err = errors.New("no such user")
	b := e.browser()
	code, state := e.begin(t, b, "/")
	resp := e.get(t, b, e.callback(code, state))
	if got := resp.Header.Get("Location"); got != "/login?error=sso_failed" {
		t.Fatalf("Location = %q", got)
	}
}

func TestUnknownProvider(t *testing.T) {
	e := newHandlerEnv(t)
	if resp := e.get(t, e.browser(), "/api/auth/start?idp=nope"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown provider: status %d", resp.StatusCode)
	}
}

func TestLoginRefusalsReachTheLoginPage(t *testing.T) {
	for want, refusal := range map[string]error{
		"sso_no_account": auth.ErrNotProvisioned,
		"sso_disabled":   auth.ErrUserDisabled,
	} {
		t.Run(want, func(t *testing.T) {
			e := newHandlerEnv(t)
			e.manager.err = fmt.Errorf("wrapped: %w", refusal)
			b := e.browser()
			code, state := e.begin(t, b, "/")
			resp := e.get(t, b, e.callback(code, state))
			if got := resp.Header.Get("Location"); got != "/login?error="+want {
				t.Errorf("%v: Location = %q, want error=%s", refusal, got, want)
			}
		})
	}
}

func TestOpenRedirectIsNeutralized(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	code, state := e.begin(t, b, "https://evil.example/phish")
	resp := e.get(t, b, e.callback(code, state))
	if got := resp.Header.Get("Location"); got != "/" {
		t.Fatalf("Location = %q, want /", got)
	}
}

// The regression behind a spurious "sign-in expired": two sign-ins open in one
// browser (two tabs, a double click) used to overwrite each other.
func TestTwoSignInsInOneBrowserBothComplete(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	codeA, stateA := e.begin(t, b, "/a")
	codeB, stateB := e.begin(t, b, "/b")

	for _, c := range []struct{ code, state, to string }{{codeA, stateA, "/a"}, {codeB, stateB, "/b"}} {
		resp := e.get(t, b, e.callback(c.code, c.state))
		if got := resp.Header.Get("Location"); got != c.to {
			t.Errorf("Location = %q, want %q", got, c.to)
		}
	}
	if len(e.manager.got) != 2 {
		t.Fatalf("handoffs = %d, want 2", len(e.manager.got))
	}
}

func TestConcurrentStartsAllComplete(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	e.get(t, b, "/api/auth/start?idp="+e.idpID) // the browser already has its cookie

	type started struct{ code, state string }
	out := make(chan started, 6)
	for i := 0; i < cap(out); i++ {
		go func() {
			code, state := e.begin(t, b, "/")
			out <- started{code, state}
		}()
	}
	for i := 0; i < cap(out); i++ {
		s := <-out
		if loc := e.get(t, b, e.callback(s.code, s.state)).Header.Get("Location"); loc != "/" {
			t.Errorf("Location = %q", loc)
		}
	}
}

// An attacker starts a sign-in of their own and tricks a victim into finishing
// it. The victim may have a sign-in cookie of their own, but not the attacker's.
func TestAttackersFlowCannotBeFinishedByAVictimWithTheirOwnCookie(t *testing.T) {
	e := newHandlerEnv(t)
	code, state := e.begin(t, e.browser(), "/")
	victim := e.browser()
	e.get(t, victim, "/api/auth/start?idp="+e.idpID) // the victim has a sign-in cookie too

	resp := e.get(t, victim, e.callback(code, state))
	if got := resp.Header.Get("Location"); got != "/login?error=sso_expired" || len(e.manager.got) != 0 {
		t.Fatalf("Location = %q, handoffs = %d", got, len(e.manager.got))
	}
}

func flowCookieOf(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, k := range resp.Cookies() {
		if strings.HasPrefix(k.Name, flowCookiePrefix) {
			return k
		}
	}
	t.Fatalf("no %s* cookie in the response", flowCookiePrefix)
	return nil
}

func TestFlowCookieIsLockedDown(t *testing.T) {
	e := newHandlerEnv(t)
	c := flowCookieOf(t, e.get(t, e.browser(), "/api/auth/start?idp="+e.idpID))
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/api/auth/oidc/" || c.MaxAge != int(flowTTL.Seconds()) {
		t.Errorf("cookie = %+v", c)
	}
	if !strings.HasPrefix(c.Value, "enc:v1:") || len(c.Value) > 1000 {
		t.Errorf("the cookie value is not a sealed, small value: %d bytes", len(c.Value))
	}
}

// Every sign-in gets a cookie of its own, so tabs do not share or overwrite one.
func TestEachSignInHasItsOwnCookie(t *testing.T) {
	e := newHandlerEnv(t)
	b := e.browser()
	a := flowCookieOf(t, e.get(t, b, "/api/auth/start?idp="+e.idpID))
	c := flowCookieOf(t, e.get(t, b, "/api/auth/start?idp="+e.idpID))
	if a.Name == c.Name {
		t.Fatalf("both sign-ins used the cookie %q", a.Name)
	}
}

// The callback uses the cookie up, whether the sign-in worked or not.
func TestCallbackRemovesTheFlowCookie(t *testing.T) {
	for name, fail := range map[string]bool{"success": false, "failure": true} {
		t.Run(name, func(t *testing.T) {
			e := newHandlerEnv(t)
			e.manager.err = nil
			if fail {
				e.manager.err = errors.New("no")
			}
			b := e.browser()
			code, state := e.begin(t, b, "/")
			resp := e.get(t, b, e.callback(code, state))
			if c := flowCookieOf(t, resp); c.MaxAge >= 0 || c.Value != "" {
				t.Errorf("the callback left the cookie in place: %+v", c)
			}
		})
	}
}
