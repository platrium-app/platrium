package sqlauthz_test

import (
	"context"
	"testing"
	"time"

	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent/driveitem"
	"platrium/internal/infra/db/ent/grant"
)

type env struct {
	db *db.DB
	az *sqlauthz.Authorizer
}

func newEnv(t *testing.T) *env { return newEnv2(t) }

func newEnv2(t testing.TB) *env {
	t.Helper()
	d := dbtest.New(t)
	return &env{db: d, az: sqlauthz.New(d)}
}

// tenant is a tenant with a LOCAL identity provider.
type tenant struct {
	id, idp string
}

func (e *env) tenant(t testing.TB, alias string) tenant {
	t.Helper()
	ctx := context.Background()
	tn, err := e.db.Tenant.Create().SetAlias(alias).SetName(alias).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idp, err := e.db.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return tenant{id: tn.ID, idp: idp.ID}
}

func (e *env) user(t testing.TB, tn tenant, name string) string {
	t.Helper()
	u, err := e.db.User.Create().SetTenantID(tn.id).SetIdpID(tn.idp).
		SetExternalID(name).SetEmail(name + "@x.com").SetDisplayName(name).Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

// principal resolves a user the way a request would.
func (e *env) principal(t testing.TB, tn tenant, userID string) authz.Principal {
	t.Helper()
	p, err := e.az.Principal(context.Background(), tn.id, userID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// drive creates a private drive owned by owner and returns its ID, which is
// also its root folder's ID.
func (e *env) drive(t testing.TB, tn tenant, owner string) string {
	t.Helper()
	ctx := context.Background()
	d, err := e.db.Drive.Create().SetTenantID(tn.id).SetOwnerID(owner).SetName("Drive").SetType("PRIVATE").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.DriveItem.Create().SetID(d.ID).SetTenantID(tn.id).SetDriveID(d.ID).
		SetKind("FOLDER").SetName("Drive").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	return d.ID
}

func (e *env) item(t testing.TB, tn tenant, driveID, parentID, name, kind string) string {
	t.Helper()
	c := e.db.DriveItem.Create().SetTenantID(tn.id).SetDriveID(driveID).SetParentID(parentID).
		SetName(name).SetKind(kindOf(kind))
	it, err := c.Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func (e *env) folder(t testing.TB, tn tenant, driveID, parentID, name string) string {
	return e.item(t, tn, driveID, parentID, name, "FOLDER")
}

func (e *env) file(t testing.TB, tn tenant, driveID, parentID, name string) string {
	return e.item(t, tn, driveID, parentID, name, "FILE")
}

func (e *env) group(t testing.TB, tn tenant, name string) string {
	t.Helper()
	g, err := e.db.Group.Create().SetTenantID(tn.id).SetIdpID(tn.idp).SetExternalID(name).SetName(name).Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return g.ID
}

// share grants an item to a subject as actor and fails the test on error.
func (e *env) share(t testing.TB, actor authz.Principal, itemID string, s authz.Subject, role authz.Role) *authz.Grant {
	t.Helper()
	g, err := e.az.Grant(context.Background(), actor, authz.GrantInput{ItemID: itemID, Subject: s, Role: role})
	if err != nil {
		t.Fatalf("share %s with %s/%s: %v", itemID, s.Type, s.ID, err)
	}
	return g
}

func userSubject(id string) authz.Subject  { return authz.Subject{Type: authz.SubjectUser, ID: id} }
func groupSubject(id string) authz.Subject { return authz.Subject{Type: authz.SubjectGroup, ID: id} }

func tenantSubject(tn tenant) authz.Subject {
	return authz.Subject{Type: authz.SubjectTenant, ID: tn.id}
}

func publicSubject() authz.Subject {
	return authz.Subject{Type: authz.SubjectPublic, ID: authz.PublicSubjectID}
}

func (e *env) caps(t testing.TB, p authz.Principal, itemID string) authz.Capability {
	t.Helper()
	c, err := e.az.Caps(context.Background(), p, itemID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (e *env) expire(t testing.TB, grantID string) {
	t.Helper()
	if err := e.db.Grant.Update().Where(grant.ID(grantID)).SetExpiresAt(time.Now().UTC().Add(-time.Minute)).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func kindOf(k string) driveitem.Kind { return driveitem.Kind(k) }

// Benchmark-friendly aliases (testing.B satisfies testing.TB).
func (e *env) tenantB(b testing.TB, alias string) tenant           { return e.tenant(b, alias) }
func (e *env) userB(b testing.TB, tn tenant, name string) string   { return e.user(b, tn, name) }
func (e *env) driveB(b testing.TB, tn tenant, owner string) string { return e.drive(b, tn, owner) }
func (e *env) groupB(b testing.TB, tn tenant, name string) string  { return e.group(b, tn, name) }
func (e *env) folderB(b testing.TB, tn tenant, driveID, parentID, name string) string {
	return e.folder(b, tn, driveID, parentID, name)
}

// seedGrant writes a grant straight to the database, bypassing the rules about
// which roles an item may be shared with. Tests use it to put someone in a
// position (say, holding SHARE on one folder) that the public API only reaches
// through a shared drive.
func (e *env) seedGrant(t testing.TB, itemID string, s authz.Subject, role authz.Role) {
	t.Helper()
	ctx := context.Background()
	item, err := e.db.DriveItem.Get(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	caps, _ := role.Caps()
	if err := e.db.Grant.Create().SetTenantID(item.TenantID).SetDriveID(item.DriveID).SetResourceID(itemID).
		SetSubjectType(string(s.Type)).SetSubjectID(s.ID).SetRole(string(role)).SetCaps(int64(caps)).Exec(ctx); err != nil {
		t.Fatal(err)
	}
}

// sharedDrive creates a shared drive and its root, and makes admin its Drive Admin.
func (e *env) sharedDrive(t testing.TB, tn tenant, name, adminUserID string) string {
	t.Helper()
	ctx := context.Background()
	d, err := e.db.Drive.Create().SetTenantID(tn.id).SetName(name).SetType("SHARED").SetSharedNameKey(name).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.db.DriveItem.Create().SetID(d.ID).SetTenantID(tn.id).SetDriveID(d.ID).SetKind("FOLDER").SetName(name).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	e.seedGrant(t, d.ID, userSubject(adminUserID), authz.RoleDriveAdmin)
	return d.ID
}
