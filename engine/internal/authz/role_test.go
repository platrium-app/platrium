package authz

import "testing"

// Role values are derived from capabilities, but pinned here because they show
// up in stored snapshots and in documentation.
func TestBuiltinRoleCapabilities(t *testing.T) {
	want := map[Role]uint64{
		RoleViewer:      7,
		RoleContributor: 263,
		RoleEditor:      1799,
		RoleManager:     198407,
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
	for _, r := range []Role{RoleViewer, RoleContributor, RoleEditor, RoleManager} {
		if !r.Grantable() {
			t.Errorf("%s must be grantable", r)
		}
	}
	for _, r := range []Role{RoleOwner, RoleCustom, "NOPE", ""} {
		if r.Grantable() {
			t.Errorf("%s must not be grantable", r)
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
			continue // today identical to MANAGER; Describe labels it MANAGER
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
	editorNoDL, _ := GrantCaps(RoleEditor, true)
	if role, nd := Describe(editorNoDL); role != RoleEditor || !nd {
		t.Errorf("editor without download = %s, %v", role, nd)
	}
	if role, _ := Describe(CapView | CapDelete); role != RoleCustom {
		t.Errorf("odd combination must be custom, got %s", role)
	}
	if role, _ := Describe(0); role != RoleCustom {
		t.Errorf("no capabilities is custom, got %s", role)
	}
}

func TestParseRole(t *testing.T) {
	if r, ok := ParseRole("EDITOR"); !ok || r != RoleEditor {
		t.Error("EDITOR")
	}
	if _, ok := ParseRole("editor"); ok {
		t.Error("role names are case-sensitive")
	}
	if _, ok := ParseRole("NOPE"); ok {
		t.Error("unknown role")
	}
}
