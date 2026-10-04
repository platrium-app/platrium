package authz

// Role is a named bundle of capabilities. Roles are labels over bitmasks: a
// grant stores a snapshot of its capabilities plus a role label, so adding a
// capability to a role later never silently widens existing grants.
type Role string

const (
	RoleViewer           Role = "VIEWER"            // list, view, download
	RoleCommenter        Role = "COMMENTER"         // + comment
	RoleRestrictedEditor Role = "RESTRICTED_EDITOR" // + create, edit; cannot move or delete
	RoleFullEditor       Role = "FULL_EDITOR"       // + move, trash
	RoleDriveAdmin       Role = "DRIVE_ADMIN"       // + delete, move out, manage access, delete the drive
	RoleOwner            Role = "OWNER"             // everything; implicit for a private drive's owner, not grantable
	RoleCustom           Role = "CUSTOM"            // capabilities that match no built-in role
)

type roleDef struct {
	role Role
	caps Capability
}

// builtinRoles is ordered from least to most privileged; each role includes
// everything the one before it has.
var builtinRoles = func() []roleDef {
	viewer := CapList | CapView | CapDownload
	commenter := viewer | CapComment
	restrictedEditor := commenter | CapCreate | CapEdit
	fullEditor := restrictedEditor | CapMove | CapTrash
	driveAdmin := fullEditor | CapDelete | CapMoveOut | CapShare | CapManage | CapDeleteDrive
	return []roleDef{
		{RoleViewer, Normalize(viewer)},
		{RoleCommenter, Normalize(commenter)},
		{RoleRestrictedEditor, Normalize(restrictedEditor)},
		{RoleFullEditor, Normalize(fullEditor)},
		{RoleDriveAdmin, Normalize(driveAdmin)},
		{RoleOwner, AllCaps},
	}
}()

// RoleContext says where a role is being offered. The capabilities behind a
// role are the same everywhere, but which roles make sense, and what they are
// called, depends on what is being shared.
type RoleContext int

const (
	// ContextItemShare is sharing a file or folder: it has an owner, and can be
	// shared with people, groups, the organization or the public as a Viewer or
	// an Editor. Administering is not a role a single item has.
	ContextItemShare RoleContext = iota
	// ContextDriveMember is adding a member to a shared drive (sharing its
	// root): the full ladder, from Viewer up to Drive Admin.
	ContextDriveMember
)

// RoleOption is a role as shown to people choosing one.
type RoleOption struct {
	Role        Role
	Label       string
	Description string
	Caps        Capability
}

// RoleOptions lists the roles to offer in a context, least privileged first,
// with the wording to show. The backend owns both the list and the wording, so
// every client says the same thing and the server can refuse a role that does
// not belong (see Offers).
func RoleOptions(c RoleContext) []RoleOption {
	type text struct{ label, description string }
	var roles []Role
	words := map[Role]text{}

	switch c {
	case ContextItemShare:
		roles = []Role{RoleViewer, RoleFullEditor}
		words[RoleViewer] = text{"Viewer", "Can view and download"}
		words[RoleFullEditor] = text{"Editor", "Can add, edit, move and delete files and folders"}
	case ContextDriveMember:
		roles = []Role{RoleViewer, RoleCommenter, RoleRestrictedEditor, RoleFullEditor, RoleDriveAdmin}
		words[RoleViewer] = text{"Viewer", "Can view and download"}
		words[RoleCommenter] = text{"Commenter", "Can view, download and comment"}
		words[RoleRestrictedEditor] = text{"Restricted Editor", "Can add and edit files, but cannot move or delete them"}
		words[RoleFullEditor] = text{"Full Editor", "Can add, edit, move and trash files and folders"}
		words[RoleDriveAdmin] = text{"Drive Admin", "Can do everything in this drive, including managing members and deleting the drive"}
	default:
		return nil
	}

	out := make([]RoleOption, 0, len(roles))
	for _, r := range roles {
		caps, _ := r.Caps()
		out = append(out, RoleOption{Role: r, Label: words[r].label, Description: words[r].description, Caps: caps})
	}
	return out
}

// Offers reports whether a role may be assigned in a context.
func (c RoleContext) Offers(r Role) bool {
	for _, o := range RoleOptions(c) {
		if o.Role == r {
			return true
		}
	}
	return false
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

// Grantable reports whether the role may be assigned through a grant in any
// context. Owner is implicit and Custom is derived, so neither is. Which
// contexts offer it is a separate question (see RoleContext.Offers).
func (r Role) Grantable() bool {
	return ContextItemShare.Offers(r) || ContextDriveMember.Offers(r)
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
		if d.role != RoleOwner && c == d.caps {
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
