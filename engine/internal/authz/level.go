package authz

// LevelDef describes one general-access level: who it opens an item to, what
// they may be given, and what can be tuned on it. It is the single source of
// truth for levels. Validation, storage and the options clients render all
// read it, so a new level is one entry here (plus the check that lets a
// principal match its subject) and nothing else.
type LevelDef struct {
	Level GeneralAccessLevel
	Label string
	Blurb string
	// Subject is the kind of grant that stores the level. Empty for Restricted,
	// which stores nothing.
	Subject SubjectType
	// Roles are the roles the level may carry, least privileged first.
	Roles []Role
	// SupportsExpiry says whether the level's access can end on a date.
	SupportsExpiry bool
	// RequiresPublicSharing says the tenant must allow public sharing.
	RequiresPublicSharing bool
}

// Stored reports whether the level is kept as a grant.
func (d LevelDef) Stored() bool { return d.Subject != "" }

// Offers reports whether the level may carry a role.
func (d LevelDef) Offers(r Role) bool {
	for _, o := range d.Roles {
		if o == r {
			return true
		}
	}
	return false
}

// SubjectFor is the grant subject of the level for an actor's tenant.
func (d LevelDef) SubjectFor(tenantID string) Subject {
	if d.Subject == SubjectPublic {
		return Subject{Type: SubjectPublic, ID: PublicSubjectID}
	}
	return Subject{Type: d.Subject, ID: tenantID}
}

// levels is ordered from the narrowest level to the widest.
var levels = []LevelDef{
	{
		Level: AccessRestricted,
		Label: "Restricted",
		Blurb: "Only the people added above can open this",
	},
	{
		Level:          AccessTenant,
		Label:          "Anyone in your organization",
		Blurb:          "Everyone in your organization can open this",
		Subject:        SubjectTenant,
		Roles:          []Role{RoleViewer, RoleFullEditor}, // Viewer or Editor
		SupportsExpiry: true,
	},
	{
		Level:                 AccessPublic,
		Label:                 "Anyone with the link",
		Blurb:                 "Anyone who has the link can view this, signed in or not",
		Subject:               SubjectPublic,
		Roles:                 []Role{RoleViewer}, // anonymous visitors can never write
		SupportsExpiry:        true,
		RequiresPublicSharing: true,
	},
}

// Levels lists every general-access level, narrowest first.
func Levels() []LevelDef { return append([]LevelDef(nil), levels...) }

// LevelOf looks a level up.
func LevelOf(l GeneralAccessLevel) (LevelDef, bool) {
	for _, d := range levels {
		if d.Level == l {
			return d, true
		}
	}
	return LevelDef{}, false
}
