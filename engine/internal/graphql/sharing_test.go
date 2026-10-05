package graphql

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"platrium/internal/auth/actor"
	"platrium/internal/auth/session"
	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/notifications"
	"platrium/internal/orchestrator"
)

// harness is a resolver wired to a real database, with alice owning a drive
// that holds docs/a.txt, plus bob and carol in the same tenant.
type harness struct {
	db                *db.DB
	r                 *Resolver
	tenant            string
	alice, bob, carol string
	admin             string // a tenant administrator
	drive, docs, file string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	az := sqlauthz.New(d)
	fs := fsops.NewFSOps(d, nil, az)

	broker := notifications.NewBroker()
	h := &harness{db: d, r: &Resolver{
		DriveOrch:   orchestrator.NewDriveOrchestrator(d, fs, az, identity.NewUserStore(d), identity.NewPolicyStore(d)),
		FSOps:       fs,
		Broker:      broker,
		Authz:       az,
		UserStore:   identity.NewUserStore(d),
		GroupStore:  identity.NewGroupStore(d),
		TenantStore: identity.NewTenantStore(d),
	}}

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		tn, err := tx.Tenant.Create().SetAlias("acme").SetName("Acme Inc").Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
		if err != nil {
			return err
		}
		mk := func(name string) (string, error) {
			role := identity.RoleMember
			if name == "ada" {
				role = identity.RoleAdmin
			}
			u, err := tx.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID(name).
				SetEmail(name + "@acme.com").SetDisplayName(name).SetRole(role).Save(ctx)
			if err != nil {
				return "", err
			}
			return u.ID, nil
		}
		h.tenant = tn.ID
		if h.alice, err = mk("alice"); err != nil {
			return err
		}
		if h.bob, err = mk("bob"); err != nil {
			return err
		}
		if h.carol, err = mk("carol"); err != nil {
			return err
		}
		if h.admin, err = mk("ada"); err != nil {
			return err
		}
		dr, err := fs.CreateDriveTx(ctx, tx, fsops.CreateDriveParams{TenantID: tn.ID, OwnerID: h.alice, Name: "Alice", Type: fsops.DriveTypePrivate})
		h.drive = dr.ID
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	alice, err := az.Principal(ctx, h.tenant, h.alice)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := fs.CreateFolder(ctx, alice, h.drive, "docs")
	if err != nil {
		t.Fatal(err)
	}
	h.docs = docs.ID
	if h.file, err = fs.CreateFile(ctx, fsops.CreateFileParams{Actor: alice, ParentID: h.docs, Name: "a.txt", Size: 3, MimeType: "text/plain", HexHashes: []string{"aabb"}}); err != nil {
		t.Fatal(err)
	}
	return h
}

// as returns a context signed in as the given user.
func (h *harness) as(userID string) context.Context {
	return session.WithSession(context.Background(), &session.PlatriumSession{UserID: userID, TenantID: h.tenant})
}

// anonymous returns a context with no session.
func anonymous() context.Context { return context.Background() }

func (h *harness) m() *mutationResolver { return &mutationResolver{h.r} }
func (h *harness) q() *queryResolver    { return &queryResolver{h.r} }

func ptr[T any](v T) *T { return &v }

func code(err error) string {
	g := ErrorPresenter(context.Background(), err)
	if c, ok := g.Extensions["code"].(string); ok {
		return c
	}
	return ""
}

func TestShareAndInspectAccess(t *testing.T) {
	h := newHarness(t)

	grant, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "user", SubjectID: h.bob, Role: "viewer", NoDownload: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if grant.SubjectType != "USER" || grant.SubjectName != "bob" || grant.Role != "VIEWER" || !grant.NoDownload || slices.Contains(grant.Capabilities, "DOWNLOAD") {
		t.Fatalf("unexpected grant: %+v", grant)
	}

	access, err := h.q().ItemAccess(h.as(h.alice), h.docs)
	if err != nil {
		t.Fatal(err)
	}
	if !access.InheritsPermissions || access.GeneralAccess.Level != "RESTRICTED" || len(access.Grants) != 1 || access.Grants[0].ID != grant.ID {
		t.Fatalf("unexpected access: %+v", access)
	}

	// Bob can now open it, and is told what he may do.
	item, err := h.q().Item(h.as(h.bob), h.docs)
	if err != nil {
		t.Fatal(err)
	}
	if caps := item.GetMyCapabilities(); !slices.Contains(caps, "VIEW") || slices.Contains(caps, "EDIT") || slices.Contains(caps, "DOWNLOAD") {
		t.Fatalf("capabilities = %v", caps)
	}

	// ...but cannot see or change who else has access.
	if _, err := h.q().ItemAccess(h.as(h.bob), h.docs); !errors.Is(err, authz.ErrForbidden) || code(err) != "FORBIDDEN" {
		t.Fatalf("a viewer must not see the sharing list: %v", err)
	}
	if _, err := h.m().RevokeAccess(h.as(h.bob), grant.ID); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("a viewer must not revoke: %v", err)
	}

	// Revoking takes effect on the next request.
	if ok, err := h.m().RevokeAccess(h.as(h.alice), grant.ID); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err := h.q().Item(h.as(h.bob), h.docs); code(err) != "NOT_FOUND" {
		t.Fatalf("access must end at once, got %v", err)
	}
}

func TestSharedWithMe(t *testing.T) {
	h := newHarness(t)
	if _, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "FULL_EDITOR"}); err != nil {
		t.Fatal(err)
	}

	conn, err := h.q().SharedWithMe(h.as(h.bob), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(conn.Edges) != 1 || conn.PageInfo.HasNextPage {
		t.Fatalf("shared with bob: %+v", conn)
	}
	e := conn.Edges[0]
	if e.Node.Item.GetID() != h.docs || e.Node.Role != "FULL_EDITOR" || !slices.Contains(e.Node.Capabilities, "MOVE") || slices.Contains(e.Node.Capabilities, "SHARE") || e.Cursor != h.docs {
		t.Fatalf("unexpected edge: %+v", e.Node)
	}

	// Carol, who was given nothing, sees nothing; alice's own drive is not "shared with" her.
	for _, user := range []string{h.carol, h.alice} {
		conn, err := h.q().SharedWithMe(h.as(user), nil, nil)
		if err != nil || len(conn.Edges) != 0 {
			t.Fatalf("%s: %+v %v", user, conn, err)
		}
	}
	if _, err := h.q().SharedWithMe(anonymous(), nil, nil); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Fatalf("sign in required: %v", err)
	}
}

func TestGeneralAccessAndAnonymousVisitors(t *testing.T) {
	h := newHarness(t)

	// Nothing is public by default.
	if _, err := h.q().Item(anonymous(), h.file); code(err) != "NOT_FOUND" {
		t.Fatalf("private items read as missing to visitors: %v", err)
	}

	got, err := h.m().SetGeneralAccess(h.as(h.alice), GeneralAccessInput{ItemID: h.docs, Level: "public", Role: ptr("viewer"), NoDownload: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Level != "PUBLIC" || got.Role == nil || *got.Role != "VIEWER" || !got.NoDownload {
		t.Fatalf("general access = %+v", got)
	}

	// An anonymous visitor can open the folder and its contents, but not download.
	folder, err := h.q().Item(anonymous(), h.docs)
	if err != nil || folder.GetName() != "docs" {
		t.Fatalf("anonymous Item: %v %v", folder, err)
	}
	page, err := h.q().FolderContents(anonymous(), h.docs, nil, nil)
	if err != nil || page.TotalCount != 1 || len(page.Edges) != 1 {
		t.Fatalf("anonymous FolderContents: %+v %v", page, err)
	}
	file := page.Edges[0].Node
	if caps := file.GetMyCapabilities(); !slices.Contains(caps, "VIEW") || slices.Contains(caps, "DOWNLOAD") {
		t.Fatalf("a view-only link: %v", caps)
	}
	path, err := (&fileResolver{h.r}).Path(anonymous(), file.(*File))
	if err != nil || len(path) != 1 || path[0].Name != "docs" {
		t.Fatalf("breadcrumbs start at the shared folder: %+v %v", path, err)
	}

	// Anonymous visitors cannot manage anything, or see anything private.
	if _, err := h.q().ItemAccess(anonymous(), h.docs); !errors.Is(err, actor.ErrUnauthenticated) || code(err) != "UNAUTHENTICATED" {
		t.Fatalf("ItemAccess: %v", err)
	}
	if _, err := h.m().CreateFolder(anonymous(), h.docs, "x"); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Fatalf("writes need a session: %v", err)
	}
	if _, err := h.q().Item(anonymous(), h.drive); code(err) != "NOT_FOUND" {
		t.Fatalf("the rest of the drive stays private: %v", err)
	}

	// The owner sees the setting; turning it off closes the link at once.
	access, _ := h.q().ItemAccess(h.as(h.alice), h.docs)
	if access.GeneralAccess.Level != "PUBLIC" || len(access.Grants) != 0 {
		t.Fatalf("general access is reported separately: %+v", access)
	}
	if _, err := h.m().SetGeneralAccess(h.as(h.alice), GeneralAccessInput{ItemID: h.docs, Level: "RESTRICTED"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.q().Item(anonymous(), h.docs); code(err) != "NOT_FOUND" {
		t.Fatalf("link must stop working: %v", err)
	}
}

func TestOrganizationWideAccessAndExpiry(t *testing.T) {
	h := newHarness(t)
	soon := time.Now().Add(time.Hour)
	got, err := h.m().SetGeneralAccess(h.as(h.alice), GeneralAccessInput{ItemID: h.docs, Level: "TENANT", Role: ptr("FULL_EDITOR"), ExpiresAt: &soon})
	if err != nil {
		t.Fatal(err)
	}
	if got.Level != "TENANT" || got.ExpiresAt == nil {
		t.Fatalf("got %+v", got)
	}
	// Any member of the organization can open it; visitors cannot.
	if _, err := h.q().Item(h.as(h.carol), h.docs); err != nil {
		t.Fatal(err)
	}
	if _, err := h.q().Item(anonymous(), h.docs); code(err) != "NOT_FOUND" {
		t.Fatal(err)
	}

	access, _ := h.q().ItemAccess(h.as(h.alice), h.docs)
	if access.GeneralAccess.Role == nil || *access.GeneralAccess.Role != "FULL_EDITOR" {
		t.Fatalf("access = %+v", access.GeneralAccess)
	}
}

func TestSharingValidationAndErrorCodes(t *testing.T) {
	h := newHarness(t)
	ctx := h.as(h.alice)

	bad := []struct {
		name string
		call func() error
		code string
	}{
		{"unknown role", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "BOSS"})
			return err
		}, "BAD_REQUEST"},
		{"owner cannot be granted", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "OWNER"})
			return err
		}, "BAD_REQUEST"},
		{"drive admin on a file", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "DRIVE_ADMIN"})
			return err
		}, "BAD_REQUEST"},
		{"commenter on a file", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "COMMENTER"})
			return err
		}, "BAD_REQUEST"},
		{"unknown subject type", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "ROBOT", SubjectID: "x", Role: "VIEWER"})
			return err
		}, "BAD_REQUEST"},
		{"missing user", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: "nobody", Role: "VIEWER"})
			return err
		}, "NOT_FOUND"},
		{"missing item", func() error {
			_, err := h.m().ShareItem(ctx, ShareInput{ItemID: "nope", SubjectType: "USER", SubjectID: h.bob, Role: "VIEWER"})
			return err
		}, "NOT_FOUND"},
		{"unknown access level", func() error {
			_, err := h.m().SetGeneralAccess(ctx, GeneralAccessInput{ItemID: h.docs, Level: "WORLD", Role: ptr("VIEWER")})
			return err
		}, "BAD_REQUEST"},
		{"public editor", func() error {
			_, err := h.m().SetGeneralAccess(ctx, GeneralAccessInput{ItemID: h.docs, Level: "PUBLIC", Role: ptr("FULL_EDITOR")})
			return err
		}, "BAD_REQUEST"},
		{"signed out", func() error {
			_, err := h.m().ShareItem(anonymous(), ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "VIEWER"})
			return err
		}, "UNAUTHENTICATED"},
	}
	for _, c := range bad {
		if got := code(c.call()); got != c.code {
			t.Errorf("%s: code = %q, want %q", c.name, got, c.code)
		}
	}
}

func TestSharingWithGroupsAndTheOrganization(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// Group "design" with carol in it.
	cfg, err := h.r.TenantStore.GetPublicTenantAuthConfig(ctx, "acme")
	if err != nil || len(cfg.Providers) == 0 {
		t.Fatal(err)
	}
	groupID := mustGroup(t, h, cfg.Providers[0].ID, "design")
	if err := h.r.Authz.AddMember(ctx, h.tenant, groupID, authz.MemberUser, h.carol); err != nil {
		t.Fatal(err)
	}

	grant, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "GROUP", SubjectID: groupID, Role: "FULL_EDITOR"})
	if err != nil {
		t.Fatal(err)
	}
	if grant.SubjectName != "design" || grant.Role != "FULL_EDITOR" {
		t.Fatalf("grant = %+v", grant)
	}
	if _, err := h.m().CreateFolder(h.as(h.carol), h.docs, "from-carol"); err != nil {
		t.Fatalf("a group member may create: %v", err)
	}

	org, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "TENANT", SubjectID: h.tenant, Role: "VIEWER"})
	if err != nil || org.SubjectName != "Acme Inc" {
		t.Fatalf("org grant: %+v %v", org, err)
	}
}

func TestInheritanceMutation(t *testing.T) {
	h := newHarness(t)
	if _, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "VIEWER"}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.q().Item(h.as(h.bob), h.file); err != nil {
		t.Fatal(err)
	}

	if ok, err := h.m().SetInheritance(h.as(h.alice), h.file, false); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err := h.q().Item(h.as(h.bob), h.file); code(err) != "NOT_FOUND" {
		t.Fatalf("the file is restricted now: %v", err)
	}
	access, _ := h.q().ItemAccess(h.as(h.alice), h.file)
	if access.InheritsPermissions {
		t.Fatal("must report that it no longer inherits")
	}
	if _, err := h.m().SetInheritance(h.as(h.bob), h.file, true); code(err) != "NOT_FOUND" {
		t.Fatalf("bob cannot even see it: %v", err)
	}
}

func TestEveryItemReportsCapabilities(t *testing.T) {
	h := newHarness(t)
	drives, err := h.q().Drives(h.as(h.alice))
	if err != nil || len(drives) != 1 || len(drives[0].MyCapabilities) == 0 {
		t.Fatalf("drive caps: %+v %v", drives, err)
	}
	owner := drives[0].MyCapabilities
	if !slices.Contains(owner, "DELETE_DRIVE") || !slices.Contains(owner, "SHARE") {
		t.Fatalf("an owner holds everything: %v", owner)
	}
}

// mustGroup creates a group directly; there is no API for it yet.
func mustGroup(t *testing.T, h *harness, idpID, name string) string {
	t.Helper()
	g, err := h.db.Group.Create().SetTenantID(h.tenant).SetIdpID(idpID).SetExternalID(name).SetName(name).Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return g.ID
}

func TestShareRolesDependOnTheItem(t *testing.T) {
	h := newHarness(t)

	roles, err := h.q().ShareRoles(h.as(h.alice), h.docs)
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, r := range roles {
		labels = append(labels, r.Label)
	}
	want := []string{"Viewer", "Editor"}
	if !slices.Equal(labels, want) || roles[1].Role != "FULL_EDITOR" || !slices.Contains(roles[1].Capabilities, "MOVE") {
		t.Fatalf("item roles = %v", labels)
	}

	// A shared drive's root is where members are managed, and says "Drive Admin".
	d, err := h.m().CreateSharedDrive(h.as(h.admin), "Finance")
	if err != nil {
		t.Fatal(err)
	}
	roles, err = h.q().ShareRoles(h.as(h.admin), d.ID)
	if err != nil || len(roles) != 5 || roles[4].Label != "Drive Admin" || roles[4].Role != "DRIVE_ADMIN" || roles[2].Label != "Restricted Editor" {
		t.Fatalf("drive roles = %+v %v", roles, err)
	}

	if _, err := h.q().ShareRoles(h.as(h.carol), h.docs); code(err) != "NOT_FOUND" {
		t.Fatalf("an invisible item: %v", err)
	}
}

func TestItemAccessReportsTheOwner(t *testing.T) {
	h := newHarness(t)

	access, err := h.q().ItemAccess(h.as(h.alice), h.docs)
	if err != nil || access.Owner.Type != "USER" || access.Owner.ID != h.alice || access.Owner.Name != "alice" {
		t.Fatalf("private drive owner: %+v %v", access.Owner, err)
	}

	d, err := h.m().CreateSharedDrive(h.as(h.admin), "Finance")
	if err != nil {
		t.Fatal(err)
	}
	access, err = h.q().ItemAccess(h.as(h.admin), d.ID)
	if err != nil || access.Owner.Type != "TENANT" || access.Owner.Name != "Acme Inc" {
		t.Fatalf("a shared drive belongs to the organization: %+v %v", access.Owner, err)
	}
}

func TestSearchDirectory(t *testing.T) {
	h := newHarness(t)
	ctx := h.as(h.alice)
	cfg, err := h.r.TenantStore.GetPublicTenantAuthConfig(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	mustGroup(t, h, cfg.Providers[0].ID, "Design Team")

	names := func(found []*DirectorySubject) []string {
		var out []string
		for _, f := range found {
			out = append(out, f.Type+":"+f.Name)
		}
		return out
	}

	// Everyone in the tenant can search it, by name or email, case-insensitively.
	found, err := h.q().SearchDirectory(ctx, "BO", nil, nil)
	if err != nil || !slices.Equal(names(found), []string{"USER:bob"}) || found[0].Email == nil || *found[0].Email != "bob@acme.com" {
		t.Fatalf("by name: %v %v", names(found), err)
	}
	if found, _ := h.q().SearchDirectory(ctx, "@acme.com", nil, nil); len(found) != 3 {
		t.Fatalf("by email: %v", names(found)) // bob, carol, ada; never the caller
	}
	if found, _ := h.q().SearchDirectory(ctx, "design", nil, nil); !slices.Equal(names(found), []string{"GROUP:Design Team"}) {
		t.Fatalf("groups are searchable too: %v", names(found))
	}

	// Too-short queries return nothing (no browsing the directory by accident).
	if found, err := h.q().SearchDirectory(ctx, "b", nil, nil); err != nil || len(found) != 0 {
		t.Fatalf("short query: %v %v", found, err)
	}
	if found, _ := h.q().SearchDirectory(ctx, "@acme.com", ptr(2), nil); len(found) != 2 {
		t.Fatalf("limit: %d", len(found))
	}

	// The picker skips people who already have access, and the owner.
	if _, err := h.m().ShareItem(ctx, ShareInput{ItemID: h.docs, SubjectType: "USER", SubjectID: h.bob, Role: "VIEWER"}); err != nil {
		t.Fatal(err)
	}
	found, _ = h.q().SearchDirectory(ctx, "@acme.com", nil, &h.docs)
	if slices.Contains(names(found), "USER:bob") || len(found) != 2 {
		t.Fatalf("exclusion: %v", names(found))
	}

	// Other tenants are not searchable, and signed-out callers cannot search.
	if _, err := h.q().SearchDirectory(anonymous(), "bob", nil, nil); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Fatalf("signed out: %v", err)
	}
}

func TestAccessListMarksYouAndShowsInheritedAccess(t *testing.T) {
	h := newHarness(t)
	if _, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.docs, SubjectType: "user", SubjectID: h.bob, Role: "full_editor"}); err != nil {
		t.Fatal(err)
	}

	// The owner sees themselves marked, and bob's access from the folder above.
	access, err := h.q().ItemAccess(h.as(h.alice), h.file)
	if err != nil {
		t.Fatal(err)
	}
	if !access.Owner.IsYou {
		t.Errorf("the owner's own row must be marked: %+v", access.Owner)
	}
	if len(access.Grants) != 0 || len(access.Inherited) != 1 {
		t.Fatalf("the file has no grants of its own and bob's from docs: %+v", access)
	}
	got := access.Inherited[0]
	if got.SubjectID != h.bob || got.IsYou || got.InheritedFrom == nil || got.InheritedFrom.Name != "docs" || got.InheritedFrom.ID == nil || *got.InheritedFrom.ID != h.docs {
		t.Errorf("bob's grant should say it comes from docs: %+v from %+v", got, got.InheritedFrom)
	}

	// A grant made on the file itself is direct, and not alice's.
	if _, err := h.m().ShareItem(h.as(h.alice), ShareInput{ItemID: h.file, SubjectType: "user", SubjectID: h.carol, Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	direct, err := h.q().ItemAccess(h.as(h.alice), h.file)
	if err != nil {
		t.Fatal(err)
	}
	if len(direct.Grants) != 1 || direct.Grants[0].IsYou || direct.Grants[0].InheritedFrom != nil {
		t.Errorf("a direct grant is not inherited and not alice: %+v", direct.Grants)
	}
}
