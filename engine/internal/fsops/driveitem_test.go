package fsops_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"platrium/internal/authz"
	"platrium/internal/fsops"
)

func TestGetItemAndTenantIsolation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.newWorld(t, "acme")
	b := e.newWorld(t, "other")
	f := e.folder(t, a, a.drive.ID, "docs")

	item, err := e.fs.GetItem(ctx, a.p, f.ID)
	if err != nil || item.Name != "docs" || !item.IsFolder() {
		t.Fatalf("GetItem: %+v %v", item, err)
	}
	if _, err := e.fs.GetItem(ctx, b.p, f.ID); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("GetItem across tenants: %v", err)
	}
	if items, err := e.fs.GetFolderContents(ctx, b.p, a.drive.ID, 10, ""); !errors.Is(err, fsops.ErrNotFound) || len(items) != 0 {
		t.Fatalf("listing across tenants: %d items, %v", len(items), err)
	}
	if _, err := e.fs.RenameItem(ctx, b.p, f.ID, "pwned"); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("rename across tenants: %v", err)
	}
	if _, err := e.fs.MoveItem(ctx, b.p, f.ID, b.drive.ID); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("move across tenants: %v", err)
	}
	if _, err := e.fs.MoveItem(ctx, a.p, f.ID, b.drive.ID); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("move into another tenant's folder: %v", err)
	}
	if path, err := e.fs.GetItemPath(ctx, b.p, f.ID); !errors.Is(err, fsops.ErrNotFound) || len(path) != 0 {
		t.Fatalf("breadcrumbs across tenants: %+v %v", path, err)
	}
}

func TestGetItemPath(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	a := e.folder(t, w, w.drive.ID, "a")
	b := e.folder(t, w, a.ID, "b")
	c := e.folder(t, w, b.ID, "c")
	file := e.file(t, w, c.ID, "f.txt")

	path, err := e.fs.GetItemPath(ctx, w.p, file)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range path {
		names = append(names, f.Name)
	}
	want := []string{"My Drive", "a", "b", "c"} // root first, item excluded
	if len(names) != len(want) {
		t.Fatalf("path = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("path = %v, want %v", names, want)
		}
	}

	if path, err := e.fs.GetItemPath(ctx, w.p, w.drive.ID); err != nil || len(path) != 0 {
		t.Fatalf("root has no ancestors: %+v %v", path, err)
	}
	if _, err := e.fs.GetItemPath(ctx, w.p, "missing"); !errors.Is(err, fsops.ErrNotFound) {
		t.Fatalf("missing item: %v", err)
	}
}

func TestFolderContentsPagination(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	for i := 0; i < 5; i++ {
		e.folder(t, w, w.drive.ID, "f")
	}

	total, err := e.fs.GetFolderContentsTotalCount(ctx, w.p, w.drive.ID)
	if err != nil || total != 5 {
		t.Fatalf("total = %d, %v", total, err)
	}

	// Walking the cursor must visit every child exactly once. The ordering is
	// the database's (binary) order, so assert completeness, not Go's string order.
	want := map[string]bool{}
	items, err := e.fs.GetFolderContents(ctx, w.p, w.drive.ID, 100, "")
	if err != nil || len(items) != 5 {
		t.Fatalf("full listing: %d items, %v", len(items), err)
	}
	for _, it := range items {
		want[it.ID] = true
	}

	seen := map[string]bool{}
	after := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("cursor does not advance")
		}
		page, err := e.fs.GetFolderContents(ctx, w.p, w.drive.ID, 2, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, it := range page {
			if seen[it.ID] {
				t.Fatalf("item %s returned twice", it.ID)
			}
			seen[it.ID] = true
		}
		after = page[len(page)-1].ID
	}
	if len(seen) != len(want) {
		t.Fatalf("paged %d items, want %d", len(seen), len(want))
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("item %s was skipped by pagination", id)
		}
	}
}

func TestRenameItemBumpsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	f := e.folder(t, w, w.drive.ID, "old")

	renamed, err := e.fs.RenameItem(ctx, w.p, f.ID, "new")
	if err != nil || renamed.Name != "new" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if !renamed.UpdatedAt.After(f.UpdatedAt) {
		t.Fatalf("updated_at not bumped: %v -> %v", f.UpdatedAt, renamed.UpdatedAt)
	}
	if _, err := e.fs.RenameItem(ctx, w.p, f.ID, ""); !errors.Is(err, fsops.ErrInvalid) {
		t.Errorf("empty name: %v", err)
	}
	if _, err := e.fs.RenameItem(ctx, w.p, "missing", "x"); !errors.Is(err, fsops.ErrNotFound) {
		t.Errorf("missing item: %v", err)
	}
}

func TestMoveItem(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	a := e.folder(t, w, w.drive.ID, "a")
	b := e.folder(t, w, w.drive.ID, "b")
	child := e.folder(t, w, a.ID, "child")
	file := e.file(t, w, a.ID, "f.txt")

	moved, err := e.fs.MoveItem(ctx, w.p, child.ID, b.ID)
	if err != nil || moved.ParentID == nil || *moved.ParentID != b.ID {
		t.Fatalf("move folder: %+v %v", moved, err)
	}
	if !moved.UpdatedAt.After(child.UpdatedAt) {
		t.Errorf("updated_at not bumped by move")
	}
	if moved, err := e.fs.MoveItem(ctx, w.p, file, b.ID); err != nil || *moved.ParentID != b.ID {
		t.Fatalf("move file: %+v %v", moved, err)
	}

	cases := []struct {
		name         string
		item, parent string
		want         error
	}{
		{"into itself", a.ID, a.ID, fsops.ErrInvalid},
		{"into own descendant", b.ID, child.ID, fsops.ErrInvalid},
		{"drive root", w.drive.ID, a.ID, fsops.ErrInvalid},
		{"into a file", a.ID, file, fsops.ErrInvalid},
		{"missing item", "missing", a.ID, fsops.ErrNotFound},
		{"missing destination", a.ID, "missing", fsops.ErrNotFound},
	}
	for _, c := range cases {
		if _, err := e.fs.MoveItem(ctx, w.p, c.item, c.parent); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestMoveItemAcrossDrivesIsRejected(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	f := e.folder(t, w, w.drive.ID, "a")

	second, err := e.fs.CreateDrive(ctx, fsops.CreateDriveParams{TenantID: w.tenantID, Name: "Team", Type: fsops.DriveTypeShared})
	if err != nil {
		t.Fatal(err)
	}
	// Give alice access to the second drive so the refusal is about drives, not visibility.
	adminCaps, _ := authz.RoleDriveAdmin.Caps()
	if err := e.db.Grant.Create().SetTenantID(w.tenantID).SetDriveID(second.ID).SetResourceID(second.ID).
		SetSubjectType("USER").SetSubjectID(w.userID).SetRole("DRIVE_ADMIN").SetCaps(int64(adminCaps)).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := e.fs.MoveItem(ctx, w.p, f.ID, second.ID); !errors.Is(err, fsops.ErrInvalid) {
		t.Fatalf("cross-drive move: %v", err)
	}
}

// Two moves that are each valid alone but form a cycle together (A into B
// while B into A) must never both succeed.
func TestMoveItemConcurrentCycles(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")

	for round := 0; round < 15; round++ {
		a := e.folder(t, w, w.drive.ID, "a")
		b := e.folder(t, w, w.drive.ID, "b")

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); _, errs[0] = e.fs.MoveItem(ctx, w.p, a.ID, b.ID) }()
		go func() { defer wg.Done(); _, errs[1] = e.fs.MoveItem(ctx, w.p, b.ID, a.ID) }()
		wg.Wait()

		if errs[0] == nil && errs[1] == nil {
			t.Fatalf("round %d: both moves succeeded, creating a cycle", round)
		}
		// Whatever happened, both folders still reach the drive root.
		for _, id := range []string{a.ID, b.ID} {
			path, err := e.fs.GetItemPath(ctx, w.p, id)
			if err != nil || len(path) == 0 || path[0].ID != w.drive.ID {
				t.Fatalf("round %d: %s does not reach the root: %+v %v", round, id, path, err)
			}
		}
	}
}

func TestGetFolderChanges(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	w := e.newWorld(t, "acme")
	old := e.folder(t, w, w.drive.ID, "old")
	time.Sleep(10 * time.Millisecond)
	since := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	fresh := e.folder(t, w, w.drive.ID, "fresh")

	changes, err := e.fs.GetFolderChanges(ctx, w.p, w.drive.ID, since)
	if err != nil || len(changes) != 1 || changes[0].ID != fresh.ID {
		t.Fatalf("changes since: %+v %v", changes, err)
	}

	// A rename is a change, which the graph version silently missed.
	time.Sleep(10 * time.Millisecond)
	if _, err := e.fs.RenameItem(ctx, w.p, old.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	changes, _ = e.fs.GetFolderChanges(ctx, w.p, w.drive.ID, since)
	if len(changes) != 2 {
		t.Fatalf("rename must appear in the change feed, got %d changes", len(changes))
	}
}
