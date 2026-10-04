package fsops_test

import (
	"context"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/fsops"
)

// publish makes an item readable by anyone with its link.
func (e *env) publish(t *testing.T, w world, itemID string, role authz.Role, noDownload bool) {
	t.Helper()
	err := e.az.SetGeneralAccess(context.Background(), w.p, authz.GeneralAccessInput{
		ItemID: itemID, Level: authz.AccessPublic, Role: role, NoDownload: noDownload,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAnonymousReadsPublicItems(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	deep := e.folder(t, s.w, s.sub, "deep").ID
	leaf := e.file(t, s.w, deep, "leaf.txt", "aabb")
	e.publish(t, s.w, s.docs, authz.RoleViewer, false)
	anon := authz.Anonymous()

	if item, err := e.fs.GetItem(ctx, anon, s.file); err != nil || item.Name != "a.txt" {
		t.Fatalf("GetItem: %+v %v", item, err)
	}
	items, err := e.fs.GetFolderContents(ctx, anon, s.docs, 10, "")
	if err != nil || len(items) != 2 {
		t.Fatalf("GetFolderContents: %d items, %v", len(items), err)
	}
	if n, err := e.fs.GetFolderContentsTotalCount(ctx, anon, s.docs); err != nil || n != 2 {
		t.Fatalf("count: %d %v", n, err)
	}
	if f, err := e.fs.GetFileForDownload(ctx, anon, s.file); err != nil || len(f.InlineChunks) != 1 {
		t.Fatalf("download: %+v %v", f, err)
	}

	// Breadcrumbs start at the published folder, never above it.
	path, err := e.fs.GetItemPath(ctx, anon, leaf)
	if err != nil || len(path) == 0 || path[0].Name != "docs" {
		t.Fatalf("breadcrumbs = %+v %v", path, err)
	}

	// Anything outside the published subtree stays invisible.
	for _, id := range []string{s.prv, s.open, s.w.drive.ID} {
		if _, err := e.fs.GetItem(ctx, anon, id); !isNotFound(err) {
			t.Errorf("%s must stay private: %v", id, err)
		}
	}
	// And it is read-only, whatever the role says.
	if _, err := e.fs.CreateFolder(ctx, anon, s.docs, "x"); !isForbidden(err) {
		t.Errorf("anonymous writes: %v", err)
	}
}

func TestAnonymousViewOnlyLinkCannotDownload(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.publish(t, s.w, s.docs, authz.RoleViewer, true)

	if _, err := e.fs.GetFile(ctx, authz.Anonymous(), s.file); err != nil {
		t.Errorf("a view-only link can still show the file: %v", err)
	}
	if _, err := e.fs.GetFileForDownload(ctx, authz.Anonymous(), s.file); !isForbidden(err) {
		t.Errorf("download must be refused: %v", err)
	}
}

func TestAnonymousCannotReachAnotherTenantsItemsByGuessing(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := newScene(t, e)
	b := e.newWorld(t, "other")
	secret := e.folder(t, b, b.drive.ID, "secret").ID
	e.publish(t, a.w, a.docs, authz.RoleViewer, false)

	// Only tenant A's published subtree opens; tenant B's items do not.
	if _, err := e.fs.GetItem(ctx, authz.Anonymous(), a.docs); err != nil {
		t.Fatal(err)
	}
	if _, err := e.fs.GetItem(ctx, authz.Anonymous(), secret); !isNotFound(err) {
		t.Fatalf("got %v", err)
	}
}

func TestRecordsCarryTheCallersCapabilities(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleContributor)
	contributor, _ := authz.RoleContributor.Caps()

	item, err := e.fs.GetItem(ctx, s.bob, s.file)
	if err != nil || item.Caps != contributor {
		t.Fatalf("GetItem caps = %s, %v", item.Caps, err)
	}
	items, _ := e.fs.GetFolderContents(ctx, s.bob, s.docs, 10, "")
	for _, it := range items {
		if it.Caps != contributor {
			t.Errorf("%s caps = %s", it.Name, it.Caps)
		}
	}
	if f, _ := e.fs.GetFile(ctx, s.bob, s.file); f.Caps != contributor {
		t.Errorf("file caps = %s", f.Caps)
	}
	if created, _ := e.fs.CreateFolder(ctx, s.bob, s.docs, "n"); created.Caps != contributor {
		t.Errorf("new folder caps = %s", created.Caps)
	}
	if mine, _ := e.fs.GetItem(ctx, s.w.p, s.file); mine.Caps != authz.AllCaps {
		t.Errorf("owner caps = %s", mine.Caps)
	}
	path, _ := e.fs.GetItemPath(ctx, s.bob, e.file(t, s.w, s.sub, "deep.txt"))
	if len(path) == 0 || path[len(path)-1].Caps != contributor {
		t.Errorf("breadcrumb caps: %+v", path)
	}
}

func TestGetItemsBatch(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleViewer)

	got, err := e.fs.GetItems(ctx, s.bob, []string{s.sub, s.prv, "missing", s.file, s.docs})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range got {
		ids = append(ids, it.ID)
	}
	want := []string{s.sub, s.file, s.docs} // invisible ones skipped, order kept
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
	if out, err := e.fs.GetItems(ctx, s.bob, nil); err != nil || out != nil {
		t.Errorf("empty input: %v %v", out, err)
	}
	_ = fsops.KindFolder
}
