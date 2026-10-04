package fsops_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/fsops"
)

func TestCreateAndGetFile(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	id := e.file(t, w, w.drive.ID, "a.txt", "aabb", "ccdd")
	got, err := e.fs.GetFile(ctx, w.p, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "a.txt" || got.Size != 3 || got.MimeType != "text/plain" || len(got.InlineChunks) != 2 {
		t.Fatalf("unexpected file: %+v", got)
	}

	empty := e.file(t, w, w.drive.ID, "empty")
	if got, _ := e.fs.GetFile(ctx, w.p, empty); got == nil || got.InlineChunks == nil || len(got.InlineChunks) != 0 {
		t.Fatalf("empty file must keep an empty, non-nil chunk list: %+v", got)
	}

	if _, err := e.fs.GetFile(ctx, w.p, w.drive.ID); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("a folder is not a file: %v", err)
	}
	if _, err := e.fs.CreateFile(ctx, fsops.CreateFileParams{Actor: w.p, ParentID: id, Name: "x"}); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("a file cannot be a parent: %v", err)
	}
	if _, err := e.fs.CreateFile(ctx, fsops.CreateFileParams{Actor: w.p, ParentID: w.drive.ID, Name: "x", HexHashes: []string{"zz"}}); err == nil {
		t.Error("invalid hex hash must fail")
	}
}

func TestFileTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")
	id := e.file(t, a, a.drive.ID, "secret.txt", "aabb")

	if _, err := e.fs.GetFile(ctx, b.p, id); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("GetFile across tenants: %v", err)
	}
	if _, err := e.fs.CreateFile(ctx, fsops.CreateFileParams{Actor: b.p, ParentID: a.drive.ID, Name: "x"}); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("CreateFile under another tenant's folder: %v", err)
	}
	if _, err := e.fs.CopyFile(ctx, b.p, id, b.drive.ID, "stolen"); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("copying another tenant's file: %v", err)
	}
	if _, err := e.fs.CopyFile(ctx, a.p, id, b.drive.ID, "leaked"); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("copying into another tenant's folder: %v", err)
	}
}

func TestCopyFile(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	dest := e.folder(t, w, w.drive.ID, "dest")
	id := e.file(t, w, w.drive.ID, "a.txt", "aabb")

	cp, err := e.fs.CopyFile(ctx, w.p, id, dest.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if cp.ID == id || cp.Name != "a.txt" || cp.Size != 3 || len(cp.InlineChunks) != 1 {
		t.Fatalf("unexpected copy: %+v", cp)
	}
	if item, _ := e.fs.GetItem(ctx, w.p, cp.ID); item == nil || item.ParentID == nil || *item.ParentID != dest.ID {
		t.Fatalf("copy not under destination: %+v", item)
	}

	renamed, err := e.fs.CopyFile(ctx, w.p, id, dest.ID, "b.txt")
	if err != nil || renamed.Name != "b.txt" {
		t.Fatalf("copy with new name: %+v %v", renamed, err)
	}
	if _, err := e.fs.CopyFile(ctx, w.p, id, id, ""); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("a file is not a valid destination: %v", err)
	}
}
