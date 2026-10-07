package authz_test

import (
	"slices"
	"testing"

	"platrium/internal/authz"
)

func TestEveryRolePermissionIsRegistered(t *testing.T) {
	for _, role := range authz.TenantRoles() {
		for _, p := range authz.EffectivePermissions(role, true).Sorted() {
			if _, ok := p.Scope(); !ok {
				t.Errorf("role %s carries unregistered permission %q", role, p)
			}
		}
	}
}

func TestEffectivePermissionsByRole(t *testing.T) {
	member := authz.EffectivePermissions(authz.TenantRoleMember, false)
	if member.Len() != 0 {
		t.Errorf("a member holds nothing: %v", member.Sorted())
	}
	if p := authz.EffectivePermissions("admin", true); p.Len() != 0 {
		t.Errorf("role names are exact; an unknown role holds nothing: %v", p.Sorted())
	}

	admin := authz.EffectivePermissions(authz.TenantRoleAdmin, false)
	for _, p := range []authz.Permission{authz.PermUsersRead, authz.PermUsersCreate, authz.PermUsersUpdate, authz.PermUsersDisable, authz.PermSharedDrivesCreate, authz.PermPoliciesManage} {
		if !admin.Has(p) {
			t.Errorf("admin should hold %s", p)
		}
	}
	if admin.Has(authz.PermRolesAssign) {
		t.Error("admin must not assign roles")
	}
}

func TestClusterPermissionsOnlyApplyInTheNativeTenant(t *testing.T) {
	native := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, true)
	org := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, false)

	if !native.Has(authz.PermTenantsManage) {
		t.Error("the native tenant's super admin administers the cluster")
	}
	if org.Has(authz.PermTenantsManage) {
		t.Error("an organization's super admin must never hold a cluster permission")
	}
	if !org.Has(authz.PermRolesAssign) || !org.Has(authz.PermUsersCreate) {
		t.Error("an organization's super admin still administers their own organization")
	}
	if !native.Covers(org) {
		t.Error("native super admin should hold everything an org super admin does")
	}
}

func TestCanAssignRole(t *testing.T) {
	super := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, false)
	admin := authz.EffectivePermissions(authz.TenantRoleAdmin, false)

	cases := []struct {
		name   string
		actor  authz.PermissionSet
		role   string
		native bool
		want   bool
	}{
		{"super admin makes an admin", super, authz.TenantRoleAdmin, false, true},
		{"super admin makes a member", super, authz.TenantRoleMember, false, true},
		{"super admin makes another super admin", super, authz.TenantRoleSuperAdmin, false, true},
		{"admin lacks roles.assign", admin, authz.TenantRoleMember, false, false},
		{"unknown role", super, "OWNER", false, false},
		{"org super admin cannot mint a cluster admin", super, authz.TenantRoleSuperAdmin, true, false},
		{"native super admin can", authz.EffectivePermissions(authz.TenantRoleSuperAdmin, true), authz.TenantRoleSuperAdmin, true, true},
	}
	for _, c := range cases {
		if got := authz.CanAssignRole(c.actor, c.role, c.native); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPermissionSetSorted(t *testing.T) {
	s := authz.NewPermissionSet(authz.PermUsersRead, authz.PermRolesAssign)
	if got := s.Sorted(); !slices.Equal(got, []authz.Permission{authz.PermRolesAssign, authz.PermUsersRead}) {
		t.Errorf("sorted: %v", got)
	}
}

// A set resolved once is shared by a whole request, so nothing it hands out may
// be a way to change it.
func TestPermissionSetCannotBeChangedFromOutside(t *testing.T) {
	s := authz.NewPermissionSet(authz.PermUsersRead, authz.PermRolesAssign)
	got := s.Sorted()
	got[0] = authz.PermTenantsManage
	got = append(got, authz.PermUsersCreate)
	_ = got
	if s.Len() != 2 || !s.Has(authz.PermUsersRead) || !s.Has(authz.PermRolesAssign) || s.Has(authz.PermTenantsManage) {
		t.Fatalf("the set changed: %v", s.Sorted())
	}

	// A role's permissions are a fresh set each time, never the table's own.
	a := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, true)
	b := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, true)
	if a.Len() != b.Len() || !a.Covers(b) || !b.Covers(a) {
		t.Fatal("two reads of one role must agree")
	}
	var zero authz.PermissionSet
	if zero.Has(authz.PermUsersRead) || zero.Len() != 0 || !zero.Covers(zero) {
		t.Fatal("the zero value is an empty set")
	}
}

// The boundary that keeps one organization's administrators out of the
// installation: no role held in an ordinary tenant ever carries a cluster
// permission, whatever the role table says.
func TestOrdinaryTenantsNeverHoldClusterPermissions(t *testing.T) {
	var cluster int
	for _, role := range authz.TenantRoles() {
		for _, p := range authz.EffectivePermissions(role, false).Sorted() {
			if scope, _ := p.Scope(); scope == authz.ScopeCluster {
				t.Errorf("role %s holds cluster permission %s outside the native tenant", role, p)
			}
		}
		for _, p := range authz.EffectivePermissions(role, true).Sorted() {
			if scope, _ := p.Scope(); scope == authz.ScopeCluster {
				cluster++
			}
		}
	}
	if cluster == 0 {
		t.Error("the native tenant must be able to hold a cluster permission, or this test proves nothing")
	}
	// Assigning is bounded the same way: an organization's super admin cannot
	// hand out a role whose power they do not hold.
	org := authz.EffectivePermissions(authz.TenantRoleSuperAdmin, false)
	if !authz.CanAssignRole(org, authz.TenantRoleSuperAdmin, false) {
		t.Error("an org super admin may create another org super admin")
	}
	if authz.CanAssignRole(org, authz.TenantRoleSuperAdmin, true) {
		t.Error("an org super admin must not create a native-tenant super admin")
	}
}
