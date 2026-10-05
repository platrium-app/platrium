package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
)

type driveEnv struct {
	db     *db.DB
	az     *sqlauthz.Authorizer
	fs     *fsops.FSOps
	orch   *driveOrchestrator
	tenant string
	idp    string
}


func newDriveEnv(t *testing.T) *driveEnv {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	e := &driveEnv{db: d, az: sqlauthz.New(d)}
	e.fs = fsops.NewFSOps(d, nil, e.az)
	e.orch = newDriveOrchestrator(d, e.fs, e.az)

	tn, err := d.Tenant.Create().SetAlias("acme").SetName("Acme").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idp, err := d.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("local").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e.tenant, e.idp = tn.ID, idp.ID
	return e
}

func (e *driveEnv) user(t *testing.T, name, role string) authz.Principal {
	t.Helper()
	ctx := context.Background()
	u, err := e.db.User.Create().SetTenantID(e.tenant).SetIdpID(e.idp).SetExternalID(name).
		SetEmail(name + "@acme.com").SetDisplayName(name).SetRole(role).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return e.principal(t, u.ID)
}

func (e *driveEnv) principal(t *testing.T, userID string) authz.Principal {
	t.Helper()
	p, err := e.az.Principal(context.Background(), e.tenant, userID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *driveEnv) group(t *testing.T, name string) string {
	t.Helper()
	g, err := e.db.Group.Create().SetTenantID(e.tenant).SetIdpID(e.idp).SetExternalID(name).SetName(name).Save(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return g.ID
}

func TestAdminCreatesASharedDrive(t *testing.T) {
	ctx := context.Background()
	e := newDriveEnv(t)
	admin := e.user(t, "ada", identity.RoleAdmin)
	bob := e.user(t, "bob", identity.RoleMember)

	d, err := e.orch.CreateSharedDrive(ctx, admin, "  Finance ")
	if err != nil {
		t.Fatal(err)
	}
	if d.Type != fsops.DriveTypeShared || d.OwnerID != "" || d.Name != "Finance" {
		t.Fatalf("unexpected drive: %+v", d)
	}
	// The creator can open it and runs it; nobody else can see it yet.
	if !d.Caps.Has(authz.CapDeleteDrive) || !d.Caps.Has(authz.CapShare) {
		t.Fatalf("creator caps = %s", d.Caps)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, admin); len(drives) != 1 || drives[0].ID != d.ID {
		t.Fatalf("admin drives: %+v", drives)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, bob); len(drives) != 0 {
		t.Fatalf("bob must not see it: %+v", drives)
	}

	// The admin can add members, who then see it.
	if _, err := e.az.Grant(ctx, admin, authz.GrantInput{ItemID: d.ID, Subject: authz.Subject{Type: authz.SubjectUser, ID: bob.UserID}, Role: authz.RoleFullEditor}); err != nil {
		t.Fatal(err)
	}
	if drives, _ := e.fs.GetUserDrives(ctx, bob); len(drives) != 1 || !drives[0].Caps.Has(authz.CapMove) || drives[0].Caps.Has(authz.CapShare) {
		t.Fatalf("bob as a full editor: %+v", drives)
	}
	// Names are unique per tenant, ignoring case.
	if _, err := e.orch.CreateSharedDrive(ctx, admin, "FINANCE"); !errors.Is(err, fsops.ErrConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
}

func TestWhoMayCreateSharedDrives(t *testing.T) {
	ctx := context.Background()
	e := newDriveEnv(t)
	admin := e.user(t, "ada", identity.RoleAdmin)
	super := e.user(t, "sue", identity.RoleSuperAdmin)
	bob := e.user(t, "bob", identity.RoleMember)
	team := e.group(t, "drive-creators")

	can := func(p authz.Principal) bool {
		ok, err := e.orch.CanCreateSharedDrive(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !can(admin) || !can(super) {
		t.Error("admins always may")
	}
	if can(bob) || can(authz.Anonymous()) {
		t.Error("members and visitors may not by default")
	}
	if _, err := e.orch.CreateSharedDrive(ctx, bob, "Nope"); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("a plain member: %v", err)
	}
	if _, err := e.orch.CreateSharedDrive(ctx, authz.Anonymous(), "Nope"); !errors.Is(err, authz.ErrForbidden) {
		t.Fatalf("anonymous: %v", err)
	}

	// The admin allows a group; its members may now create drives.
	if err := e.orch.SetSharedDriveCreators(ctx, admin, []string{team}); err != nil {
		t.Fatal(err)
	}
	if err := e.az.AddMember(ctx, e.tenant, team, authz.MemberUser, bob.UserID); err != nil {
		t.Fatal(err)
	}
	bob = e.principal(t, bob.UserID) // a new request picks up the membership
	if !can(bob) {
		t.Fatal("a member of an allowed group may create")
	}
	if _, err := e.orch.CreateSharedDrive(ctx, bob, "Design"); err != nil {
		t.Fatalf("allowed creator: %v", err)
	}

	// Leaving the group, or the admin removing the group, takes the right away.
	if err := e.az.RemoveMember(ctx, e.tenant, team, authz.MemberUser, bob.UserID); err != nil {
		t.Fatal(err)
	}
	if can(e.principal(t, bob.UserID)) {
		t.Error("right must end with membership")
	}
}

func TestManagingTheCreatorGroups(t *testing.T) {
	ctx := context.Background()
	e := newDriveEnv(t)
	admin := e.user(t, "ada", identity.RoleAdmin)
	bob := e.user(t, "bob", identity.RoleMember)
	g1, g2 := e.group(t, "g1"), e.group(t, "g2")

	if err := e.orch.SetSharedDriveCreators(ctx, bob, []string{g1}); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("only admins set it: %v", err)
	}
	if _, err := e.orch.SharedDriveCreators(ctx, bob); !errors.Is(err, authz.ErrForbidden) {
		t.Errorf("only admins read it: %v", err)
	}

	if err := e.orch.SetSharedDriveCreators(ctx, admin, []string{g1, g2, g1}); err != nil {
		t.Fatal(err)
	}
	got, _ := e.orch.SharedDriveCreators(ctx, admin)
	if len(got) != 2 {
		t.Fatalf("groups = %v", got)
	}
	if err := e.orch.SetSharedDriveCreators(ctx, admin, []string{g2}); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.orch.SharedDriveCreators(ctx, admin); len(got) != 1 || got[0] != g2 {
		t.Fatalf("replaced: %v", got)
	}
	if err := e.orch.SetSharedDriveCreators(ctx, admin, []string{"no-such-group"}); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("unknown group: %v", err)
	}
	if err := e.orch.SetSharedDriveCreators(ctx, admin, nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.orch.SharedDriveCreators(ctx, admin); len(got) != 0 {
		t.Fatalf("cleared: %v", got)
	}
}

// failingGrant makes the creator's first grant fail.
type failingGrant struct{ authz.Authorizer }

func (failingGrant) GrantInitial(context.Context, string, string, string, authz.Role) error {
	return errors.New("authorizer is down")
}

func TestFailedGrantLeavesNoDrive(t *testing.T) {
	ctx := context.Background()
	e := newDriveEnv(t)
	admin := e.user(t, "ada", identity.RoleAdmin)
	broken := newDriveOrchestratorWith(e.db, e.fs, failingGrant{e.az})

	if _, err := broken.CreateSharedDrive(ctx, admin, "Finance"); err == nil {
		t.Fatal("must fail when the creator cannot be granted access")
	}
	if n, _ := e.db.Drive.Query().Count(ctx); n != 0 {
		t.Fatalf("an unreachable drive was left behind (%d)", n)
	}
	if n, _ := e.db.DriveItem.Query().Count(ctx); n != 0 {
		t.Fatalf("its root folder was left behind (%d)", n)
	}
	// And the name is free for a retry.
	if _, err := e.orch.CreateSharedDrive(ctx, admin, "Finance"); err != nil {
		t.Fatalf("retry: %v", err)
	}
}
