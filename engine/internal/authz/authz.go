package authz

import "context"

// Authorizer decides what a principal may do and stores the data it decides
// from. It owns writes as well as reads, so each engine keeps its own data and
// nothing is ever dual-written: the default SQL engine writes SQL tables, and
// an OpenFGA engine would write tuples. Switching engines is a one-time
// migration, not a sync.
type Authorizer interface {
	// Principal resolves a signed-in user, including every group they belong to.
	Principal(ctx context.Context, tenantID, userID string) (Principal, error)

	// Caps returns the capabilities p holds on an item. An item that does not
	// exist, or is not visible to p, yields no capabilities and no error, so
	// callers cannot probe for existence.
	Caps(ctx context.Context, p Principal, itemID string) (Capability, error)
	// CapsMany is Caps for many items in one round trip. Every requested ID has
	// an entry in the result.
	CapsMany(ctx context.Context, p Principal, itemIDs []string) (map[string]Capability, error)
	// Check reports whether p holds every capability in need on the item.
	Check(ctx context.Context, p Principal, itemID string, need Capability) (bool, error)

	// SharedWithMe lists items shared directly with p or p's groups, as entry
	// points. Pass the last ItemID of the previous page as after.
	SharedWithMe(ctx context.Context, p Principal, limit int, after string) ([]SharedItem, error)
	// ListGrants returns the grants on an item. Requires CapShare.
	ListGrants(ctx context.Context, actor Principal, itemID string) ([]Grant, error)

	// Grant shares an item. The actor needs CapShare and cannot hand out
	// capabilities they do not hold themselves.
	Grant(ctx context.Context, actor Principal, in GrantInput) (*Grant, error)
	// Revoke removes a grant. The actor needs CapShare on the grant's item.
	Revoke(ctx context.Context, actor Principal, grantID string) error
	// SetInheritance stops (false) or resumes (true) an item inheriting its
	// ancestors' grants. Requires CapManage. Breaking inheritance keeps the
	// actor's own access with a direct manager grant so they cannot lock
	// themselves out.
	SetInheritance(ctx context.Context, actor Principal, itemID string, inherit bool) error

	// AddMember and RemoveMember maintain group membership. They are
	// administrative (tenant admin or SCIM), not file operations.
	AddMember(ctx context.Context, tenantID, groupID string, mt MemberType, memberID string) error
	RemoveMember(ctx context.Context, tenantID, groupID string, mt MemberType, memberID string) error
}
