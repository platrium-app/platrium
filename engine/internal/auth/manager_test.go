package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"platrium/internal/apperr"
	"platrium/internal/auth"
	"platrium/internal/auth/session"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/user"
)

// userOnly provisions users without drives, which is all the manager needs.
type userOnly struct{ users *identity.UserStore }

func (p userOnly) ProvisionUserTx(ctx context.Context, tx *ent.Tx, c identity.CreateUserParams) (*identity.User, error) {
	return p.users.CreateUserTx(ctx, tx, c)
}

type menv struct {
	db     *db.DB
	idps   *auth.IdpStore
	users  *identity.UserStore
	mgr    *auth.Manager
	sm     *scs.SessionManager
	tenant *identity.Tenant
	local  *auth.IdpProvider
	oidc   *auth.IdpProvider
}

func newMenv(t *testing.T, policy auth.ProvisioningPolicy) *menv {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	e := &menv{db: d, idps: auth.NewIdpStore(d), users: identity.NewUserStore(d)}
	sm := session.NewManager()
	e.sm = sm
	e.mgr = auth.NewManager(d, e.idps, e.users, userOnly{e.users}, sm)

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if e.tenant, err = identity.NewTenantStore(d).CreateTenantTx(ctx, tx, identity.CreateTenantParams{Alias: "acme", Name: "Acme"}); err != nil {
			return err
		}
		if e.local, err = e.idps.CreateLocalIdpTx(ctx, tx, e.tenant.ID, ""); err != nil {
			return err
		}
		e.oidc, err = e.idps.CreateTx(ctx, tx, auth.CreateIdpParams{TenantID: e.tenant.ID, Type: auth.IdpTypeOIDC, Name: "Okta", ProvisioningPolicy: policy})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

var jit = auth.ProvisioningPolicy{JITUsers: true, DefaultRole: "MEMBER"}

func (e *menv) handoff(sub, email string) auth.IdpAuthHandoff {
	return auth.IdpAuthHandoff{IdpProviderID: e.oidc.ID, SubjectID: sub, Email: email, EmailVerified: true, DisplayName: "Alice"}
}

// login runs a sign-in inside a real session-managed request and returns the
// session it produced.
func (e *menv) login(h auth.IdpAuthHandoff) (*session.PlatriumSession, error) {
	var (
		sess *session.PlatriumSession
		err  error
	)
	e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err = e.mgr.HandleFederatedLogin(r.Context(), h)
		sess, _ = e.sm.Get(r.Context(), session.StoreKey).(*session.PlatriumSession)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	return sess, err
}

func (e *menv) countUsers(t *testing.T) int {
	t.Helper()
	return e.db.User.Query().CountX(context.Background())
}

func TestFirstSignInCreatesUserWithDefaultRole(t *testing.T) {
	e := newMenv(t, jit)
	sess, err := e.login(e.handoff("sub-1", "alice@acme.com"))
	if err != nil {
		t.Fatal(err)
	}
	u := e.db.User.Query().Where(user.ExternalID("sub-1")).OnlyX(context.Background())
	if u.Email != "alice@acme.com" || u.DisplayName != "Alice" || u.Role != "MEMBER" || u.IdpID != e.oidc.ID || u.TenantID != e.tenant.ID {
		t.Errorf("user = %+v", u)
	}
	if sess == nil || sess.UserID != u.ID || sess.TenantID != e.tenant.ID || sess.Email != "alice@acme.com" || sess.IssuedAt.IsZero() {
		t.Errorf("session = %+v", sess)
	}
}

func TestSecondSignInReusesTheUser(t *testing.T) {
	e := newMenv(t, jit)
	first, _ := e.login(e.handoff("sub-1", "alice@acme.com"))
	second, err := e.login(e.handoff("sub-1", "alice@acme.com"))
	if err != nil || second.UserID != first.UserID || e.countUsers(t) != 1 {
		t.Fatalf("err = %v, users = %d", err, e.countUsers(t))
	}
}

func TestProfileIsResyncedOnEverySignIn(t *testing.T) {
	e := newMenv(t, jit)
	e.login(e.handoff("sub-1", "alice@acme.com"))

	h := e.handoff("sub-1", "alice.smith@acme.com")
	h.DisplayName = "Alice Smith"
	sess, err := e.login(h)
	if err != nil {
		t.Fatal(err)
	}
	u := e.db.User.Query().OnlyX(context.Background())
	if u.Email != "alice.smith@acme.com" || u.DisplayName != "Alice Smith" || u.ExternalID != "sub-1" {
		t.Errorf("user = %+v", u)
	}
	if sess.Email != "alice.smith@acme.com" {
		t.Errorf("session email = %q", sess.Email)
	}
}

func TestNoAccountWhenJITIsOff(t *testing.T) {
	e := newMenv(t, auth.ProvisioningPolicy{DefaultRole: "MEMBER"})
	sess, err := e.login(e.handoff("sub-1", "alice@acme.com"))
	if !errors.Is(err, auth.ErrNotProvisioned) || sess != nil || e.countUsers(t) != 0 {
		t.Fatalf("err = %v, session = %+v, users = %d", err, sess, e.countUsers(t))
	}
}

func TestExistingUserSignsInWhenJITIsOff(t *testing.T) {
	e := newMenv(t, auth.ProvisioningPolicy{DefaultRole: "MEMBER"})
	ctx := context.Background()
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		_, err := e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: e.tenant.ID, IdpID: e.oidc.ID, ExternalID: "sub-1", Email: "alice@acme.com", DisplayName: "Alice"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.login(e.handoff("sub-1", "alice@acme.com")); err != nil {
		t.Fatal(err)
	}
}

func TestDisabledUserIsRefused(t *testing.T) {
	e := newMenv(t, jit)
	e.login(e.handoff("sub-1", "alice@acme.com"))
	u := e.db.User.Query().OnlyX(context.Background())
	if _, err := e.users.SetDisabled(context.Background(), e.tenant.ID, u.ID, true); err != nil {
		t.Fatal(err)
	}
	sess, err := e.login(e.handoff("sub-1", "alice@acme.com"))
	if !errors.Is(err, auth.ErrUserDisabled) || sess != nil {
		t.Fatalf("err = %v, session = %+v", err, sess)
	}
}

func TestDisabledProviderIsRefusedForNewAndExistingUsers(t *testing.T) {
	e := newMenv(t, jit)
	e.login(e.handoff("sub-1", "alice@acme.com"))
	e.db.IdpProvider.UpdateOneID(e.oidc.ID).SetEnabled(false).ExecX(context.Background())

	for _, sub := range []string{"sub-1", "sub-new"} {
		if sess, err := e.login(e.handoff(sub, "alice@acme.com")); !errors.Is(err, auth.ErrProviderDisabled) || sess != nil {
			t.Errorf("%s: err = %v, session = %+v", sub, err, sess)
		}
	}
	if e.countUsers(t) != 1 {
		t.Errorf("a disabled provider created a user")
	}
}

func TestEmailDomainRestriction(t *testing.T) {
	e := newMenv(t, auth.ProvisioningPolicy{JITUsers: true, DefaultRole: "MEMBER", AllowedEmailDomains: []string{"acme.com"}})

	if _, err := e.login(e.handoff("sub-1", "alice@evil.com")); !errors.Is(err, auth.ErrEmailNotAllowed) || e.countUsers(t) != 0 {
		t.Fatalf("outsider: err = %v, users = %d", err, e.countUsers(t))
	}
	if _, err := e.login(e.handoff("sub-2", "bob@acme.com")); err != nil {
		t.Fatal(err)
	}
	// An existing user whose address moves outside the allowed domains is out too.
	sess, err := e.login(e.handoff("sub-2", "bob@evil.com"))
	if !errors.Is(err, auth.ErrEmailNotAllowed) || sess != nil {
		t.Fatalf("moved address: err = %v, session = %+v", err, sess)
	}
	if u := e.db.User.Query().OnlyX(context.Background()); u.Email != "bob@acme.com" {
		t.Errorf("a refused sign-in changed the profile: %q", u.Email)
	}
}

func TestNeverLinksByEmail(t *testing.T) {
	e := newMenv(t, jit)
	ctx := context.Background()
	var localUser *identity.User
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		localUser, err = e.users.CreateUserTx(ctx, tx, identity.CreateUserParams{TenantID: e.tenant.ID, IdpID: e.local.ID, ExternalID: "admin@acme.com", Email: "admin@acme.com", DisplayName: "Admin", Role: "SUPER_ADMIN"})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// Someone arrives through the external provider claiming the admin's email.
	sess, err := e.login(e.handoff("attacker-sub", "admin@acme.com"))
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID == localUser.ID {
		t.Fatal("an external identity signed in as a local user with the same email")
	}
	u := e.db.User.Query().Where(user.ID(sess.UserID)).OnlyX(ctx)
	if u.Role != "MEMBER" || u.IdpID != e.oidc.ID {
		t.Errorf("new user = %+v", u)
	}
}

func TestSameSubjectOnTwoProvidersIsTwoUsers(t *testing.T) {
	e := newMenv(t, jit)
	ctx := context.Background()
	var second *auth.IdpProvider
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		second, err = e.idps.CreateTx(ctx, tx, auth.CreateIdpParams{TenantID: e.tenant.ID, Type: auth.IdpTypeOIDC, Name: "Google", ProvisioningPolicy: jit})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a, _ := e.login(e.handoff("same-sub", "x@acme.com"))
	h := e.handoff("same-sub", "x@acme.com")
	h.IdpProviderID = second.ID
	b, err := e.login(h)
	if err != nil || a.UserID == b.UserID {
		t.Fatalf("err = %v, same user: %v", err, a.UserID == b.UserID)
	}
}

func TestFederationRejectsLocalAndUnknownProviders(t *testing.T) {
	e := newMenv(t, jit)
	h := e.handoff("sub-1", "alice@acme.com")

	h.IdpProviderID = e.local.ID
	if _, err := e.login(h); !errors.Is(err, apperr.ErrInvalid) {
		t.Errorf("local: err = %v, want ErrInvalid", err)
	}
	h.IdpProviderID = "nope"
	if _, err := e.login(h); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("unknown: err = %v, want ErrNotFound", err)
	}
	for name, bad := range map[string]auth.IdpAuthHandoff{
		"no subject": e.handoff("", "alice@acme.com"),
		"no email":   e.handoff("sub-1", "  "),
	} {
		if _, err := e.login(bad); !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestUnsafeStoredDefaultRoleIsNotTrusted(t *testing.T) {
	e := newMenv(t, jit)
	// Someone edits the row directly, bypassing the store's validation.
	e.db.IdpProvider.UpdateOneID(e.oidc.ID).SetDefaultRole("SUPER_ADMIN").ExecX(context.Background())

	if _, err := e.login(e.handoff("sub-1", "alice@acme.com")); !errors.Is(err, apperr.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if e.countUsers(t) != 0 {
		t.Error("a user was created with an administrative default role")
	}
}

func TestConcurrentFirstSignInsMakeOneUser(t *testing.T) {
	e := newMenv(t, jit)
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.login(e.handoff("sub-1", "alice@acme.com"))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("sign-in failed: %v", err)
		}
	}
	if n := e.countUsers(t); n != 1 {
		t.Fatalf("%d users, want 1", n)
	}
}

func TestAdmit(t *testing.T) {
	enabled := &auth.IdpProvider{Enabled: true}
	disabledUser := &identity.User{}
	now := time.Now()
	disabledUser.DisabledAt = &now

	if err := enabled.Admit("a@b.com", nil); err != nil {
		t.Error(err)
	}
	if err := (&auth.IdpProvider{}).Admit("a@b.com", nil); !errors.Is(err, auth.ErrProviderDisabled) {
		t.Errorf("disabled provider: %v", err)
	}
	if err := enabled.Admit("a@b.com", disabledUser); !errors.Is(err, auth.ErrUserDisabled) {
		t.Errorf("disabled user: %v", err)
	}
}
