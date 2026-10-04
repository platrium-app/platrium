package authz

// Role is a named bundle of capabilities. Roles are labels over bitmasks: a
// grant stores a snapshot of its capabilities plus a role label, so adding a
// capability to a role later never silently widens existing grants.
type Role string

const (
	RoleViewer         Role = "VIEWER"          // list, view, download
	RoleCommenter      Role = "COMMENTER"       // + comment
	RoleContributor    Role = "CONTRIBUTOR"     // + create, edit
	RoleContentManager Role = "CONTENT_MANAGER" // + move, trash
	RoleEditor         Role = "EDITOR"          // + share (single-item sharing's "can edit")
	RoleManager        Role = "MANAGER"         // + delete, move out, manage, delete the drive
	RoleOwner          Role = "OWNER"           // everything; implicit for the drive owner, not grantable
	RoleCustom         Role = "CUSTOM"          // capabilities that match no built-in role
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
	contributor := commenter | CapCreate | CapEdit
	contentManager := contributor | CapMove | CapTrash
	editor := contentManager | CapShare
	manager := editor | CapDelete | CapMoveOut | CapManage | CapDeleteDrive
	return []roleDef{
		{RoleViewer, Normalize(viewer)},
		{RoleCommenter, Normalize(commenter)},
		{RoleContributor, Normalize(contributor)},
		{RoleContentManager, Normalize(contentManager)},
		{RoleEditor, Normalize(editor)},
		{RoleManager, Normalize(manager)},
		{RoleOwner, AllCaps},
	}
}()

// RoleContext says what a role is being offered for. The capability registry
// is shared; the context only filters which roles make sense to hand out.
type RoleContext int

const (
	// ContextItemShare is sharing a single file or folder: Viewer, Commenter,
	// Editor, and Manager.
	ContextItemShare RoleContext = iota
	// ContextDriveMember is adding a member to a shared drive: the full ladder
	// of Viewer, Commenter, Contributor, Content Manager, and Manager.
	ContextDriveMember
)

// RolesFor lists the roles to offer in a context, least privileged first.
func RolesFor(c RoleContext) []Role {
	switch c {
	case ContextItemShare:
		return []Role{RoleViewer, RoleCommenter, RoleEditor, RoleManager}
	case ContextDriveMember:
		return []Role{RoleViewer, RoleCommenter, RoleContributor, RoleContentManager, RoleManager}
	}
	return nil
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

// Grantable reports whether the role may be assigned through a grant in some
// context. Owner is implicit and Custom is derived, so neither is.
func (r Role) Grantable() bool {
	for _, c := range []RoleContext{ContextItemShare, ContextDriveMember} {
		for _, offered := range RolesFor(c) {
			if offered == r {
				return true
			}
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
