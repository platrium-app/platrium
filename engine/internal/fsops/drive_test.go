package fsops_test

import (
	"context"
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
		p.Name = "x"
		return e.db.WithTx(ctx, func(tx *ent.Tx) error { _, err := e.fs.CreateDriveTx(ctx, tx, p); return err })
	}
	cases := []struct {
		name string
		p    fsops.CreateDriveParams
		ok   bool
	}{
		{"tenant-owned shared drive", fsops.CreateDriveParams{OwnerType: fsops.DriveOwnedByTenant, Type: fsops.DriveTypeShared}, true},
		{"tenant-owned with an owner user", fsops.CreateDriveParams{OwnerType: fsops.DriveOwnedByTenant, OwnerID: w.userID, Type: fsops.DriveTypeShared}, false},
		{"tenant-owned private drive", fsops.CreateDriveParams{OwnerType: fsops.DriveOwnedByTenant, Type: fsops.DriveTypePrivate}, false},
		{"user-owned without an owner", fsops.CreateDriveParams{OwnerType: fsops.DriveOwnedByUser, Type: fsops.DriveTypePrivate}, false},
		{"unknown owner type", fsops.CreateDriveParams{OwnerType: "ROBOT", Type: fsops.DriveTypeShared}, false},
		{"user-owned shared drive", fsops.CreateDriveParams{OwnerID: w.userID, Type: fsops.DriveTypeShared}, true},
	}
	for _, c := range cases {
		if err := create(c.p); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok = %v", c.name, err, c.ok)
		}
	}
}
