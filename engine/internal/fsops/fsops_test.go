package fsops_test

import (
	"context"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
)

// world is a tenant with an owner, an IdP and one private drive.
type world struct {
	tenantID string
	idp      string
	userID   string
	p        authz.Principal // the owner, as a request would resolve them
	drive    *fsops.Drive    // drive.ID is also the root folder ID
}

type env struct {
	db *db.DB
	az *sqlauthz.Authorizer
	fs *fsops.FSOps
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d := dbtest.New(t)
	az := sqlauthz.New(d)
	return &env{db: d, az: az, fs: fsops.NewFSOps(d, nil, az)}
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
		w.tenantID, w.idp, w.userID = tn.ID, idp.ID, u.ID
		w.drive, err = e.fs.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
			TenantID: tn.ID, OwnerID: u.ID, Name: "My Drive", Type: fsops.DriveTypePrivate,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if w.p, err = e.az.Principal(ctx, w.tenantID, w.userID); err != nil {
		t.Fatal(err)
	}
	return w
}

func (e *env) folder(t *testing.T, w world, parentID, name string) *fsops.Folder {
	t.Helper()
	f, err := e.fs.CreateFolder(context.Background(), w.p, parentID, name)
	if err != nil {
		t.Fatalf("create folder %q: %v", name, err)
	}
	return f
}

func (e *env) file(t *testing.T, w world, parentID, name string, hashes ...string) string {
	t.Helper()
	id, err := e.fs.CreateFile(context.Background(), fsops.CreateFileParams{
		Actor: w.p, ParentID: parentID, Name: name, Size: 3, MimeType: "text/plain", HexHashes: hashes,
	})
	if err != nil {
		t.Fatalf("create file %q: %v", name, err)
	}
	return id
}

// member adds another user to the world's tenant and returns them as a
// principal.
func (e *env) member(t *testing.T, w world, name string) authz.Principal {
	t.Helper()
	ctx := context.Background()
	u, err := e.db.User.Create().SetTenantID(w.tenantID).SetIdpID(w.idp).
		SetExternalID(name).SetEmail(name + "@x.com").SetDisplayName(name).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.az.Principal(ctx, w.tenantID, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// share gives who a role on an item, as the world's owner.
func (e *env) share(t *testing.T, w world, itemID string, who authz.Principal, role authz.Role) {
	t.Helper()
	e.shareOpts(t, w, itemID, who, role, false)
}

// shareOpts gives who a role on an item by writing the grant directly. The
// enforcement tests are about what each role may do, so they can use any role
// anywhere; the rules about which roles an item offers are tested in authz.
func (e *env) shareOpts(t *testing.T, w world, itemID string, who authz.Principal, role authz.Role, noDownload bool) *authz.Grant {
	t.Helper()
	ctx := context.Background()
	item, err := e.db.DriveItem.Get(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	caps, err := authz.GrantCaps(role, noDownload)
	if err != nil {
		t.Fatal(err)
	}
	g, err := e.db.Grant.Create().SetTenantID(item.TenantID).SetDriveID(item.DriveID).SetResourceID(itemID).
		SetSubjectType("USER").SetSubjectID(who.UserID).SetRole(string(role)).SetCaps(int64(caps)).Save(ctx)
	if err != nil {
		t.Fatalf("share %s as %s: %v", itemID, role, err)
	}
	return &authz.Grant{ID: g.ID, ItemID: itemID, Role: role, Caps: caps}
}
