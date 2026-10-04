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

type fixture struct {
	db        *db.DB
	store     *local.LocalUserStore
	tenantID  string
	userID    string // user on the LOCAL IdP
	oidcUser  string // user on an OIDC IdP, which may not hold a local credential
	otherTnID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	f := &fixture{db: d, store: local.NewLocalUserStore(d)}

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
		lu, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(localIdp.ID).SetExternalID("l").SetEmail("l@x.com").SetDisplayName("l").Save(ctx)
		if err != nil {
			return err
		}
		ou, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(oidcIdp.ID).SetExternalID("o").SetEmail("o@x.com").SetDisplayName("o").Save(ctx)
		if err != nil {
			return err
		}
		f.tenantID, f.userID, f.oidcUser, f.otherTnID = tn.ID, lu.ID, ou.ID, other.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) create(userID, tenantID, password string) error {
	ctx := context.Background()
	hash, err := local.HashPassword(password)
	if err != nil {
		return err
	}
	return f.db.WithTx(ctx, func(tx *ent.Tx) error {
		return f.store.CreateTx(ctx, tx, tenantID, userID, hash)
	})
}

func TestCreateAndVerifyPassword(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	if err := f.create(f.userID, f.tenantID, "correct horse"); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.store.VerifyPassword(ctx, f.userID, "correct horse"); err != nil || !ok {
		t.Fatalf("right password: %v %v", ok, err)
	}
	if ok, err := f.store.VerifyPassword(ctx, f.userID, "wrong"); err != nil || ok {
		t.Fatalf("wrong password must be (false, nil): %v %v", ok, err)
	}
	if _, err := f.store.VerifyPassword(ctx, "missing", "x"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("missing credential: %v", err)
	}
}

func TestPasswordIsNeverStoredInPlaintext(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if err := f.create(f.userID, f.tenantID, "s3cret-value"); err != nil {
		t.Fatal(err)
	}
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
	if err := f.create(f.userID, f.tenantID, "old-password"); err != nil {
		t.Fatal(err)
	}
	before, _ := f.db.LocalCredential.Query().Only(ctx)

	if err := f.store.SetPassword(ctx, f.userID, "new-password"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.store.VerifyPassword(ctx, f.userID, "old-password"); ok {
		t.Error("old password must stop working")
	}
	if ok, _ := f.store.VerifyPassword(ctx, f.userID, "new-password"); !ok {
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

func TestCreateConstraints(t *testing.T) {
	f := newFixture(t)

	if err := f.create(f.userID, f.tenantID, "a"); err != nil {
		t.Fatal(err)
	}
	if err := f.create(f.userID, f.tenantID, "b"); !errors.Is(err, identity.ErrConflict) {
		t.Errorf("a second credential must conflict, got %v", err)
	}
	if err := f.create(f.oidcUser, f.tenantID, "a"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("only LOCAL-IdP users may hold a credential, got %v", err)
	}
	// Tenant isolation: the user must belong to the tenant named on the credential.
	if err := f.create(f.userID, f.otherTnID, "a"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("cross-tenant credential must be rejected, got %v", err)
	}
}

func TestCredentialRollsBackWithTransaction(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	hash, _ := local.HashPassword("pw")

	boom := errors.New("boom")
	err := f.db.WithTx(ctx, func(tx *ent.Tx) error {
		if err := f.store.CreateTx(ctx, tx, f.tenantID, f.userID, hash); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := f.store.VerifyPassword(ctx, f.userID, "pw"); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("credential must not survive a rolled back transaction: %v", err)
	}
}
