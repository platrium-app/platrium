package fsops_test

import (
	"context"
	"testing"

	"platrium/internal/fsops"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

// world is a tenant with an owner, an IdP and one private drive.
type world struct {
	tenantID string
	userID   string
	drive    *fsops.Drive // drive.ID is also the root folder ID
}

type env struct {
	db *db.DB
	fs *fsops.FSOps
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d := dbtest.New(t)
	return &env{db: d, fs: fsops.NewFSOps(d, nil)}
}

// newWorld seeds a tenant, a local IdP, a user and a private drive.
func (e *env) newWorld(t *testing.T, alias string) world {
	t.Helper()
	ctx := context.Background()
	var w world
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		tn, err := tx.Tenant.Create().SetAlias(alias).SetName(alias).Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
		if err != nil {
			return err
		}
		u, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID(alias).SetEmail(alias + "@x.com").SetDisplayName(alias).Save(ctx)
		if err != nil {
			return err
		}
		w.tenantID, w.userID = tn.ID, u.ID
		w.drive, err = e.fs.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
			TenantID: tn.ID, OwnerID: u.ID, Name: "My Drive", Type: fsops.DriveTypePrivate,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (e *env) folder(t *testing.T, w world, parentID, name string) *fsops.Folder {
	t.Helper()
	f, err := e.fs.CreateFolder(context.Background(), w.tenantID, parentID, name)
	if err != nil {
		t.Fatalf("create folder %q: %v", name, err)
	}
	return f
}

func (e *env) file(t *testing.T, w world, parentID, name string, hashes ...string) string {
	t.Helper()
	id, err := e.fs.CreateFile(context.Background(), fsops.CreateFileParams{
		TenantID: w.tenantID, ParentID: parentID, Name: name, Size: 3, MimeType: "text/plain", HexHashes: hashes,
	})
	if err != nil {
		t.Fatalf("create file %q: %v", name, err)
	}
	return id
}
