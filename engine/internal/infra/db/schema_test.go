package db

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"platrium/internal/infra/db/ent"
)

// newTestDB opens an isolated database. SQLite names are unique per test so
// shared-cache in-memory databases never leak between tests.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	cfg := testConfig()
	if cfg.Driver == "sqlite" {
		cfg.DSN = fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	}
	d, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if cfg.Driver != "sqlite" {
		resetTables(t, d, cfg.Driver)
	}
	return d
}

// tables lists every table, used to wipe a shared real database between tests.
var tables = []string{"drive_items", "drives", "devices", "domains", "grants", "group_members", "group_closures", "share_links", "groups", "local_credentials", "users", "idp_providers", "tenants"}

func resetTables(t *testing.T, d *DB, driver string) {
	t.Helper()
	ctx := context.Background()
	switch driver {
	case "postgres":
		if _, err := d.sqlDB.ExecContext(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" CASCADE"); err != nil {
			t.Fatal(err)
		}
	default: // mysql, mariadb
		conn, err := d.sqlDB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
			t.Fatal(err)
		}
		for _, tbl := range tables {
			if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE `"+tbl+"`"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1"); err != nil {
			t.Fatal(err)
		}
	}
}

type fixture struct {
	tenant *ent.Tenant
	idp    *ent.IdpProvider
	user   *ent.User
	drive  *ent.Drive
	root   *ent.DriveItem
}

// seed creates a tenant with an IdP, a user, and a drive with its root folder.
func seed(t *testing.T, d *DB, alias string) fixture {
	t.Helper()
	ctx := context.Background()
	var f fixture
	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		if f.tenant, err = tx.Tenant.Create().SetAlias(alias).SetName(alias).Save(ctx); err != nil {
			return err
		}
		if f.idp, err = tx.IdpProvider.Create().SetTenantID(f.tenant.ID).SetType("LOCAL").SetName("local").Save(ctx); err != nil {
			return err
		}
		if f.user, err = tx.User.Create().SetTenantID(f.tenant.ID).SetIdpID(f.idp.ID).
			SetExternalID("ext-" + alias).SetEmail(alias + "@x.com").SetDisplayName(alias).Save(ctx); err != nil {
			return err
		}
		if f.drive, err = tx.Drive.Create().SetTenantID(f.tenant.ID).SetOwnerID(f.user.ID).
			SetName("My Drive").SetType("PRIVATE").Save(ctx); err != nil {
			return err
		}
		f.root, err = tx.DriveItem.Create().SetID(f.drive.ID).SetTenantID(f.tenant.ID).
			SetDriveID(f.drive.ID).SetKind("FOLDER").SetName("My Drive").Save(ctx)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSchemaDefaults(t *testing.T) {
	d := newTestDB(t)
	f := seed(t, d, "acme")

	if f.tenant.ID == "" || f.user.Role != "MEMBER" || f.drive.StorageUsed != 0 || f.drive.StorageQuota != nil {
		t.Fatalf("unexpected defaults: %+v %+v %+v", f.tenant, f.user, f.drive)
	}
	if f.root.CreatedAt.Location().String() != "UTC" {
		t.Fatalf("created_at not UTC: %v", f.root.CreatedAt.Location())
	}
}

func TestUpdatedAtBumpsOnUpdate(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	f := seed(t, d, "acme")

	got, err := d.DriveItem.UpdateOneID(f.root.ID).SetName("renamed").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UpdatedAt.After(f.root.UpdatedAt) {
		t.Fatalf("updated_at not bumped: %v -> %v", f.root.UpdatedAt, got.UpdatedAt)
	}
}

func TestConstraints(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	f := seed(t, d, "acme")

	mustFail := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s: expected a constraint violation", name)
		}
	}

	_, err := d.Tenant.Create().SetAlias("Mixed").SetName("x").Save(ctx)
	mustFail("uppercase tenant alias", err)

	_, err = d.Tenant.Create().SetAlias("acme").SetName("dup").Save(ctx)
	mustFail("duplicate tenant alias", err)

	one := 1
	if _, err := d.Tenant.Create().SetAlias("native").SetName("n").SetNativeSlot(one).Save(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = d.Tenant.Create().SetAlias("native2").SetName("n2").SetNativeSlot(one).Save(ctx)
	mustFail("second native tenant", err)

	// Many non-native tenants coexist (NULL native_slot).
	for _, a := range []string{"a1", "a2", "a3"} {
		if _, err := d.Tenant.Create().SetAlias(a).SetName(a).Save(ctx); err != nil {
			t.Fatalf("non-native tenant %s: %v", a, err)
		}
	}

	_, err = d.User.Create().SetTenantID(f.tenant.ID).SetIdpID(f.idp.ID).SetExternalID(f.user.ExternalID).
		SetEmail("o@x.com").SetDisplayName("o").Save(ctx)
	mustFail("duplicate (idp, external_id)", err)

	if _, err := d.Domain.Create().SetTenantID(f.tenant.ID).SetName("acme.com").Save(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = d.Domain.Create().SetTenantID(f.tenant.ID).SetName("acme.com").Save(ctx)
	mustFail("duplicate domain", err)
	_, err = d.Domain.Create().SetTenantID(f.tenant.ID).SetName("ACME.org").Save(ctx)
	mustFail("uppercase domain", err)

	// Only a drive's root may lack a parent.
	_, err = d.DriveItem.Create().SetTenantID(f.tenant.ID).SetDriveID(f.drive.ID).SetKind("FOLDER").SetName("orphan").Save(ctx)
	mustFail("non-root item without parent", err)

	// Folders cannot carry file metadata.
	_, err = d.DriveItem.Create().SetTenantID(f.tenant.ID).SetDriveID(f.drive.ID).SetParentID(f.root.ID).
		SetKind("FOLDER").SetName("bad").SetSize(10).Save(ctx)
	mustFail("folder with size", err)

	_, err = d.DriveItem.Create().SetTenantID(f.tenant.ID).SetDriveID(f.drive.ID).SetParentID(f.root.ID).
		SetKind("FILE").SetName("neg").SetSize(-1).Save(ctx)
	mustFail("negative file size", err)

	file, err := d.DriveItem.Create().SetTenantID(f.tenant.ID).SetDriveID(f.drive.ID).SetParentID(f.root.ID).
		SetKind("FILE").SetName("a.txt").SetSize(3).SetMimeType("text/plain").SetInlineChunks([]string{"ab"}).Save(ctx)
	if err != nil {
		t.Fatalf("valid file: %v", err)
	}
	if len(file.InlineChunks) != 1 {
		t.Fatalf("inline_chunks round trip: %v", file.InlineChunks)
	}

	// Foreign keys restrict, never silently orphan.
	mustFail("delete parent with children", d.DriveItem.DeleteOneID(f.root.ID).Exec(ctx))
	mustFail("delete tenant with users", d.Tenant.DeleteOneID(f.tenant.ID).Exec(ctx))
}

// IDs differing only by case must be distinct on every backend. MySQL and
// MariaDB default to case-insensitive collations, which would reject the
// second insert (or, worse, merge the two rows) without binary ID columns.
func TestIDsAreCaseSensitive(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)

	for i, id := range []string{"aB", "Ab", "AB", "ab"} {
		// Aliases must be lowercase and distinct; the IDs are what is under test.
		if _, err := d.Tenant.Create().SetID(id).SetAlias(fmt.Sprintf("t%d", i)).SetName(id).Save(ctx); err != nil {
			t.Fatalf("tenant id %q: %v", id, err)
		}
	}
	if n, _ := d.Tenant.Query().Count(ctx); n != 4 {
		t.Fatalf("expected 4 tenants, got %d", n)
	}
	got, err := d.Tenant.Get(ctx, "Ab")
	if err != nil || got.ID != "Ab" {
		t.Fatalf("lookup by exact-case id: %+v %v", got, err)
	}
}
