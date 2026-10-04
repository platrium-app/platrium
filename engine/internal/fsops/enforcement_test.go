package fsops_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/infra/db/ent"
)

// scene is a drive owned by alice:
//
//	root
//	├── docs        (shared with bob in these tests)
//	│   ├── a.txt
//	│   ├── sub
//	│   └── secret  (restricted in some tests)
//	├── open        (a second, separate folder)
//	└── private
type scene struct {
	w                          world
	bob, carol                 authz.Principal
	docs, file, sub, open, prv string
}

func newScene(t *testing.T, e *env) scene {
	t.Helper()
	var s scene
	s.w = e.newWorld(t, "acme")
	s.bob, s.carol = e.member(t, s.w, "bob"), e.member(t, s.w, "carol")
	s.docs = e.folder(t, s.w, s.w.drive.ID, "docs").ID
	s.file = e.file(t, s.w, s.docs, "a.txt", "aabb")
	s.sub = e.folder(t, s.w, s.docs, "sub").ID
	s.open = e.folder(t, s.w, s.w.drive.ID, "open").ID
	s.prv = e.folder(t, s.w, s.w.drive.ID, "private").ID
	return s
}

func isForbidden(err error) bool { return errors.Is(err, fsops.ErrForbidden) }
func isNotFound(err error) bool  { return errors.Is(err, fsops.ErrNotFound) }

func TestViewerCanReadButNotWrite(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleViewer)

	if _, err := e.fs.GetItem(ctx, s.bob, s.file); err != nil {
		t.Errorf("GetItem: %v", err)
	}
	if items, err := e.fs.GetFolderContents(ctx, s.bob, s.docs, 10, ""); err != nil || len(items) != 2 {
		t.Errorf("GetFolderContents: %d items, %v", len(items), err)
	}
	if n, err := e.fs.GetFolderContentsTotalCount(ctx, s.bob, s.docs); err != nil || n != 2 {
		t.Errorf("GetFolderContentsTotalCount: %d, %v", n, err)
	}
	if _, err := e.fs.GetFile(ctx, s.bob, s.file); err != nil {
		t.Errorf("GetFile: %v", err)
	}
	if _, err := e.fs.GetFileForDownload(ctx, s.bob, s.file); err != nil {
		t.Errorf("download: %v", err)
	}

	denied := map[string]error{}
	_, denied["CreateFolder"] = e.fs.CreateFolder(ctx, s.bob, s.docs, "x")
	_, denied["CreateFile"] = e.fs.CreateFile(ctx, fsops.CreateFileParams{Actor: s.bob, ParentID: s.docs, Name: "x"})
	_, denied["Rename"] = e.fs.RenameItem(ctx, s.bob, s.file, "x")
	_, denied["Move"] = e.fs.MoveItem(ctx, s.bob, s.file, s.sub)
	_, denied["Copy"] = e.fs.CopyFile(ctx, s.bob, s.file, s.sub, "")
	for op, err := range denied {
		if !isForbidden(err) {
			t.Errorf("%s as a viewer: got %v, want forbidden", op, err)
		}
	}
	// Nothing was written by the refused calls.
	if n, _ := e.fs.GetFolderContentsTotalCount(ctx, s.w.p, s.docs); n != 2 {
		t.Errorf("refused writes must change nothing, docs has %d children", n)
	}
}

func TestViewOnlyShareCannotDownloadOrCopy(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.shareOpts(t, s.w, s.docs, s.bob, authz.RoleViewer, true)
	e.share(t, s.w, s.open, s.bob, authz.RoleContributor) // somewhere to copy to

	if _, err := e.fs.GetFile(ctx, s.bob, s.file); err != nil {
		t.Errorf("a view-only share can still see the file: %v", err)
	}
	if _, err := e.fs.GetFileForDownload(ctx, s.bob, s.file); !isForbidden(err) {
		t.Errorf("download must be refused: %v", err)
	}
	if _, err := e.fs.CopyFile(ctx, s.bob, s.file, s.open, ""); !isForbidden(err) {
		t.Errorf("copying exposes the content and must be refused: %v", err)
	}
}

func TestContributorCreatesAndEditsButCannotMoveOrCopyOut(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleContributor)

	if _, err := e.fs.CreateFolder(ctx, s.bob, s.docs, "new"); err != nil {
		t.Errorf("CreateFolder: %v", err)
	}
	if _, err := e.fs.CreateFile(ctx, fsops.CreateFileParams{Actor: s.bob, ParentID: s.docs, Name: "b.txt"}); err != nil {
		t.Errorf("CreateFile: %v", err)
	}
	if _, err := e.fs.RenameItem(ctx, s.bob, s.file, "renamed.txt"); err != nil {
		t.Errorf("Rename: %v", err)
	}
	if _, err := e.fs.MoveItem(ctx, s.bob, s.file, s.sub); !isForbidden(err) {
		t.Errorf("a contributor cannot move: %v", err)
	}
	// Copy within the folder works (download + create), and the copy lands there.
	if cp, err := e.fs.CopyFile(ctx, s.bob, s.file, s.sub, "copy.txt"); err != nil || cp.Name != "copy.txt" {
		t.Errorf("copy inside the shared folder: %+v %v", cp, err)
	}
	// ...but not into a folder bob cannot write to.
	if _, err := e.fs.CopyFile(ctx, s.bob, s.file, s.open, ""); !isNotFound(err) {
		t.Errorf("copy to an invisible folder: %v", err)
	}
}

func TestContentManagerMovesNeedsCreateOnDestination(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleContentManager)
	e.share(t, s.w, s.open, s.bob, authz.RoleViewer) // can see, cannot write

	if got, err := e.fs.MoveItem(ctx, s.bob, s.file, s.sub); err != nil || got.ParentID == nil || *got.ParentID != s.sub {
		t.Fatalf("move inside the shared folder: %+v %v", got, err)
	}
	if _, err := e.fs.MoveItem(ctx, s.bob, s.sub, s.open); !isForbidden(err) {
		t.Errorf("moving into a read-only folder must be refused: %v", err)
	}
	if _, err := e.fs.MoveItem(ctx, s.bob, s.sub, s.w.drive.ID); !isNotFound(err) {
		t.Errorf("moving to a folder bob cannot see must read as not found: %v", err)
	}
}

func TestInvisibleLooksLikeMissing(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleManager)

	// Carol has no access to anything: every operation reads as "not found",
	// exactly as it would for an ID that does not exist.
	for _, id := range []string{s.docs, s.file, "does-not-exist"} {
		_, e1 := e.fs.GetItem(ctx, s.carol, id)
		_, e2 := e.fs.GetFolderContents(ctx, s.carol, id, 10, "")
		_, e3 := e.fs.CreateFolder(ctx, s.carol, id, "x")
		_, e4 := e.fs.RenameItem(ctx, s.carol, id, "x")
		_, e5 := e.fs.GetItemPath(ctx, s.carol, id)
		_, e6 := e.fs.GetFile(ctx, s.carol, id)
		for i, err := range []error{e1, e2, e3, e4, e5, e6} {
			if !isNotFound(err) {
				t.Errorf("op %d on %s: got %v, want not found", i, id, err)
			}
		}
	}
	// A folder bob manages is not enough to reach the private sibling.
	if _, err := e.fs.GetItem(ctx, s.bob, s.prv); !isNotFound(err) {
		t.Errorf("sibling of a shared folder: %v", err)
	}
}

func TestRevocationIsImmediate(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	g := e.shareOpts(t, s.w, s.docs, s.bob, authz.RoleViewer, false)
	if _, err := e.fs.GetItem(ctx, s.bob, s.file); err != nil {
		t.Fatal(err)
	}

	if err := e.az.Revoke(ctx, s.w.p, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.fs.GetItem(ctx, s.bob, s.file); !isNotFound(err) {
		t.Fatalf("the very next request must be refused: %v", err)
	}
}

func TestAnonymousAndUnsignedCallersAreRefused(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	for _, p := range []authz.Principal{authz.Anonymous(), {TenantID: s.w.tenantID}} {
		_, e1 := e.fs.GetItem(ctx, p, s.docs)
		_, e2 := e.fs.GetFolderContents(ctx, p, s.docs, 10, "")
		_, e3 := e.fs.CreateFolder(ctx, p, s.docs, "x")
		_, e4 := e.fs.GetUserDrives(ctx, p)
		_, e5 := e.fs.MoveItem(ctx, p, s.file, s.sub)
		for i, err := range []error{e1, e2, e3, e4, e5} {
			if !isForbidden(err) {
				t.Errorf("op %d for %+v: got %v, want forbidden", i, p, err)
			}
		}
	}
}

// A forbidden upload must be refused before anything is written, including the
// chunk manifest in the KV store. The test environment has no manifest store,
// so reaching it would panic.
func TestRefusedUploadWritesNothing(t *testing.T) {
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleViewer)

	manyChunks := []string{"aa", "bb", "cc", "dd", "ee"} // more than the inline limit
	_, err := e.fs.CreateFile(context.Background(), fsops.CreateFileParams{Actor: s.bob, ParentID: s.docs, Name: "big", HexHashes: manyChunks})
	if !isForbidden(err) {
		t.Fatalf("got %v", err)
	}
}

func TestRestrictedChildrenAreHiddenFromListings(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleViewer)

	// Interleave visible and restricted children so pages have to be filled
	// from several reads.
	var visible []string
	var restricted []string
	for i := 0; i < 6; i++ {
		visible = append(visible, e.folder(t, s.w, s.docs, fmt.Sprintf("v%d", i)).ID)
		r := e.folder(t, s.w, s.docs, fmt.Sprintf("r%d", i)).ID
		if err := e.az.SetInheritance(ctx, s.w.p, r, false); err != nil {
			t.Fatal(err)
		}
		restricted = append(restricted, r)
	}
	// Bob has a direct grant on one restricted child, so he sees that one.
	e.share(t, s.w, restricted[0], s.bob, authz.RoleViewer)

	want := map[string]bool{s.file: true, s.sub: true, restricted[0]: true}
	for _, id := range visible {
		want[id] = true
	}

	total, err := e.fs.GetFolderContentsTotalCount(ctx, s.bob, s.docs)
	if err != nil || total != len(want) {
		t.Fatalf("count = %d, want %d (%v)", total, len(want), err)
	}

	seen := map[string]bool{}
	after := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("the cursor does not advance")
		}
		page, err := e.fs.GetFolderContents(ctx, s.bob, s.docs, 4, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 4 {
			t.Fatalf("page of %d exceeds the limit", len(page))
		}
		for _, it := range page {
			if !want[it.ID] {
				t.Fatalf("%s must be hidden from bob", it.Name)
			}
			seen[it.ID] = true
		}
		after = page[len(page)-1].ID
	}
	if len(seen) != len(want) {
		t.Fatalf("paged %d items, want %d", len(seen), len(want))
	}

	// The owner sees everything, and a hidden item cannot be fetched directly.
	if n, _ := e.fs.GetFolderContentsTotalCount(ctx, s.w.p, s.docs); n != len(want)+len(restricted)-1 {
		t.Errorf("owner count = %d", n)
	}
	if _, err := e.fs.GetItem(ctx, s.bob, restricted[1]); !isNotFound(err) {
		t.Errorf("a restricted item must read as missing: %v", err)
	}
}

func TestChangeFeedHidesRestrictedItems(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	e.share(t, s.w, s.docs, s.bob, authz.RoleViewer)
	since := time.Now().UTC().Add(-time.Minute)

	secret := e.folder(t, s.w, s.docs, "secret").ID
	if err := e.az.SetInheritance(ctx, s.w.p, secret, false); err != nil {
		t.Fatal(err)
	}

	changes, err := e.fs.GetFolderChanges(ctx, s.bob, s.docs, since)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.ID == secret {
			t.Fatal("a restricted item leaked through the change feed")
		}
	}
	if len(changes) == 0 {
		t.Fatal("visible changes must still appear")
	}
}

func TestBreadcrumbsStartAtTheSharedFolder(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	deep := e.folder(t, s.w, s.sub, "deep").ID
	leaf := e.file(t, s.w, deep, "leaf.txt")

	// Bob is shared "sub" only: he must not learn the names of "docs" or the drive above it.
	e.share(t, s.w, s.sub, s.bob, authz.RoleViewer)
	path, err := e.fs.GetItemPath(ctx, s.bob, leaf)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range path {
		names = append(names, f.Name)
	}
	if len(names) != 2 || names[0] != "sub" || names[1] != "deep" {
		t.Fatalf("breadcrumbs = %v, want [sub deep]", names)
	}

	full, _ := e.fs.GetItemPath(ctx, s.w.p, leaf)
	if len(full) != 4 {
		t.Fatalf("the owner sees the whole path, got %d folders", len(full))
	}
}

func TestSharedDrivesOwnedByTheTenant(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)

	var shared *fsops.Drive
	err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		shared, err = e.fs.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{
			TenantID: s.w.tenantID, OwnerType: fsops.DriveOwnedByTenant, Name: "Company", Type: fsops.DriveTypeShared,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if shared.OwnerType != fsops.DriveOwnedByTenant || shared.OwnerID != "" {
		t.Fatalf("unexpected drive: %+v", shared)
	}

	// Nobody owns it implicitly: not even the user who is an admin elsewhere.
	if drives, _ := e.fs.GetUserDrives(ctx, s.w.p); len(drives) != 1 {
		t.Fatalf("alice sees only her own drive, got %+v", drives)
	}
	if _, err := e.fs.GetItem(ctx, s.w.p, shared.ID); !isNotFound(err) {
		t.Fatalf("a tenant-owned drive grants no implicit access: %v", err)
	}

	// Access comes from grants. Bootstrap a manager (no one can share yet).
	managerCaps, _ := authz.RoleManager.Caps()
	if err := e.db.Grant.Create().SetTenantID(s.w.tenantID).SetDriveID(shared.ID).SetResourceID(shared.ID).
		SetSubjectType("USER").SetSubjectID(s.w.userID).SetRole("MANAGER").SetCaps(int64(managerCaps)).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, s.w.p); len(drives) != 2 {
		t.Fatalf("alice now sees the shared drive, got %+v", drives)
	}
	if _, err := e.fs.CreateFolder(ctx, s.w.p, shared.ID, "Finance"); err != nil {
		t.Fatalf("a manager can build inside it: %v", err)
	}

	// The manager shares it with bob as a viewer; carol, who has nothing, sees nothing.
	if _, err := e.az.Grant(ctx, s.w.p, authz.GrantInput{ItemID: shared.ID, Subject: authz.Subject{Type: authz.SubjectUser, ID: s.bob.UserID}, Role: authz.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, s.bob); len(drives) != 1 || drives[0].ID != shared.ID {
		t.Fatalf("bob sees the shared drive: %+v", drives)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, s.carol); len(drives) != 0 {
		t.Fatalf("carol has no drive and no access, got %+v", drives)
	}
}
