package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/user"
	"platrium/internal/orchestrator"
)

type adminEnv struct {
	db     *db.DB
	admin  *orchestrator.UserAdmin
	local  *local.LocalUserStore
	users  *identity.UserStore
	tenant string // the organization under test (not native)
	home   string // the native tenant
	idp    string // the organization's LOCAL IdP
	oidc   string // the organization's OIDC IdP
	extID  string // a user from the OIDC IdP
}

func newAdminEnv(t *testing.T) *adminEnv {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	users := identity.NewUserStore(d)
	idps := auth.NewIdpStore(d)
	fs := fsops.NewFSOps(d, nil, sqlauthz.New(d))
	lus := local.NewLocalUserStore(d, orchestrator.NewUserOrchestrator(users, fs))
	e := &adminEnv{db: d, admin: orchestrator.NewUserAdmin(d, users, idps, lus), local: lus, users: users}

	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		home, err := tx.Tenant.Create().SetAlias("home").SetName("home").SetNativeSlot(1).Save(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.IdpProvider.Create().SetTenantID(home.ID).SetType("LOCAL").SetName("local").Save(ctx); err != nil {
			return err
		}
		acme, err := tx.Tenant.Create().SetAlias("acme").SetName("acme").Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(acme.ID).SetType("LOCAL").SetName("Platrium Authentication").Save(ctx)
		if err != nil {
			return err
		}
		oidc, err := tx.IdpProvider.Create().SetTenantID(acme.ID).SetType("OIDC").SetName("Okta").Save(ctx)
		if err != nil {
			return err
		}
		ext, err := tx.User.Create().SetTenantID(acme.ID).SetIdpID(oidc.ID).SetExternalID("sub-1").SetEmail("ext@acme.com").SetDisplayName("Ext").Save(ctx)
		if err != nil {
			return err
		}
		e.home, e.tenant, e.idp, e.oidc, e.extID = home.ID, acme.ID, idp.ID, oidc.ID, ext.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// user creates a built-in user of the organization with the given role.
func (e *adminEnv) user(t *testing.T, name, role string) authz.Principal {
	t.Helper()
	ctx := context.Background()
	hash, err := local.HashPassword("password-1")
	if err != nil {
		t.Fatal(err)
	}
	var u *identity.User
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		u, err = e.local.CreateUserTx(ctx, tx, identity.CreateUserParams{
			TenantID: e.tenant, IdpID: e.idp, ExternalID: name, Email: name + "@acme.com", DisplayName: name, Role: role,
		}, hash)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return authz.Principal{TenantID: e.tenant, UserID: u.ID}
}

func wantErr(t *testing.T, what string, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("%s: got %v, want %v", what, err, target)
	}
}

func TestCreateLocalUser(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	super := e.user(t, "sue", identity.RoleSuperAdmin)
	admin := e.user(t, "ada", identity.RoleAdmin)
	member := e.user(t, "mia", identity.RoleMember)

	got, err := e.admin.CreateLocalUser(ctx, admin, orchestrator.CreateLocalUserInput{Email: "  New.User@Acme.com ", DisplayName: " New User ", Password: "password-2"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "new.user@acme.com" || got.DisplayName != "New User" || got.Role != identity.RoleMember || got.IdpType != "LOCAL" || got.Disabled() {
		t.Fatalf("unexpected user: %+v", got)
	}
	if ok, err := e.local.VerifyPassword(ctx, got.ID, "password-2"); err != nil || !ok {
		t.Fatalf("the new user must be able to sign in: %v %v", ok, err)
	}
	if n, _ := e.db.Drive.Query().Count(ctx); n != 4 { // sue, ada, mia and the new user each have a private drive
		t.Errorf("drives = %d, want a private drive for each of the 4 users", n)
	}

	_, err = e.admin.CreateLocalUser(ctx, admin, orchestrator.CreateLocalUserInput{Email: "new.user@acme.com", DisplayName: "Dup", Password: "password-2"})
	wantErr(t, "same sign-in name twice", err, identity.ErrConflict)
	if err == nil || !strings.Contains(err.Error(), "a user with the email new.user@acme.com already exists") {
		t.Errorf("a duplicate must say so in plain words: %v", err)
	}

	for name, in := range map[string]orchestrator.CreateLocalUserInput{
		"bad email":      {Email: "not-an-email", DisplayName: "A", Password: "password-2"},
		"email w/ name":  {Email: "Bob <bob@acme.com>", DisplayName: "A", Password: "password-2"},
		"blank name":     {Email: "a@acme.com", DisplayName: "  ", Password: "password-2"},
		"short password": {Email: "a@acme.com", DisplayName: "A", Password: "short"},
		"long password":  {Email: "a@acme.com", DisplayName: "A", Password: fmt.Sprintf("%073d", 1)},
		"unknown role":   {Email: "a@acme.com", DisplayName: "A", Password: "password-2", Role: "OWNER"},
	} {
		_, err := e.admin.CreateLocalUser(ctx, super, in)
		wantErr(t, name, err, authz.ErrInvalid)
	}

	// Roles: you hand out no more than you hold.
	_, err = e.admin.CreateLocalUser(ctx, admin, orchestrator.CreateLocalUserInput{Email: "x@acme.com", DisplayName: "X", Password: "password-2", Role: identity.RoleAdmin})
	wantErr(t, "an admin cannot promote (no roles.assign)", err, authz.ErrForbidden)
	if _, err := e.admin.CreateLocalUser(ctx, super, orchestrator.CreateLocalUserInput{Email: "y@acme.com", DisplayName: "Y", Password: "password-2", Role: identity.RoleAdmin}); err != nil {
		t.Errorf("a super admin may create an admin: %v", err)
	}
	_, err = e.admin.CreateLocalUser(ctx, member, orchestrator.CreateLocalUserInput{Email: "z@acme.com", DisplayName: "Z", Password: "password-2"})
	wantErr(t, "a member cannot create users", err, authz.ErrForbidden)
	_, err = e.admin.CreateLocalUser(ctx, authz.Anonymous(), orchestrator.CreateLocalUserInput{Email: "z@acme.com", DisplayName: "Z", Password: "password-2"})
	wantErr(t, "anonymous", err, authz.ErrForbidden)

	// Nothing a rejected request did may linger.
	if n, _ := e.db.User.Query().Count(ctx); n != 6 { // 3 + ext + new user + y
		t.Errorf("users = %d, want 6", n)
	}
}

func TestAdminsManageOnlyUsersTheyOutrank(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	super := e.user(t, "sue", identity.RoleSuperAdmin)
	admin := e.user(t, "ada", identity.RoleAdmin)
	other := e.user(t, "olga", identity.RoleAdmin)
	member := e.user(t, "mia", identity.RoleMember)

	_, err := e.admin.SetUserDisabled(ctx, admin, super.UserID, true)
	wantErr(t, "an admin disabling a super admin", err, authz.ErrForbidden)
	wantErr(t, "an admin resetting a super admin's password", e.admin.ResetLocalUserPassword(ctx, admin, super.UserID, "password-9"), authz.ErrForbidden)
	_, err = e.admin.UpdateLocalUser(ctx, admin, super.UserID, ptr("Boss"), nil)
	wantErr(t, "an admin renaming a super admin", err, authz.ErrForbidden)

	if _, err := e.admin.SetUserDisabled(ctx, admin, other.UserID, true); err != nil {
		t.Errorf("an admin may disable a peer: %v", err)
	}
	if _, err := e.admin.SetUserDisabled(ctx, admin, member.UserID, true); err != nil {
		t.Errorf("an admin may disable a member: %v", err)
	}
	if err := e.admin.ResetLocalUserPassword(ctx, admin, member.UserID, "password-9"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := e.local.VerifyPassword(ctx, member.UserID, "password-9"); !ok {
		t.Error("the new password must work")
	}
	if ok, _ := e.local.VerifyPassword(ctx, member.UserID, "password-1"); ok {
		t.Error("the old password must stop working")
	}
	wantErr(t, "too short", e.admin.ResetLocalUserPassword(ctx, admin, member.UserID, "x"), authz.ErrInvalid)

	// The list tells the admin up front who they may act on.
	items, _, _, err := e.admin.ListUsers(ctx, admin, identity.UserFilter{}, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if want := it.ID != super.UserID; it.Manageable != want {
			t.Errorf("%s: manageable = %v, want %v", it.DisplayName, it.Manageable, want)
		}
	}

	// A disabled actor holds nothing, even mid-request.
	_, err = e.admin.SetUserDisabled(ctx, other, member.UserID, false)
	wantErr(t, "a disabled admin", err, authz.ErrForbidden)
}

func TestExternalUsersCanOnlyBeDisabled(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	admin := e.user(t, "ada", identity.RoleAdmin)

	_, err := e.admin.UpdateLocalUser(ctx, admin, e.extID, ptr("New Name"), nil)
	wantErr(t, "editing an external user", err, authz.ErrInvalid)
	wantErr(t, "resetting an external user's password", e.admin.ResetLocalUserPassword(ctx, admin, e.extID, "password-9"), authz.ErrInvalid)

	got, err := e.admin.SetUserDisabled(ctx, admin, e.extID, true)
	if err != nil || !got.Disabled() || got.IdpName != "Okta" || got.IdpType != "OIDC" {
		t.Fatalf("disable external user: %+v %v", got, err)
	}
	if got, err = e.admin.SetUserDisabled(ctx, admin, e.extID, false); err != nil || got.Disabled() {
		t.Fatalf("enable external user: %+v %v", got, err)
	}
}

func TestRoleChanges(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	super := e.user(t, "sue", identity.RoleSuperAdmin)
	admin := e.user(t, "ada", identity.RoleAdmin)
	member := e.user(t, "mia", identity.RoleMember)

	_, err := e.admin.UpdateLocalUser(ctx, admin, member.UserID, nil, ptr(identity.RoleAdmin))
	wantErr(t, "an admin cannot assign roles", err, authz.ErrForbidden)

	got, err := e.admin.UpdateLocalUser(ctx, super, member.UserID, ptr("Mia M"), ptr(identity.RoleAdmin))
	if err != nil || got.Role != identity.RoleAdmin || got.DisplayName != "Mia M" {
		t.Fatalf("promote: %+v %v", got, err)
	}
	_, err = e.admin.UpdateLocalUser(ctx, super, member.UserID, nil, ptr("OWNER"))
	wantErr(t, "unknown role", err, authz.ErrForbidden)

	// A rename that keeps the role must not need roles.assign.
	if _, err := e.admin.UpdateLocalUser(ctx, admin, member.UserID, ptr("Mia Again"), ptr(identity.RoleAdmin)); err != nil {
		t.Errorf("setting the same role is not a promotion: %v", err)
	}
	_, err = e.admin.UpdateLocalUser(ctx, super, "missing", ptr("x"), nil)
	wantErr(t, "unknown user", err, identity.ErrNotFound)
}

func TestTheLastAssigningAdminIsProtected(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	a := e.user(t, "alpha", identity.RoleSuperAdmin)
	b := e.user(t, "beta", identity.RoleSuperAdmin)

	_, err := e.admin.SetUserDisabled(ctx, a, a.UserID, true)
	wantErr(t, "disabling yourself", err, authz.ErrForbidden)

	// With a second super admin, one may step down.
	if _, err := e.admin.UpdateLocalUser(ctx, a, b.UserID, nil, ptr(identity.RoleMember)); err != nil {
		t.Fatalf("demoting one of two super admins: %v", err)
	}
	// Now alpha is the only one left: neither demoting nor disabling may remove them.
	_, err = e.admin.UpdateLocalUser(ctx, a, a.UserID, nil, ptr(identity.RoleAdmin))
	wantErr(t, "demoting the last super admin", err, authz.ErrForbidden)

	// A disabled super admin does not count as one.
	c := e.user(t, "gamma", identity.RoleSuperAdmin)
	if _, err := e.admin.SetUserDisabled(ctx, a, c.UserID, true); err != nil {
		t.Fatal(err) // alpha is still there
	}
	if _, err := e.admin.SetUserDisabled(ctx, a, c.UserID, false); err != nil {
		t.Fatal(err)
	}
}

// Two super admins acting on each other at the same moment must not both
// succeed, or nobody could assign roles again. (On the default in-memory SQLite
// the single connection already serializes them; run with TEST_DB_DRIVER set
// to see the tenant row lock do the work on a real server.)
func TestConcurrentAdminsCannotRemoveEachOther(t *testing.T) {
	ctx := context.Background()
	for name, act := range map[string]func(e *adminEnv, by, target authz.Principal) error{
		"demote": func(e *adminEnv, by, target authz.Principal) error {
			_, err := e.admin.UpdateLocalUser(ctx, by, target.UserID, nil, ptr(identity.RoleMember))
			return err
		},
		"disable": func(e *adminEnv, by, target authz.Principal) error {
			_, err := e.admin.SetUserDisabled(ctx, by, target.UserID, true)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			for round := 0; round < 10; round++ {
				t.Run(fmt.Sprint("round", round), func(t *testing.T) {
					e := newAdminEnv(t)
					a := e.user(t, "alpha", identity.RoleSuperAdmin)
					b := e.user(t, "beta", identity.RoleSuperAdmin)

					errs := make(chan error, 2)
					start := make(chan struct{})
					go func() { <-start; errs <- act(e, a, b) }()
					go func() { <-start; errs <- act(e, b, a) }()
					close(start)
					var failed int
					for i := 0; i < 2; i++ {
						if err := <-errs; err != nil {
							failed++
							wantErr(t, "the loser is refused, not broken", err, authz.ErrForbidden)
						}
					}
					if failed != 1 {
						t.Fatalf("exactly one of two opposing changes may win, %d were refused", failed)
					}
					left, err := e.db.User.Query().Where(user.TenantID(e.tenant), user.Role(identity.RoleSuperAdmin), user.DisabledAtIsNil()).Count(ctx)
					if err != nil || left != 1 {
						t.Fatalf("one super admin must remain: %d %v", left, err)
					}
				})
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	e := newAdminEnv(t)
	super := e.user(t, "sue", identity.RoleSuperAdmin)
	for _, n := range []string{"ann", "bob", "cat", "dan"} {
		e.user(t, n, identity.RoleMember)
	}
	// A user of another tenant must never appear.
	var otherUser string
	if err := e.db.WithTx(ctx, func(tx *ent.Tx) error {
		other, err := tx.Tenant.Create().SetAlias("other").SetName("other").Save(ctx)
		if err != nil {
			return err
		}
		idp, err := tx.IdpProvider.Create().SetTenantID(other.ID).SetType("LOCAL").SetName("l").Save(ctx)
		if err != nil {
			return err
		}
		u, err := tx.User.Create().SetTenantID(other.ID).SetIdpID(idp.ID).SetExternalID("x").SetEmail("x@other.com").SetDisplayName("ann elsewhere").Save(ctx)
		otherUser = u.ID
		return err
	}); err != nil {
		t.Fatal(err)
	}

	names := func(f identity.UserFilter, after *identity.UserCursor, limit int) (out []string, hasNext bool, total int) {
		t.Helper()
		items, hasNext, total, err := e.admin.ListUsers(ctx, super, f, after, limit)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			out = append(out, it.DisplayName)
		}
		return out, hasNext, total
	}

	all, next, total := names(identity.UserFilter{}, nil, 100)
	if fmt.Sprint(all) != "[Ext ann bob cat dan sue]" || next || total != 6 {
		t.Fatalf("all: %v next=%v total=%d", all, next, total)
	}

	// Walk the list two at a time: every user once, in order, totals unaffected by paging.
	var walked []string
	var cursor *identity.UserCursor
	for pages := 0; pages < 10; pages++ {
		items, hasNext, total, err := e.admin.ListUsers(ctx, super, identity.UserFilter{}, cursor, 2)
		if err != nil || total != 6 {
			t.Fatalf("page: %v total=%d", err, total)
		}
		for _, it := range items {
			walked = append(walked, it.DisplayName)
		}
		if !hasNext {
			break
		}
		last := items[len(items)-1]
		cursor = &identity.UserCursor{DisplayName: last.DisplayName, ID: last.ID}
	}
	if fmt.Sprint(walked) != "[Ext ann bob cat dan sue]" {
		t.Errorf("paged walk: %v", walked)
	}

	if got, _, total := names(identity.UserFilter{Search: "AN"}, nil, 100); fmt.Sprint(got) != "[ann dan]" || total != 2 {
		t.Errorf("search: %v %d (must not see %s's tenant)", got, total, otherUser)
	}
	if got, _, _ := names(identity.UserFilter{IdpID: e.oidc}, nil, 100); fmt.Sprint(got) != "[Ext]" {
		t.Errorf("by source: %v", got)
	}
	if _, err := e.admin.SetUserDisabled(ctx, super, e.extID, true); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := names(identity.UserFilter{Status: identity.UserStatusDisabled}, nil, 100); fmt.Sprint(got) != "[Ext]" {
		t.Errorf("disabled: %v", got)
	}
	if _, _, total := names(identity.UserFilter{Status: identity.UserStatusActive}, nil, 100); total != 5 {
		t.Errorf("active total: %d", total)
	}
	if got, _, _ := names(identity.UserFilter{Search: "%"}, nil, 100); len(got) != 0 {
		t.Errorf("wildcards are literal: %v", got)
	}

	member := e.user(t, "mia", identity.RoleMember)
	_, _, _, err := e.admin.ListUsers(ctx, member, identity.UserFilter{}, nil, 10)
	wantErr(t, "a member listing users", err, authz.ErrForbidden)
}

func ptr[T any](v T) *T { return &v }
