package local_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/auth/protocol/local"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

// provisioner stands in for the orchestrator: it creates the user row without
// the drive, which these tests do not need.
type provisioner struct{ users *identity.UserStore }

func (p provisioner) ProvisionUserTx(ctx context.Context, tx *ent.Tx, params identity.CreateUserParams) (*identity.User, error) {
	return p.users.CreateUserTx(ctx, tx, params)
}

type fixture struct {
	db        *db.DB
	store     *local.LocalUserStore
	tenantID  string
	localIdp  string
	oidcIdp   string
	otherIdp  string // a LOCAL IdP of a different tenant
	otherTnID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	f := &fixture{db: d, store: local.NewLocalUserStore(d, provisioner{identity.NewUserStore(d)})}

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		tn, err := tx.Tenant.Create().SetAlias("acme").SetName("acme").Save(ctx)
		if err != nil {
			return err
		}
		other, err := tx.Tenant.Create().SetAlias("other").SetName("other").Save(ctx)
		if err != nil {
			return err
		}
		localIdp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
		if err != nil {
			return err
		}
		oidcIdp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("OIDC").SetName("sso").Save(ctx)
		if err != nil {
			return err
		}
		otherIdp, err := tx.IdpProvider.Create().SetTenantID(other.ID).SetType("LOCAL").SetName("local").Save(ctx)
		if err != nil {
			return err
		}
		f.tenantID, f.localIdp, f.oidcIdp, f.otherIdp, f.otherTnID = tn.ID, localIdp.ID, oidcIdp.ID, otherIdp.ID, other.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) create(tenantID, idpID, externalID, password string) (*identity.User, error) {
	ctx := context.Background()
	hash, err := local.HashPassword(password)
	if err != nil {
		return nil, err
	}
	var u *identity.User
	err = f.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		u, err = f.store.CreateUserTx(ctx, tx, identity.CreateUserParams{
			TenantID:    tenantID,
			IdpID:       idpID,
			ExternalID:  externalID,
			Email:       externalID,
			DisplayName: externalID,
		}, hash)
		return err
	})
	return u, err
}

func (f *fixture) mustCreate(t *testing.T, externalID, password string) *identity.User {
	t.Helper()
	u, err := f.create(f.tenantID, f.localIdp, externalID, password)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCreateAndVerifyPassword(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	u := f.mustCreate(t, "a@x.com", "correct horse")
	if ok, err := f.store.VerifyPassword(ctx, u.ID, "correct horse"); err != nil || !ok {
		t.Fatalf("right password: %v %v", ok, err)
	}
	if ok, err := f.store.VerifyPassword(ctx, u.ID, "wrong"); err != nil || ok {
		t.Fatalf("wrong password must be (false, nil): %v %v", ok, err)
	}
	if _, err := f.store.VerifyPassword(ctx, "missing", "x"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("missing credential: %v", err)
	}
}

func TestPasswordIsNeverStoredInPlaintext(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.mustCreate(t, "a@x.com", "s3cret-value")
	cred, err := f.db.LocalCredential.Query().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cred.PasswordHash == "s3cret-value" || len(cred.PasswordHash) < 50 {
		t.Fatalf("password hash looks wrong: %q", cred.PasswordHash)
	}
}

func TestSetPassword(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	u := f.mustCreate(t, "a@x.com", "old-password")
	before, _ := f.db.LocalCredential.Query().Only(ctx)

	if err := f.store.SetPassword(ctx, u.ID, "new-password"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.store.VerifyPassword(ctx, u.ID, "old-password"); ok {
		t.Error("old password must stop working")
	}
	if ok, _ := f.store.VerifyPassword(ctx, u.ID, "new-password"); !ok {
		t.Error("new password must work")
	}
	after, _ := f.db.LocalCredential.Query().Only(ctx)
	if !after.PasswordChangedAt.After(before.PasswordChangedAt) || after.ID != before.ID {
		t.Errorf("password_changed_at not bumped on the same record")
	}
	if err := f.store.SetPassword(ctx, "missing", "x"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("reset for missing credential: %v", err)
	}
}

func TestCreateUserConstraints(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	f.mustCreate(t, "a@x.com", "a")
	if _, err := f.create(f.tenantID, f.localIdp, "a@x.com", "b"); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("the same login twice must conflict, got %v", err)
	}
	if _, err := f.create(f.tenantID, f.oidcIdp, "o@x.com", "a"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("only LOCAL-IdP users may hold a credential, got %v", err)
	}
	// Tenant isolation: the IdP must belong to the tenant the user is created in.
	if _, err := f.create(f.tenantID, f.otherIdp, "c@x.com", "a"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("cross-tenant IdP must be rejected, got %v", err)
	}

	// None of the rejected attempts may leave a half-created user behind.
	if n, _ := f.db.User.Query().Count(ctx); n != 1 {
		t.Errorf("expected only the first user to exist, got %d", n)
	}
}

func TestUserAndCredentialRollBackTogether(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	hash, _ := local.HashPassword("pw")

	boom := errors.New("boom")
	err := f.db.WithTx(ctx, func(tx *ent.Tx) error {
		if _, err := f.store.CreateUserTx(ctx, tx, identity.CreateUserParams{
			TenantID: f.tenantID, IdpID: f.localIdp, ExternalID: "a@x.com", Email: "a@x.com", DisplayName: "A",
		}, hash); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if n, _ := f.db.User.Query().Count(ctx); n != 0 {
		t.Errorf("user must not survive a rolled back transaction, got %d", n)
	}
	if n, _ := f.db.LocalCredential.Query().Count(ctx); n != 0 {
		t.Errorf("credential must not survive a rolled back transaction, got %d", n)
	}
}
