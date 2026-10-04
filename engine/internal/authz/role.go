package authz

// Role is a named bundle of capabilities. Roles are labels over bitmasks: a
// grant stores a snapshot of its capabilities plus a role label, so adding a
// capability to a role later never silently widens existing grants.
type Role string

const (
	RoleViewer      Role = "VIEWER"      // list, view, download
	RoleContributor Role = "CONTRIBUTOR" // + create
	RoleEditor      Role = "EDITOR"      // + edit, delete
	RoleManager     Role = "MANAGER"     // + share, manage
	RoleOwner       Role = "OWNER"       // everything; implicit for the drive owner, not grantable
	RoleCustom      Role = "CUSTOM"      // capabilities that match no built-in role
)

type roleDef struct {
	role Role
	caps Capability
}

// builtinRoles is ordered from least to most privileged.
var builtinRoles = []roleDef{
	{RoleViewer, Normalize(CapList | CapView | CapDownload)},
	{RoleContributor, Normalize(CapList | CapView | CapDownload | CapCreate)},
	{RoleEditor, Normalize(CapList | CapView | CapDownload | CapCreate | CapEdit | CapDelete)},
	{RoleManager, Normalize(CapList | CapView | CapDownload | CapCreate | CapEdit | CapDelete | CapShare | CapManage)},
	{RoleOwner, AllCaps},
}

// Caps returns the capabilities of a built-in role.
func (r Role) Caps() (Capability, bool) {
	for _, d := range builtinRoles {
		if d.role == r {
			return d.caps, true
		}
	}
	return 0, false
}

// Grantable reports whether the role may be assigned through a grant. Owner is
// implicit and Custom is derived, so neither is.
func (r Role) Grantable() bool {
	switch r {
	case RoleViewer, RoleContributor, RoleEditor, RoleManager:
		return true
	}
	return false
}

// ParseRole parses a built-in role name.
func ParseRole(s string) (Role, bool) {
	r := Role(s)
	_, ok := r.Caps()
	return r, ok
}

// Describe labels a capability set for display. It returns the matching
// built-in role and whether download is disabled on it ("Viewer, no download").
// Sets that match no built-in role are RoleCustom.
func Describe(c Capability) (role Role, noDownload bool) {
	for _, d := range builtinRoles {
		if c == d.caps {
			return d.role, false
		}
	}
	for _, d := range builtinRoles {
		if d.role != RoleOwner && c == d.caps.Without(CapDownload) {
			return d.role, true
		}
	}
	return RoleCustom, false
}

// GrantCaps resolves the capabilities a grant of role should carry.
// noDownload clears the download capability.
func GrantCaps(role Role, noDownload bool) (Capability, error) {
	if !role.Grantable() {
		return 0, ErrInvalid
	}
	c, _ := role.Caps()
	if noDownload {
		c = c.Without(CapDownload)
	}
	return c, nil
}
