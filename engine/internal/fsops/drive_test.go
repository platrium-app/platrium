package fsops_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/infra/db/ent"
)

func TestCreateDriveCreatesRootFolder(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	root, err := e.fs.GetItem(ctx, w.p, w.drive.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsFolder() || root.ParentID != nil || root.Name != "My Drive" {
		t.Fatalf("unexpected root: %+v", root)
	}
	if w.drive.StorageUsed != 0 || w.drive.StorageQuota != 0 || w.drive.Type != fsops.DriveTypePrivate {
		t.Fatalf("unexpected drive: %+v", w.drive)
	}

	drives, err := e.fs.GetUserDrives(ctx, w.p)
	if err != nil || len(drives) != 1 || drives[0].ID != w.drive.ID {
		t.Fatalf("drives: %+v %v", drives, err)
	}
}

func TestCreateDriveValidation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")

	create := func(p fsops.CreateDriveParams) error {
		return e.db.WithTx(ctx, func(tx *ent.Tx) error { _, err := e.fs.CreateDriveTx(ctx, tx, p); return err })
	}
	if err := create(fsops.CreateDriveParams{TenantID: a.tenantID, OwnerID: a.userID, Name: "", Type: fsops.DriveTypeShared}); err == nil {
		t.Error("empty name must fail")
	}
	if err := create(fsops.CreateDriveParams{TenantID: a.tenantID, OwnerID: a.userID, Name: "x", Type: "WRONG"}); err == nil {
		t.Error("invalid type must fail")
	}
	// Tenant isolation: tenant A cannot create a drive owned by tenant B's user.
	if err := create(fsops.CreateDriveParams{TenantID: a.tenantID, OwnerID: b.userID, Name: "x", Type: fsops.DriveTypeShared}); err == nil {
		t.Error("owner from another tenant must fail")
	}
	// A failed drive creation leaves no orphan root item behind.
	if n, _ := e.db.DriveItem.Query().Count(ctx); n != 2 {
		t.Errorf("expected only the two seeded roots, got %d items", n)
	}
}

func TestGetUserDrivesIsTenantScoped(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")

	// Tenant A's view of tenant B's user: the principal's tenant scopes everything.
	spoofed := authz.Principal{TenantID: a.tenantID, UserID: b.userID}
	if drives, _ := e.fs.GetUserDrives(ctx, spoofed); len(drives) != 0 {
		t.Fatalf("another tenant's user must have no drives here, got %+v", drives)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, b.p); len(drives) != 1 || drives[0].TenantID != b.tenantID {
		t.Fatalf("own drives only, got %+v", drives)
	}
}

func TestCreateDriveOwnership(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	create := func(p fsops.CreateDriveParams) error {
		p.TenantID = w.tenantID
		return e.db.WithTx(ctx, func(tx *ent.Tx) error { _, err := e.fs.CreateDriveTx(ctx, tx, p); return err })
	}
	cases := []struct {
		name string
		p    fsops.CreateDriveParams
		ok   bool
	}{
		{"shared drive", fsops.CreateDriveParams{Name: "Company", Type: fsops.DriveTypeShared}, true},
		{"shared drive with an owner", fsops.CreateDriveParams{Name: "x", OwnerID: w.userID, Type: fsops.DriveTypeShared}, false},
		{"private drive without an owner", fsops.CreateDriveParams{Name: "x", Type: fsops.DriveTypePrivate}, false},
		{"private drive with an owner", fsops.CreateDriveParams{Name: "x", OwnerID: w.userID, Type: fsops.DriveTypePrivate}, true},
		{"unknown type", fsops.CreateDriveParams{Name: "x", Type: "ROBOT"}, false},
		{"blank name", fsops.CreateDriveParams{Name: "   ", Type: fsops.DriveTypeShared}, false},
	}
	for _, c := range cases {
		if err := create(c.p); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
}

// Shared-drive names are unique within a tenant, ignoring case and surrounding
// spaces. Private drives are not named uniquely (everyone has a "My Drive").
func TestSharedDriveNamesAreUnique(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")

	create := func(tenantID, name string) error {
		_, err := e.fs.CreateDrive(ctx, fsops.CreateDriveParams{TenantID: tenantID, Name: name, Type: fsops.DriveTypeShared})
		return err
	}
	if err := create(a.tenantID, "Finance"); err != nil {
		t.Fatal(err)
	}
	for _, dup := range []string{"Finance", "finance", "FINANCE", "  Finance "} {
		if err := create(a.tenantID, dup); !errors.Is(err, fsops.ErrConflict) {
			t.Errorf("%q must conflict: %v", dup, err)
		}
	}
	if err := create(a.tenantID, "Finance 2"); err != nil {
		t.Errorf("a different name is fine: %v", err)
	}
	if err := create(b.tenantID, "Finance"); err != nil {
		t.Errorf("another tenant may reuse the name: %v", err)
	}
	// Private drives may share a name.
	for i := 0; i < 2; i++ {
		if _, err := e.fs.CreateDrive(ctx, fsops.CreateDriveParams{TenantID: a.tenantID, OwnerID: a.userID, Name: "My Drive", Type: fsops.DriveTypePrivate}); err != nil {
			t.Errorf("private drive %d: %v", i, err)
		}
	}
}

func TestDiscardNewDrive(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	fresh, err := e.fs.CreateDrive(ctx, fsops.CreateDriveParams{TenantID: w.tenantID, Name: "Scratch", Type: fsops.DriveTypeShared})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.fs.DiscardNewDrive(ctx, w.tenantID, fresh.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.db.Drive.Query().Count(ctx); n != 1 { // only the seeded private drive remains
		t.Fatalf("drives left: %d", n)
	}
	// The name is free again.
	if _, err := e.fs.CreateDrive(ctx, fsops.CreateDriveParams{TenantID: w.tenantID, Name: "Scratch", Type: fsops.DriveTypeShared}); err != nil {
		t.Fatalf("name must be reusable: %v", err)
	}

	// A drive with content is never discarded.
	e.folder(t, w, w.drive.ID, "x")
	if err := e.fs.DiscardNewDrive(ctx, w.tenantID, w.drive.ID); !errors.Is(err, fsops.ErrInvalid) {
		t.Fatalf("a non-empty drive must be refused: %v", err)
	}
}
