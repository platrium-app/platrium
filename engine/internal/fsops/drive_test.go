package fsops_test

import (
	"context"
	"testing"

	"platrium/internal/fsops"
	"platrium/internal/infra/db/ent"
)

func TestCreateDriveCreatesRootFolder(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	root, err := e.fs.GetItem(ctx, w.tenantID, w.drive.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !root.IsFolder() || root.ParentID != nil || root.Name != "My Drive" {
		t.Fatalf("unexpected root: %+v", root)
	}
	if w.drive.StorageUsed != 0 || w.drive.StorageQuota != 0 || w.drive.Type != fsops.DriveTypePrivate {
		t.Fatalf("unexpected drive: %+v", w.drive)
	}

	drives, err := e.fs.GetUserDrives(ctx, w.tenantID, w.userID)
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

	if drives, _ := e.fs.GetUserDrives(ctx, a.tenantID, b.userID); len(drives) != 0 {
		t.Fatalf("another tenant's user must have no drives here, got %+v", drives)
	}
}
