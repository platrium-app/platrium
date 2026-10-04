package authz

import "testing"

// Role values are derived from capabilities, but pinned here because they show
// up in stored snapshots and in documentation.
func TestBuiltinRoleCapabilities(t *testing.T) {
	want := map[Role]uint64{
		RoleViewer:           7,
		RoleCommenter:        15,
		RoleRestrictedEditor: 783,
		RoleFullEditor:       6927,
		RoleDriveAdmin:       474895,
	}
	for role, v := range want {
		c, ok := role.Caps()
		if !ok || uint64(c) != v {
			t.Errorf("%s = %d, want %d", role, c, v)
		}
	}
	if c, _ := RoleOwner.Caps(); c != AllCaps {
		t.Error("the owner holds every capability")
	}
}

func TestRolesAreOrdered(t *testing.T) {
	var prev Capability
	for _, d := range builtinRoles {
		if !prev.SubsetOf(d.caps) {
			t.Errorf("%s must include everything the previous role has", d.role)
		}
		if d.caps != Normalize(d.caps) {
			t.Errorf("%s is not normalized", d.role)
		}
		prev = d.caps
	}
}

func TestGrantable(t *testing.T) {
	for _, r := range []Role{RoleViewer, RoleCommenter, RoleRestrictedEditor, RoleFullEditor, RoleDriveAdmin} {
		if !r.Grantable() {
			t.Errorf("%s must be grantable", r)
		}
	}
	for _, r := range []Role{RoleOwner, RoleCustom, "NOPE", "EDITOR", "MANAGER", ""} {
		if r.Grantable() {
			t.Errorf("%s must not be grantable", r)
		}
	}
}

func TestRoleOptions(t *testing.T) {
	want := []Role{RoleViewer, RoleCommenter, RoleRestrictedEditor, RoleFullEditor, RoleDriveAdmin}
	labels := map[RoleContext][]string{
		ContextItemShare:   {"Viewer", "Commenter", "Restricted Editor", "Full Editor", "Admin"},
		ContextDriveMember: {"Viewer", "Commenter", "Restricted Editor", "Full Editor", "Drive Admin"},
	}
	for ctx, wantLabels := range labels {
		got := RoleOptions(ctx)
		if len(got) != len(want) {
			t.Fatalf("context %d: %d options", ctx, len(got))
		}
		for i, o := range got {
			if o.Role != want[i] || o.Label != wantLabels[i] || o.Description == "" {
				t.Errorf("context %d option %d = %+v", ctx, i, o)
			}
			if c, _ := o.Role.Caps(); o.Caps != c {
				t.Errorf("%s: option capabilities do not match the role", o.Role)
			}
		}
	}
}

// The ladder encodes the behaviors people expect from each role.
func TestRoleBehaviors(t *testing.T) {
	has := func(r Role, c Capability) bool { caps, _ := r.Caps(); return caps.Has(c) }
	cases := []struct {
		role Role
		can  []Capability
		cant []Capability
	}{
		{RoleViewer, []Capability{CapList, CapView, CapDownload}, []Capability{CapComment, CapCreate, CapEdit}},
		{RoleCommenter, []Capability{CapComment}, []Capability{CapCreate, CapEdit}},
		{RoleRestrictedEditor, []Capability{CapCreate, CapEdit}, []Capability{CapMove, CapTrash, CapDelete}},
		{RoleFullEditor, []Capability{CapMove, CapTrash}, []Capability{CapDelete, CapMoveOut, CapShare, CapManage}},
		{RoleDriveAdmin, []Capability{CapDelete, CapMoveOut, CapShare, CapManage, CapDeleteDrive}, nil},
	}
	for _, c := range cases {
		for _, k := range c.can {
			if !has(c.role, k) {
				t.Errorf("%s must hold %s", c.role, k)
			}
		}
		for _, k := range c.cant {
			if has(c.role, k) {
				t.Errorf("%s must not hold %s", c.role, k)
			}
		}
	}
}

func TestGrantCaps(t *testing.T) {
	c, err := GrantCaps(RoleViewer, false)
	if err != nil || !c.Has(CapDownload) {
		t.Fatalf("viewer downloads by default: %v %v", c, err)
	}
	c, err = GrantCaps(RoleViewer, true)
	if err != nil || c.Has(CapDownload) || !c.Has(CapView) {
		t.Fatalf("no-download viewer: %v %v", c, err)
	}
	if _, err := GrantCaps(RoleOwner, false); err == nil {
		t.Error("owner cannot be granted")
	}
}

func TestDescribe(t *testing.T) {
	for _, d := range builtinRoles {
		if d.role == RoleOwner {
			continue // today identical to DRIVE_ADMIN; Describe labels it DRIVE_ADMIN
		}
		role, nd := Describe(d.caps)
		if role != d.role || nd {
			t.Errorf("Describe(%s caps) = %s, %v", d.role, role, nd)
		}
	}
	viewerNoDL, _ := GrantCaps(RoleViewer, true)
	if role, nd := Describe(viewerNoDL); role != RoleViewer || !nd {
		t.Errorf("viewer without download = %s, %v", role, nd)
	}
	editorNoDL, _ := GrantCaps(RoleFullEditor, true)
	if role, nd := Describe(editorNoDL); role != RoleFullEditor || !nd {
		t.Errorf("full editor without download = %s, %v", role, nd)
	}
	if role, _ := Describe(CapView | CapDelete); role != RoleCustom {
		t.Errorf("odd combination must be custom, got %s", role)
	}
	if role, _ := Describe(0); role != RoleCustom {
		t.Errorf("no capabilities is custom, got %s", role)
	}
}

func TestParseRole(t *testing.T) {
	if r, ok := ParseRole("FULL_EDITOR"); !ok || r != RoleFullEditor {
		t.Error("FULL_EDITOR")
	}
	if _, ok := ParseRole("full_editor"); ok {
		t.Error("role names are case-sensitive")
	}
	for _, old := range []string{"EDITOR", "MANAGER", "CONTRIBUTOR", "CONTENT_MANAGER", "NOPE"} {
		if _, ok := ParseRole(old); ok {
			t.Errorf("%s is not a role any more", old)
		}
	}
}
