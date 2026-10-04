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

// RoleContext says where a role is being offered. The roles and their
// capabilities are the same everywhere; only the wording differs, because
// "Drive Admin" reads wrong on a single file.
type RoleContext int

const (
	// ContextItemShare is sharing a file or folder inside a drive.
	ContextItemShare RoleContext = iota
	// ContextDriveMember is adding a member to a shared drive (sharing its root).
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
// with the wording to show. The backend owns the wording so every client says
// the same thing, and so future custom roles can appear in the same list.
func RoleOptions(c RoleContext) []RoleOption {
	adminLabel, adminDesc := "Admin", "Can do everything with this item, including deleting it and managing who has access"
	if c == ContextDriveMember {
		adminLabel, adminDesc = "Drive Admin", "Can do everything in this drive, including managing members and deleting the drive"
	}
	text := map[Role][2]string{
		RoleViewer:           {"Viewer", "Can view and download"},
		RoleCommenter:        {"Commenter", "Can view, download and comment"},
		RoleRestrictedEditor: {"Restricted Editor", "Can add and edit files, but cannot move or delete them"},
		RoleFullEditor:       {"Full Editor", "Can add, edit, move and trash files and folders"},
		RoleDriveAdmin:       {adminLabel, adminDesc},
	}

	var out []RoleOption
	for _, d := range builtinRoles {
		t, ok := text[d.role]
		if !ok {
			continue
		}
		out = append(out, RoleOption{Role: d.role, Label: t[0], Description: t[1], Caps: d.caps})
	}
	return out
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
	for _, o := range RoleOptions(ContextItemShare) {
		if o.Role == r {
			return true
		}
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
