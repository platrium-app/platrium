package authz

import (
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"
)

var (
	me    = Principal{TenantID: "t", UserID: "me"}
	other = Subject{Type: SubjectUser, ID: "other"}
	self  = Subject{Type: SubjectUser, ID: "me"}
	team  = Subject{Type: SubjectGroup, ID: "team"}
)

func g(s Subject, r Role) Grant {
	c, _ := r.Caps()
	return Grant{Subject: s, Role: r, Caps: c}
}

func until(g Grant) Grant {
	t := time.Now().Add(time.Hour)
	g.ExpiresAt = &t
	return g
}

// change is a base case: the actor manages an item in a shared drive.
func change(op ChangeOp, item ItemFacts, before, after []Grant) Change {
	caps, _ := RoleDriveAdmin.Caps()
	return Change{Op: op, Actor: me, ActorCaps: caps, Item: item, PublicSharingAllowed: true, Before: before, After: after}
}

var (
	root   = ItemFacts{ID: "root", IsDriveRoot: true, SharedDrive: true, Roles: ContextDriveMember}
	folder = ItemFacts{ID: "f", SharedDrive: true, Roles: ContextItemShare}
	mine   = ItemFacts{ID: "f", Roles: ContextItemShare} // in a private drive
)

func TestRules(t *testing.T) {
	admin := RoleDriveAdmin
	cases := []struct {
		name string
		c    Change
		want error
	}{
		// no-self-edit
		{"share with yourself", change(OpGrant, mine, nil, []Grant{g(self, RoleViewer)}), ErrInvalid},
		{"demote yourself", change(OpGrant, root, []Grant{g(self, admin), g(other, admin)}, []Grant{g(self, RoleViewer), g(other, admin)}), ErrInvalid},
		{"end-date yourself", change(OpGrant, root, []Grant{g(self, admin), g(other, admin)}, []Grant{until(g(self, admin)), g(other, admin)}), ErrInvalid},
		{"revoke yourself", change(OpRevoke, root, []Grant{g(self, admin), g(other, admin)}, []Grant{g(other, admin)}), ErrInvalid},
		{"re-share yourself unchanged", change(OpGrant, root, []Grant{g(self, admin), g(other, admin)}, []Grant{g(self, admin), g(other, admin)}), nil},
		{"restricting changes no grants", change(OpInheritance, folder, []Grant{g(other, RoleViewer)}, []Grant{g(other, RoleViewer)}), nil},
		{"first grant of a new drive", Change{Op: OpInitial, Item: root, Before: nil, After: []Grant{g(other, admin)}}, nil},

		// never-orphan
		{"revoke the last admin", change(OpRevoke, root, []Grant{g(other, admin)}, nil), ErrInvalid},
		{"demote the last admin", change(OpGrant, root, []Grant{g(other, admin)}, []Grant{g(other, RoleViewer)}), ErrInvalid},
		{"end-date the last admin", change(OpGrant, root, []Grant{g(other, admin)}, []Grant{until(g(other, admin))}), ErrInvalid},
		{"an expiring admin is not a manager", change(OpRevoke, root, []Grant{g(other, admin), until(g(self, admin))}, []Grant{until(g(self, admin))}), ErrInvalid},
		{"remove one of two admins", change(OpRevoke, root, []Grant{g(other, admin), g(team, admin)}, []Grant{g(team, admin)}), nil},
		{"a group is an admin", change(OpRevoke, root, []Grant{g(other, RoleViewer), g(team, admin)}, []Grant{g(other, RoleViewer)}), ErrInvalid},
		{"an item inside a drive needs no manager of its own", change(OpRevoke, folder, []Grant{g(other, RoleViewer)}, nil), nil},
		{"private drives cannot be orphaned", change(OpRevoke, ItemFacts{ID: "r", IsDriveRoot: true, Roles: ContextItemShare}, []Grant{g(other, RoleViewer)}, nil), nil},
		{"a broken item can be repaired", change(OpGrant, root, []Grant{g(other, RoleViewer)}, []Grant{g(other, RoleViewer), g(team, admin)}), nil},
		{"a broken item can lose a viewer", change(OpRevoke, root, []Grant{g(other, RoleViewer)}, nil), nil},

		// role-offered
		{"drive admin on a folder", change(OpGrant, folder, nil, []Grant{g(other, admin)}), ErrInvalid},
		{"editor on a folder", change(OpGrant, folder, nil, []Grant{g(other, RoleFullEditor)}), nil},
		{"commenter on a folder", change(OpGrant, folder, nil, []Grant{g(other, RoleCommenter)}), ErrInvalid},
		{"commenter on a drive", change(OpGrant, root, nil, []Grant{g(other, RoleCommenter)}), nil},
		{"editor for the organization", change(OpGeneralAccess, folder, nil, []Grant{g(Subject{Type: SubjectTenant, ID: "t"}, RoleFullEditor)}), nil},
		{"admin for the organization", change(OpGeneralAccess, folder, nil, []Grant{g(Subject{Type: SubjectTenant, ID: "t"}, admin)}), ErrInvalid},
		{"editor for the public", change(OpGeneralAccess, folder, nil, []Grant{g(Subject{Type: SubjectPublic, ID: PublicSubjectID}, RoleFullEditor)}), ErrInvalid},
		{"public links may expire", change(OpGeneralAccess, folder, nil, []Grant{until(g(Subject{Type: SubjectPublic, ID: PublicSubjectID}, RoleViewer))}), nil},

		// no-escalation
		{"grant what you hold", change(OpGrant, root, nil, []Grant{g(other, RoleFullEditor)}), nil},
		{"grant more than you hold", Change{Op: OpGrant, Actor: me, ActorCaps: mustCaps(RoleViewer), Item: root, Before: nil, After: []Grant{g(other, RoleFullEditor)}}, ErrForbidden},
		{"trusted code is not held to the actor's caps", Change{Op: OpInitial, Item: root, After: []Grant{g(self, admin), g(other, admin)}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckChange(tc.c)
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestPublicSharingSwitch(t *testing.T) {
	pub := g(Subject{Type: SubjectPublic, ID: PublicSubjectID}, RoleViewer)
	c := change(OpGeneralAccess, folder, nil, []Grant{pub})
	if err := CheckChange(c); err != nil {
		t.Fatal(err)
	}
	c.PublicSharingAllowed = false
	if err := CheckChange(c); !errors.Is(err, ErrForbidden) {
		t.Fatalf("public sharing off: %v", err)
	}
	// A public grant that is already there is not touched by an unrelated change.
	c.Before, c.After = []Grant{pub}, []Grant{pub, g(other, RoleViewer)}
	if err := CheckChange(c); err != nil {
		t.Fatalf("unrelated change next to an existing public grant: %v", err)
	}
	// Taking it away is always fine.
	c.Before, c.After = []Grant{pub}, nil
	if err := CheckChange(c); err != nil {
		t.Fatalf("removing public access: %v", err)
	}
}

func mustCaps(r Role) Capability { c, _ := r.Caps(); return c }

// The rules are the contract every adapter shares, so they must not reach into
// any one adapter's storage.
func TestRulesImportNothingStorageSpecific(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "rules.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		if strings.Contains(imp.Path.Value, "platrium/") || strings.Contains(imp.Path.Value, "sql") {
			t.Errorf("rules.go imports %s", imp.Path.Value)
		}
	}
}
