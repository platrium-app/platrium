package actor_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platrium/internal/auth/actor"
	"platrium/internal/auth/session"
	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

type env struct {
	db       *db.DB
	users    *identity.UserStore
	res      *actor.Resolver
	home     string // the native tenant
	acme     string
	root     string // super admin of the native tenant
	boss     string // super admin of acme
	bob      string // member of acme
	groupOfB string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	e := &env{db: d, users: identity.NewUserStore(d)}
	e.res = actor.NewResolver(sqlauthz.New(d), e.users)

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		home, err := tx.Tenant.Create().SetAlias("home").SetName("home").SetNativeSlot(1).Save(ctx)
		if err != nil {
			return err
		}
		acme, err := tx.Tenant.Create().SetAlias("acme").SetName("acme").Save(ctx)
		if err != nil {
			return err
		}
		mk := func(tenantID, name, role string) (string, error) {
			idp, err := tx.IdpProvider.Create().SetTenantID(tenantID).SetType("LOCAL").SetName(name).Save(ctx)
			if err != nil {
				return "", err
			}
			u, err := tx.User.Create().SetTenantID(tenantID).SetIdpID(idp.ID).SetExternalID(name).
				SetEmail(name + "@x.com").SetDisplayName(name).SetRole(role).Save(ctx)
			return u.ID, err
		}
		if e.root, err = mk(home.ID, "root", identity.RoleSuperAdmin); err != nil {
			return err
		}
		if e.boss, err = mk(acme.ID, "boss", identity.RoleSuperAdmin); err != nil {
			return err
		}
		if e.bob, err = mk(acme.ID, "bob", identity.RoleMember); err != nil {
			return err
		}
		e.home, e.acme = home.ID, acme.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// ctx is a request context signed in as the user, session issued now.
func (e *env) ctx(tenantID, userID string) context.Context {
	return e.ctxAt(tenantID, userID, time.Now().UTC())
}

func (e *env) ctxAt(tenantID, userID string, issued time.Time) context.Context {
	return session.WithSession(context.Background(), &session.PlatriumSession{UserID: userID, TenantID: tenantID, IssuedAt: issued})
}

func (e *env) setRole(t *testing.T, userID, role string) {
	t.Helper()
	if err := e.db.User.UpdateOneID(userID).SetRole(role).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityCarriesWhatTheUserMayDo(t *testing.T) {
	e := newEnv(t)

	id, err := e.res.Identity(e.ctx(e.home, e.root))
	if err != nil {
		t.Fatal(err)
	}
	if !id.Native || !id.Perms.Has(authz.PermTenantsManage) || id.UserID != e.root || id.TenantID != e.home || id.Role != identity.RoleSuperAdmin {
		t.Errorf("native super admin: %+v", id)
	}

	// An organization's super admin administers their org, never the cluster.
	id, err = e.res.Identity(e.ctx(e.acme, e.boss))
	if err != nil {
		t.Fatal(err)
	}
	if id.Native || id.Perms.Has(authz.PermTenantsManage) || !id.Perms.Has(authz.PermUsersCreate) {
		t.Errorf("org super admin: native=%v perms=%v", id.Native, id.Perms.Sorted())
	}

	id, err = e.res.Identity(e.ctx(e.acme, e.bob))
	if err != nil || id.Perms.Len() != 0 {
		t.Errorf("member: %v %v", id, err)
	}
}

func TestIdentityRejectsSessionsThatAreNoLongerGood(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	unauth := func(what string, c context.Context) {
		t.Helper()
		if _, err := e.res.Identity(c); !errors.Is(err, actor.ErrUnauthenticated) {
			t.Errorf("%s: %v", what, err)
		}
	}
	unauth("no session", context.Background())
	unauth("unknown user", e.ctx(e.acme, "missing"))
	unauth("user of another tenant", e.ctx(e.home, e.bob))

	// Disabled, and back again.
	if _, err := e.users.SetDisabled(ctx, e.acme, e.bob, true); err != nil {
		t.Fatal(err)
	}
	unauth("disabled", e.ctx(e.acme, e.bob))
	if _, err := e.users.SetDisabled(ctx, e.acme, e.bob, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.res.Identity(e.ctx(e.acme, e.bob)); err != nil {
		t.Errorf("re-enabled: %v", err)
	}

	// Signed out everywhere: only sessions issued before it die.
	before := time.Now().UTC().Add(-time.Minute)
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error { return e.users.RevokeSessionsTx(ctx, tx, e.acme, e.bob) }); err != nil {
		t.Fatal(err)
	}
	unauth("session issued before the revocation", e.ctxAt(e.acme, e.bob, before))
	unauth("session with no issue time (saved before revocation existed)", e.ctxAt(e.acme, e.bob, time.Time{}))
	if _, err := e.res.Identity(e.ctx(e.acme, e.bob)); err != nil {
		t.Errorf("a sign-in after the revocation: %v", err)
	}
	// Another user is untouched.
	if _, err := e.res.Identity(e.ctxAt(e.acme, e.boss, before)); err != nil {
		t.Errorf("someone else's old session: %v", err)
	}
}

func TestForUserAppliesTheSameRulesWithoutASession(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	if id, err := e.res.ForUser(ctx, e.acme, e.bob); err != nil || id.UserID != e.bob {
		t.Fatalf("%v %v", id, err)
	}
	if _, err := e.users.SetDisabled(ctx, e.acme, e.bob, true); err != nil {
		t.Fatal(err)
	}
	// An upload passport must not outlive the account.
	if _, err := e.res.ForUser(ctx, e.acme, e.bob); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Errorf("disabled: %v", err)
	}
}

func TestIdentityIncludesGroups(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	g, err := e.db.Group.Create().SetTenantID(e.acme).SetIdpID(mustIdp(t, e, e.bob)).SetExternalID("g").SetName("g").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlauthz.New(e.db).AddMember(ctx, e.acme, g.ID, authz.MemberUser, e.bob); err != nil {
		t.Fatal(err)
	}
	id, err := e.res.Identity(e.ctx(e.acme, e.bob))
	if err != nil || len(id.GroupIDs) != 1 || id.GroupIDs[0] != g.ID {
		t.Fatalf("groups: %v %v", id, err)
	}
}

func mustIdp(t *testing.T, e *env, userID string) string {
	t.Helper()
	u, err := e.db.User.Get(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return u.IdpID
}

// Within a request that went through the middleware the identity is resolved
// once; Invalidate makes the next ask read again. Without the middleware, and
// on a WebSocket, every ask is fresh.
func TestMemoAndInvalidate(t *testing.T) {
	e := newEnv(t)

	run := func(upgrade string, fn func(ctx context.Context)) {
		t.Helper()
		var seen bool
		h := e.res.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = true
			fn(session.WithSession(r.Context(), &session.PlatriumSession{UserID: e.bob, TenantID: e.acme, IssuedAt: time.Now().UTC()}))
		}))
		req := httptest.NewRequest("GET", "/", nil)
		if upgrade != "" {
			req.Header.Set("Upgrade", upgrade)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if !seen {
			t.Fatal("handler did not run")
		}
	}
	ask := func(ctx context.Context) *actor.Identity {
		t.Helper()
		id, err := e.res.Identity(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	e.setRole(t, e.bob, identity.RoleMember)
	run("", func(ctx context.Context) {
		first := ask(ctx)
		e.setRole(t, e.bob, identity.RoleAdmin) // changes mid-request
		if got := ask(ctx); got.Role != identity.RoleMember || got.UserID != first.UserID {
			t.Errorf("a second ask in the same request must reuse the first: %+v", got)
		}
		actor.Invalidate(ctx)
		if got := ask(ctx); got.Role != identity.RoleAdmin {
			t.Errorf("after Invalidate the change must be seen: %+v", got)
		}
	})

	e.setRole(t, e.bob, identity.RoleMember)
	run("websocket", func(ctx context.Context) {
		ask(ctx)
		e.setRole(t, e.bob, identity.RoleAdmin)
		if got := ask(ctx); got.Role != identity.RoleAdmin {
			t.Errorf("a WebSocket must never remember: %+v", got)
		}
	})

	e.setRole(t, e.bob, identity.RoleMember)
	ctx := e.ctx(e.acme, e.bob) // no middleware at all
	ask(ctx)
	e.setRole(t, e.bob, identity.RoleAdmin)
	if got := ask(ctx); got.Role != identity.RoleAdmin {
		t.Errorf("without the middleware nothing is remembered: %+v", got)
	}
}

func TestPrincipalOrAnonymous(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	p, err := e.res.PrincipalOrAnonymous(context.Background())
	if err != nil || !p.IsAnonymous() {
		t.Errorf("no session is a visitor: %+v %v", p, err)
	}
	if p, err = e.res.PrincipalOrAnonymous(e.ctx(e.acme, e.bob)); err != nil || p.UserID != e.bob {
		t.Errorf("signed in: %+v %v", p, err)
	}
	if _, err := e.users.SetDisabled(ctx, e.acme, e.bob, true); err != nil {
		t.Fatal(err)
	}
	if p, err = e.res.PrincipalOrAnonymous(e.ctx(e.acme, e.bob)); err != nil || !p.IsAnonymous() {
		t.Errorf("a stale session is just a visitor: %+v %v", p, err)
	}
}

// Everyone who asks gets their own copy, so one resolver changing what it was
// handed cannot change what another sees later in the request.
func TestEveryAskGetsItsOwnCopy(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	g, err := e.db.Group.Create().SetTenantID(e.acme).SetIdpID(mustIdp(t, e, e.bob)).SetExternalID("g").SetName("g").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlauthz.New(e.db).AddMember(ctx, e.acme, g.ID, authz.MemberUser, e.bob); err != nil {
		t.Fatal(err)
	}

	h := e.res.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		c := session.WithSession(r.Context(), &session.PlatriumSession{UserID: e.bob, TenantID: e.acme, IssuedAt: time.Now().UTC()})
		first, err := e.res.Identity(c)
		if err != nil {
			t.Fatal(err)
		}
		first.Role = "TAMPERED"
		first.GroupIDs[0] = "tampered"
		first.UserID = "someone-else"

		second, err := e.res.Identity(c)
		if err != nil {
			t.Fatal(err)
		}
		if second.Role != identity.RoleMember || second.GroupIDs[0] != g.ID || second.UserID != e.bob {
			t.Errorf("a later ask saw the first one's changes: %+v", second)
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}
