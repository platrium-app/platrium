package authz

import "time"

// SubjectType says who a grant applies to. It is a string, not a closed enum,
// so new kinds (service accounts, devices) can be added without a schema change.
type SubjectType string

const (
	SubjectUser   SubjectType = "USER"   // one user
	SubjectGroup  SubjectType = "GROUP"  // a group, including nested members
	SubjectTenant SubjectType = "TENANT" // every member of the tenant
	SubjectPublic SubjectType = "PUBLIC" // anyone, signed in or not
)

// PublicSubjectID is the subject ID used for PUBLIC grants.
const PublicSubjectID = "*"

// Subject identifies the receiver of a grant.
type Subject struct {
	Type SubjectType
	ID   string
}

// Principal is who is acting. Resolve it once per request; checks do not
// re-resolve group membership.
type Principal struct {
	TenantID string
	UserID   string
	GroupIDs []string // every group the user belongs to, nested ones included
}

// Anonymous returns a principal with no identity: only PUBLIC grants apply.
func Anonymous() Principal { return Principal{} }

// IsAnonymous reports whether the principal has no user.
func (p Principal) IsAnonymous() bool { return p.UserID == "" }

// Grant gives a subject capabilities on an item and everything beneath it
// (until an item that does not inherit).
type Grant struct {
	ID        string
	TenantID  string
	DriveID   string
	ItemID    string
	Subject   Subject
	Role      Role       // label; Caps is authoritative
	Caps      Capability // snapshot taken when the grant was written
	ExpiresAt *time.Time
	CreatedBy string
	CreatedAt time.Time
}

// GrantInput is a request to share an item. Sharing the same item with the
// same subject again replaces the earlier grant.
type GrantInput struct {
	ItemID     string
	Subject    Subject
	Role       Role // a grantable built-in role
	NoDownload bool // clear the download capability from the role
	ExpiresAt  *time.Time
}

// SharedItem is an entry point returned by SharedWithMe: an item shared
// directly with the principal (or one of their groups), not its descendants.
type SharedItem struct {
	ItemID string
	Caps   Capability // union of every grant on the item that applies
	Role   Role       // label for Caps
}

// MemberType is the kind of a group member.
type MemberType string

const (
	MemberUser  MemberType = "USER"
	MemberGroup MemberType = "GROUP"
)
