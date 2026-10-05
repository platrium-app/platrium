package fsops_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/fsops"
)

func TestCreateFolder(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	f := e.folder(t, w, w.drive.ID, "docs")
	if f.ParentID == nil || *f.ParentID != w.drive.ID || f.TenantID != w.tenantID {
		t.Fatalf("unexpected folder: %+v", f)
	}
	sub := e.folder(t, w, f.ID, "sub")
	if item, err := e.fs.GetItem(ctx, w.p, sub.ID); err != nil || !item.IsFolder() {
		t.Fatalf("subfolder: %+v %v", item, err)
	}

	if _, err := e.fs.CreateFolder(ctx, w.p, w.drive.ID, ""); !errors.Is(err, fsops.ErrInvalid) {
		t.Errorf("empty name: %v", err)
	}
	if _, err := e.fs.CreateFolder(ctx, w.p, "missing", "x"); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("missing parent: %v", err)
	}
	id := e.file(t, w, w.drive.ID, "a.txt")
	if _, err := e.fs.CreateFolder(ctx, w.p, id, "x"); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("a file cannot be a parent: %v", err)
	}
}

func TestCreateFolderTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")

	if _, err := e.fs.CreateFolder(ctx, b.p, a.drive.ID, "intruder"); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("creating under another tenant's folder must fail, got %v", err)
	}
	if n, _ := e.fs.GetFolderContentsTotalCount(ctx, a.p, a.drive.ID); n != 0 {
		t.Fatalf("victim folder was modified: %d children", n)
	}
}
