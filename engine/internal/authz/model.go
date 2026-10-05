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

// GeneralAccessLevel is the "who else can open this" setting of an item beyond
// the people and groups added to it by name. It is stored as at most one grant:
// a TENANT grant or a PUBLIC grant. The item's ID is the link, which is
// unguessable (about 126 random bits), so nothing else is needed to share by link.
type GeneralAccessLevel string

const (
	// AccessRestricted means only the people and groups added by name.
	AccessRestricted GeneralAccessLevel = "RESTRICTED"
	// AccessTenant means every member of the organization.
	AccessTenant GeneralAccessLevel = "TENANT"
	// AccessPublic means anyone who has the link, signed in or not.
	AccessPublic GeneralAccessLevel = "PUBLIC"
)

// GeneralAccessInput sets an item's general access. Role is required unless
// the level is AccessRestricted. Organization-wide access may use Viewer or
// Editor (the Full Editor role); public access may only use Viewer, because
// anonymous visitors can never write.
type GeneralAccessInput struct {
	ItemID     string
	Level      GeneralAccessLevel
	Role       Role
	NoDownload bool
	ExpiresAt  *time.Time
}

// GeneralAccessOf reads an item's general access from its grants. It returns
// the level and the grant behind it (nil when restricted).
func GeneralAccessOf(grants []Grant) (GeneralAccessLevel, *Grant) {
	// The widest level present wins.
	for i := len(levels) - 1; i >= 0; i-- {
		if !levels[i].Stored() {
			continue
		}
		for j := range grants {
			if grants[j].Subject.Type == levels[i].Subject {
				return levels[i].Level, &grants[j]
			}
		}
	}
	return AccessRestricted, nil
}

// ItemRef names an item, for saying where access comes from.
type ItemRef struct {
	// ID is empty when the viewer cannot open the item, so a name they could not
	// otherwise see is not given away either.
	ID   string
	Name string
}

// InheritedGrant is a grant that applies to an item because of a folder or
// drive above it.
type InheritedGrant struct {
	Grant
	From ItemRef
}

// ItemAccess is who can open an item: whether it inherits, the grants placed on
// it directly, and the access that reaches it from above.
type ItemAccess struct {
	ItemID              string
	InheritsPermissions bool
	Grants              []Grant
	// Inherited is the access that comes from ancestors, nearest first: every
	// grant above an unrestricted item, and only managers' grants above a
	// restriction. It is read-only here; it is changed where it was granted.
	Inherited []InheritedGrant
	// OwnerUserID is the user who owns the item's drive. Empty for shared
	// drives, which belong to the tenant.
	OwnerUserID string
}
