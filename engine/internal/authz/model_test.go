package authz

import "testing"

func TestGeneralAccessOf(t *testing.T) {
	user := Grant{Subject: Subject{Type: SubjectUser, ID: "u"}}
	tenant := Grant{ID: "t", Subject: Subject{Type: SubjectTenant, ID: "x"}}
	public := Grant{ID: "p", Subject: Subject{Type: SubjectPublic, ID: PublicSubjectID}}

	if level, g := GeneralAccessOf(nil); level != AccessRestricted || g != nil {
		t.Errorf("no grants is restricted, got %s %v", level, g)
	}
	if level, g := GeneralAccessOf([]Grant{user}); level != AccessRestricted || g != nil {
		t.Errorf("a named user is still restricted, got %s %v", level, g)
	}
	if level, g := GeneralAccessOf([]Grant{user, tenant}); level != AccessTenant || g.ID != "t" {
		t.Errorf("tenant access: %s %v", level, g)
	}
	if level, g := GeneralAccessOf([]Grant{tenant, public}); level != AccessPublic || g.ID != "p" {
		t.Errorf("public wins if both somehow exist: %s %v", level, g)
	}
}
