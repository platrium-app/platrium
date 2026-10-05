package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"platrium/internal/auth/session"
	"platrium/internal/auth/token"
	"platrium/internal/identity"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

type bearerFixture struct {
	store          *token.Store
	tenantA, userA string
	tenantB, userB string
	sm             http.Handler
	seen           *session.PlatriumSession
	seenInfo       session.AuthInfo
	seenOK         bool
}

func newBearerFixture(t *testing.T) (*bearerFixture, http.Handler) {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	f := &bearerFixture{store: token.NewStore(d, identity.NewEntDeviceStore(d), 0)}

	mk := func(alias string) (string, string) {
		var tid, uid string
		err := d.WithTx(ctx, func(tx *ent.Tx) error {
			tn, err := tx.Tenant.Create().SetAlias(alias).SetName(alias).Save(ctx)
			if err != nil {
				return err
			}
			idp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("l").Save(ctx)
			if err != nil {
				return err
			}
			u, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID("u").SetEmail(alias + "@x.com").SetDisplayName("u").Save(ctx)
			tid, uid = tn.ID, u.ID
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return tid, uid
	}
	f.tenantA, f.userA = mk("a")
	f.tenantB, f.userB = mk("b")

	sm := session.NewManager()
	h := sm.LoadAndSave(session.Middleware(sm)(session.Bearer(f.store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.seen, f.seenOK = session.FromContext(r.Context())
		f.seenInfo, _ = session.AuthInfoFromContext(r.Context())
	}))))
	return f, h
}

func (f *bearerFixture) do(h http.Handler, authz string) *httptest.ResponseRecorder {
	f.seen, f.seenOK = nil, false
	req := httptest.NewRequest("GET", "/x", nil)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBearerResolvesSession(t *testing.T) {
	f, h := newBearerFixture(t)
	ctx := context.Background()

	dev, _ := f.store.Issue(ctx, token.IssueReq{TenantID: f.tenantA, UserID: f.userA, Name: "ph",
		Device: &identity.RegisterDeviceReq{Name: "ph", Platform: "IOS"}})
	app, _ := f.store.Issue(ctx, token.IssueReq{TenantID: f.tenantB, UserID: f.userB, Name: "cli"})

	if rec := f.do(h, "Bearer "+dev.Secret); rec.Code != 200 || !f.seenOK {
		t.Fatalf("device bearer rejected: %d", rec.Code)
	}
	if f.seen.TenantID != f.tenantA || f.seen.UserID != f.userA || f.seen.Email != "a@x.com" {
		t.Fatalf("session = %+v", f.seen)
	}
	if f.seenInfo.Kind != session.AuthKindDevice || f.seenInfo.DeviceID != dev.DeviceID || f.seenInfo.TokenID != dev.ID {
		t.Fatalf("info = %+v", f.seenInfo)
	}

	// Tenant comes from the token, so tenants stay isolated.
	f.do(h, "bearer "+app.Secret)
	if !f.seenOK || f.seen.TenantID != f.tenantB || f.seenInfo.Kind != session.AuthKindApp {
		t.Fatalf("app session = %+v %+v", f.seen, f.seenInfo)
	}
}

func TestBearerInvalidIsRejectedAndDeletedTokenStops(t *testing.T) {
	f, h := newBearerFixture(t)
	ctx := context.Background()
	app, _ := f.store.Issue(ctx, token.IssueReq{TenantID: f.tenantA, UserID: f.userA, Name: "cli"})

	for _, authz := range []string{"Bearer nope", "Bearer " + token.Prefix + "unknown"} {
		if rec := f.do(h, authz); rec.Code != 401 || f.seenOK {
			t.Fatalf("%q: code %d, session %v", authz, rec.Code, f.seenOK)
		}
	}

	if rec := f.do(h, "Bearer "+app.Secret); rec.Code != 200 {
		t.Fatalf("valid token: %d", rec.Code)
	}
	if err := f.store.Delete(ctx, f.tenantA, f.userA, app.ID); err != nil {
		t.Fatal(err)
	}
	if rec := f.do(h, "Bearer "+app.Secret); rec.Code != 401 {
		t.Fatalf("deleted token: %d", rec.Code)
	}
}

func TestNoBearerPassesThroughAnonymous(t *testing.T) {
	f, h := newBearerFixture(t)
	if rec := f.do(h, ""); rec.Code != 200 || f.seenOK {
		t.Fatalf("anonymous: code %d, session %v", rec.Code, f.seenOK)
	}
	// Non-bearer schemes are not ours to judge.
	if rec := f.do(h, "Basic abc"); rec.Code != 200 || f.seenOK {
		t.Fatalf("basic: code %d", rec.Code)
	}
}
