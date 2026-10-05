package token_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"platrium/internal/auth/token"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/authtoken"
	"platrium/internal/infra/db/ent/device"
)

type fixture struct {
	db       *db.DB
	store    *token.Store
	devices  *identity.EntDeviceStore
	tenantID string
	userID   string
	otherTn  string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	devices := identity.NewEntDeviceStore(d)
	f := &fixture{db: d, devices: devices, store: token.NewStore(d, devices, time.Hour)}

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		tn, err := tx.Tenant.Create().SetAlias("acme").SetName("acme").Save(ctx)
		if err != nil {
			return err
		}
		other, err := tx.Tenant.Create().SetAlias("other").SetName("other").Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
		if err != nil {
			return err
		}
		u, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID("u").SetEmail("u@x.com").SetDisplayName("u").Save(ctx)
		if err != nil {
			return err
		}
		f.tenantID, f.userID, f.otherTn = tn.ID, u.ID, other.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) issueDevice(t *testing.T) *token.Issued {
	t.Helper()
	iss, err := f.store.Issue(context.Background(), token.IssueReq{
		TenantID: f.tenantID, UserID: f.userID, Name: "Pixel",
		Device: &identity.RegisterDeviceReq{Name: "Pixel", Platform: "ANDROID", AppVersion: "1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return iss
}

func (f *fixture) issueApp(t *testing.T, exp *time.Time) *token.Issued {
	t.Helper()
	iss, err := f.store.Issue(context.Background(), token.IssueReq{
		TenantID: f.tenantID, UserID: f.userID, Name: "Platrium CLI", ExpiresAt: exp,
	})
	if err != nil {
		t.Fatal(err)
	}
	return iss
}

func TestIssueAndValidate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	dev := f.issueDevice(t)
	if !strings.HasPrefix(dev.Secret, token.Prefix) || dev.DeviceID == "" {
		t.Fatalf("unexpected device token: %+v", dev)
	}
	p, err := f.store.Validate(ctx, dev.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if p.UserID != f.userID || p.TenantID != f.tenantID || p.DeviceID != dev.DeviceID || p.Email != "u@x.com" {
		t.Fatalf("unexpected principal: %+v", p)
	}

	app := f.issueApp(t, nil)
	p, err = f.store.Validate(ctx, app.Secret)
	if err != nil || p.DeviceID != "" {
		t.Fatalf("app token: %+v %v", p, err)
	}
}

func TestOnlyHashIsStored(t *testing.T) {
	f := newFixture(t)
	iss := f.issueApp(t, nil)
	row, err := f.db.AuthToken.Get(context.Background(), iss.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash == iss.Secret || row.TokenHash != token.Hash(iss.Secret) || !strings.HasPrefix(iss.Secret, row.TokenPrefix) {
		t.Fatalf("token not stored as hash: %+v", row)
	}
}

func TestValidateRejects(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, secret := range []string{"", "garbage", token.Prefix + "unknown", "Bearer"} {
		if _, err := f.store.Validate(ctx, secret); !errors.Is(err, token.ErrInvalidToken) {
			t.Errorf("Validate(%q) = %v, want ErrInvalidToken", secret, err)
		}
	}
}

func TestExpiry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	past := time.Now().UTC().Add(-time.Minute)
	app := f.issueApp(t, &past)
	if _, err := f.store.Validate(ctx, app.Secret); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("expired token validated: %v", err)
	}
	future := time.Now().UTC().Add(time.Hour)
	app = f.issueApp(t, &future)
	if _, err := f.store.Validate(ctx, app.Secret); err != nil {
		t.Fatal(err)
	}
}

func TestIdleTimeoutAndTouch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	app := f.issueApp(t, nil)

	// Idle past the 1h timeout: rejected.
	f.db.AuthToken.UpdateOneID(app.ID).SetLastUsedAt(time.Now().UTC().Add(-2 * time.Hour)).ExecX(ctx)
	if _, err := f.store.Validate(ctx, app.Secret); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("idle token validated: %v", err)
	}

	// Recently used but past the touch interval: accepted and last_used_at moves.
	old := time.Now().UTC().Add(-30 * time.Minute)
	f.db.AuthToken.UpdateOneID(app.ID).SetLastUsedAt(old).ExecX(ctx)
	if _, err := f.store.Validate(ctx, app.Secret); err != nil {
		t.Fatal(err)
	}
	if got := f.db.AuthToken.GetX(ctx, app.ID).LastUsedAt; !got.After(old) {
		t.Fatalf("last_used_at not refreshed: %v", got)
	}
}

func TestIssueRejectsForeignTenant(t *testing.T) {
	f := newFixture(t)
	_, err := f.store.Issue(context.Background(), token.IssueReq{TenantID: f.otherTn, UserID: f.userID, Name: "x"})
	if !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListShowsDevicesAndApps(t *testing.T) {
	f := newFixture(t)
	f.issueDevice(t)
	f.issueApp(t, nil)

	got, err := f.store.List(context.Background(), f.tenantID, f.userID)
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %+v, %v", got, err)
	}
	var devs, apps int
	for _, c := range got {
		if c.IsDevice() {
			devs++
			if c.Platform != "ANDROID" {
				t.Errorf("device platform = %q", c.Platform)
			}
		} else {
			apps++
		}
	}
	if devs != 1 || apps != 1 {
		t.Fatalf("devices=%d apps=%d", devs, apps)
	}
	if other, _ := f.store.List(context.Background(), f.otherTn, f.userID); len(other) != 0 {
		t.Fatalf("listed across tenants: %+v", other)
	}
}

func TestDeleteTokenDeletesDevice(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dev := f.issueDevice(t)

	if err := f.store.Delete(ctx, f.tenantID, f.userID, dev.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Validate(ctx, dev.Secret); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("deleted token still valid: %v", err)
	}
	if n := f.db.Device.Query().Where(device.ID(dev.DeviceID)).CountX(ctx); n != 0 {
		t.Fatal("device survived token deletion")
	}
}

func TestDeleteDeviceDeletesToken(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dev := f.issueDevice(t)

	f.db.Device.DeleteOneID(dev.DeviceID).ExecX(ctx)
	if n := f.db.AuthToken.Query().Where(authtoken.ID(dev.ID)).CountX(ctx); n != 0 {
		t.Fatal("token survived device deletion")
	}
	if _, err := f.store.Validate(ctx, dev.Secret); !errors.Is(err, token.ErrInvalidToken) {
		t.Fatalf("token still valid: %v", err)
	}
}

func TestDeleteIsOwnerAndTenantScoped(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	app := f.issueApp(t, nil)

	if err := f.store.Delete(ctx, f.otherTn, f.userID, app.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("cross-tenant delete: %v", err)
	}
	if err := f.store.Delete(ctx, f.tenantID, "someone-else", app.ID); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("cross-user delete: %v", err)
	}
	if _, err := f.store.Validate(ctx, app.Secret); err != nil {
		t.Fatalf("token should be intact: %v", err)
	}
}

func TestPushRegistration(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dev := f.issueDevice(t)

	if err := f.devices.SetPush(ctx, f.tenantID, dev.DeviceID, "FCM", "tok-1"); err != nil {
		t.Fatal(err)
	}
	got, _ := f.devices.GetDevicesForUser(ctx, f.userID)
	if len(got) != 1 || got[0].NotificationTransportType != "FCM" || got[0].PushToken != "tok-1" {
		t.Fatalf("devices = %+v", got)
	}
	if err := f.devices.SetPush(ctx, f.otherTn, dev.DeviceID, "FCM", "x"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("cross-tenant SetPush: %v", err)
	}
	if err := f.devices.ClearPush(ctx, f.tenantID, dev.DeviceID); err != nil {
		t.Fatal(err)
	}
	got, _ = f.devices.GetDevicesForUser(ctx, f.userID)
	if got[0].NotificationTransportType != "" || got[0].PushToken != "" {
		t.Fatalf("push not cleared: %+v", got[0])
	}
}
