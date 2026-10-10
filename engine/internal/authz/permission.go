package authz

import (
	"slices"
	"sort"
)

// Permission is something a user may do to their organization or to the
// cluster, as opposed to a Capability, which is something done to one item.
// Code asks "does this user hold permission X", never "is this user an admin":
// what a role means is defined once, in this file, and can grow into
// tenant-defined roles without touching a single call site.
type Permission string

const (
	PermUsersRead    Permission = "users.read"    // list and inspect the organization's users
	PermUsersCreate  Permission = "users.create"  // create users on the built-in (LOCAL) provider
	PermUsersUpdate  Permission = "users.update"  // edit or reset the password of LOCAL users
	PermUsersDisable Permission = "users.disable" // disable or enable any user, whatever their provider

	PermRolesAssign Permission = "roles.assign" // give users a role, up to the permissions you hold

	PermSharedDrivesCreate Permission = "shared_drives.create" // create shared drives without a policy grant
	PermPoliciesManage     Permission = "policies.manage"      // choose which groups a tenant policy applies to

	PermIdpManage Permission = "idp.manage" // configure the organization's identity providers (who can sign in, and how)

	PermTenantsManage Permission = "tenants.manage" // cluster: configure tenants (native tenant only)
)

// PermissionScope says what a permission reaches.
type PermissionScope int

const (
	// ScopeTenant permissions act inside the holder's own organization.
	ScopeTenant PermissionScope = iota
	// ScopeCluster permissions act on the whole installation. They are only ever
	// effective for users of the native tenant, whatever role they hold.
	ScopeCluster
)

type permissionDef struct {
	perm  Permission
	scope PermissionScope
}

// permissionRegistry is the single source of truth for known permissions.
var permissionRegistry = []permissionDef{
	{PermUsersRead, ScopeTenant},
	{PermUsersCreate, ScopeTenant},
	{PermUsersUpdate, ScopeTenant},
	{PermUsersDisable, ScopeTenant},
	{PermRolesAssign, ScopeTenant},
	{PermSharedDrivesCreate, ScopeTenant},
	{PermPoliciesManage, ScopeTenant},
	{PermIdpManage, ScopeTenant},
	{PermTenantsManage, ScopeCluster},
}

// Scope returns a permission's scope; ok is false for an unknown permission.
func (p Permission) Scope() (scope PermissionScope, ok bool) {
	for _, d := range permissionRegistry {
		if d.perm == p {
			return d.scope, true
		}
	}
	return ScopeTenant, false
}

// PermissionSet is a set of permissions. It is immutable: there is no way to
// add to or remove from one after it is built, so a set resolved once for a
// request can be shared by everything in it. The zero value is empty and usable.
type PermissionSet struct{ m map[Permission]struct{} }

// NewPermissionSet returns a set holding the given permissions.
func NewPermissionSet(perms ...Permission) PermissionSet {
	m := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		m[p] = struct{}{}
	}
	return PermissionSet{m: m}
}

// Has reports whether the set holds p.
func (s PermissionSet) Has(p Permission) bool { _, ok := s.m[p]; return ok }

// Len is how many permissions the set holds.
func (s PermissionSet) Len() int { return len(s.m) }

// Covers reports whether the set holds every permission in other.
func (s PermissionSet) Covers(other PermissionSet) bool {
	for p := range other.m {
		if !s.Has(p) {
			return false
		}
	}
	return true
}

// Sorted lists the permissions in a stable order, for display and tests. The
// list is a copy; changing it changes nothing.
func (s PermissionSet) Sorted() []Permission {
	out := make([]Permission, 0, len(s.m))
	for p := range s.m {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Tenant roles. A user's role says what they may administer, separate from
// what they can do with any one file (see Role). The role is stored as a plain
// string on the user; this file owns what each one means.
const (
	// TenantRoleSuperAdmin is granted to a tenant's first administrator.
	TenantRoleSuperAdmin = "SUPER_ADMIN"
	// TenantRoleAdmin administers the tenant's people and drives.
	TenantRoleAdmin = "ADMIN"
	// TenantRoleMember is an ordinary user.
	TenantRoleMember = "MEMBER"
)

// builtinTenantRoles maps each role to the permissions it carries. A role may
// list cluster permissions: they only take effect for the native tenant.
var builtinTenantRoles = func() map[string]PermissionSet {
	admin := []Permission{PermUsersRead, PermUsersCreate, PermUsersUpdate, PermUsersDisable, PermSharedDrivesCreate, PermPoliciesManage}
	return map[string]PermissionSet{
		TenantRoleMember:     NewPermissionSet(),
		TenantRoleAdmin:      NewPermissionSet(admin...),
		TenantRoleSuperAdmin: NewPermissionSet(append(slices.Clone(admin), PermRolesAssign, PermIdpManage, PermTenantsManage)...),
	}
}()

// TenantRoles lists the roles that can be assigned, least privileged first.
func TenantRoles() []string {
	return []string{TenantRoleMember, TenantRoleAdmin, TenantRoleSuperAdmin}
}

// KnownTenantRole reports whether role is one that can be assigned.
func KnownTenantRole(role string) bool { _, ok := builtinTenantRoles[role]; return ok }

// EffectivePermissions is what a user with this role may do. native says the
// user belongs to the installation's native tenant, the only place cluster
// permissions apply: an organization's super admin never holds them. An unknown
// role holds nothing.
func EffectivePermissions(role string, native bool) PermissionSet {
	var perms []Permission
	for p := range builtinTenantRoles[role].m {
		if scope, _ := p.Scope(); scope == ScopeCluster && !native {
			continue
		}
		perms = append(perms, p)
	}
	return NewPermissionSet(perms...)
}

// CanAssignRole reports whether a user holding actor may give someone role.
// The rule is the same as for sharing: you can hand out no more than you hold.
// An actor cannot, for example, create a more powerful account than their own.
func CanAssignRole(actor PermissionSet, role string, native bool) bool {
	if !actor.Has(PermRolesAssign) || !KnownTenantRole(role) {
		return false
	}
	return actor.Covers(EffectivePermissions(role, native))
}

// RolesHolding lists the roles that carry permission p for a user of a tenant
// that is (or is not) the native one.
func RolesHolding(p Permission, native bool) []string {
	var out []string
	for _, role := range TenantRoles() {
		if EffectivePermissions(role, native).Has(p) {
			out = append(out, role)
		}
	}
	return out
}
