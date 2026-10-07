package authz

import "context"

// An authz engine is the contract below, assembled from four small interfaces.
// An engine implements all of them, and each engine keeps its own data and
// owns writes as well as reads, so nothing is ever dual-written: the default
// SQL engine writes SQL tables, and an OpenFGA engine would write tuples.
// Switching engines is a one-time migration, not a sync. Callers depend on
// only the part they use (the file layer needs Checker, not Sharing), which
// keeps them easy to fake in tests.

// Resolver turns a signed-in user into the principal checks run as.
type Resolver interface {
	// Principal resolves a signed-in user, including every group they belong
	// to. How membership is expanded is the engine's business: the SQL engine
	// reads a flattened closure table, another engine may not need the list.
	// It does not check that the user exists or may sign in: that is identity's
	// question, answered by package actor before this is asked.
	Principal(ctx context.Context, tenantID, userID string) (Principal, error)
}

// Checker answers what a principal may do to an item.
type Checker interface {
	// Caps returns the capabilities p holds on an item. An item that does not
	// exist, or is not visible to p, yields no capabilities and no error, so
	// callers cannot probe for existence.
	Caps(ctx context.Context, p Principal, itemID string) (Capability, error)
	// CapsMany is Caps for many items in one round trip. Every requested ID has
	// an entry in the result.
	CapsMany(ctx context.Context, p Principal, itemIDs []string) (map[string]Capability, error)
	// Check reports whether p holds every capability in need on the item.
	Check(ctx context.Context, p Principal, itemID string, need Capability) (bool, error)
}

// Sharing reads and changes who can open an item.
type Sharing interface {
	// SharedWithMe lists items shared directly with p or p's groups, as entry
	// points. Pass the last ItemID of the previous page as after.
	SharedWithMe(ctx context.Context, p Principal, limit int, after string) ([]SharedItem, error)
	// ListGrants returns the grants on an item. Requires CapShare.
	ListGrants(ctx context.Context, actor Principal, itemID string) ([]Grant, error)
	// ItemAccess returns an item's grants, whether it inherits, and the access
	// that reaches it from ancestors. Requires CapShare.
	ItemAccess(ctx context.Context, actor Principal, itemID string) (*ItemAccess, error)

	// Grant shares an item. The actor needs CapShare and cannot hand out
	// capabilities they do not hold themselves.
	Grant(ctx context.Context, actor Principal, in GrantInput) (*Grant, error)
	// Revoke removes a grant. The actor needs CapShare on the grant's item.
	Revoke(ctx context.Context, actor Principal, grantID string) error
	// GrantInitial gives a user a role on an item with no acting user. It is for
	// trusted server code that has just created something nobody can reach yet,
	// such as a new shared drive, and must never be reachable from a request that
	// names its own arguments. It is idempotent.
	GrantInitial(ctx context.Context, tenantID, itemID, userID string, role Role) error

	// SetGeneralAccess sets who else can open an item beyond the people and
	// groups added by name: restricted, the whole organization, or anyone with
	// the link. It replaces any earlier general access in one step. Requires
	// CapShare; public access also needs the tenant to allow it.
	SetGeneralAccess(ctx context.Context, actor Principal, in GeneralAccessInput) error
	// GeneralAccessOptions lists the levels that can be chosen for an item right
	// now, narrowest first. A level the tenant does not allow, such as public
	// links when it has turned them off, is left out. Requires CapShare.
	GeneralAccessOptions(ctx context.Context, actor Principal, itemID string) ([]LevelDef, error)
	// SetInheritance stops (false) or resumes (true) an item inheriting its
	// ancestors' grants. Requires CapManage. It changes no grants. A restricted
	// item hides ancestors' grants from everyone except managers: a grant that
	// carries MANAGE, a Drive Admin's, still applies, so restricting cannot lock
	// a drive's admins out.
	SetInheritance(ctx context.Context, actor Principal, itemID string, inherit bool) error
}

// Membership maintains group membership.
type Membership interface {
	// AddMember and RemoveMember maintain group membership. They are
	// administrative (tenant admin or SCIM), not file operations.
	AddMember(ctx context.Context, tenantID, groupID string, mt MemberType, memberID string) error
	RemoveMember(ctx context.Context, tenantID, groupID string, mt MemberType, memberID string) error
}

// Authorizer is what an authz engine implements: every part of the contract.
type Authorizer interface {
	Resolver
	Checker
	Sharing
	Membership
}
