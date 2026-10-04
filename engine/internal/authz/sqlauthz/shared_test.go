package sqlauthz_test

import (
	"context"
	"fmt"
	"testing"

	"platrium/internal/authz"
)

func TestSharedWithMe(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	team := e.group(t, s.tn, "team")
	if err := e.az.AddMember(ctx, s.tn.id, team, authz.MemberUser, s.bob); err != nil {
		t.Fatal(err)
	}
	pb := e.principal(t, s.tn, s.bob)

	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	e.share(t, s.pa, s.private, groupSubject(team), authz.RoleFullEditor)
	e.share(t, s.pa, s.notes, tenantSubject(s.tn), authz.RoleViewer) // org-wide: not listed
	e.share(t, s.pa, s.specs, publicSubject(), authz.RoleViewer)     // public: not listed

	got, err := e.az.SharedWithMe(ctx, pb, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]authz.Role{}
	for _, it := range got {
		roles[it.ItemID] = it.Role
	}
	if len(got) != 2 || roles[s.docs] != authz.RoleViewer || roles[s.private] != authz.RoleFullEditor {
		t.Fatalf("shared with bob = %+v", got)
	}

	// Entry points only: the files inside docs are not expanded.
	if _, expanded := roles[s.spec]; expanded {
		t.Error("descendants must not be listed")
	}
}

func TestSharedWithMeMergesAndFilters(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	team := e.group(t, s.tn, "team")
	if err := e.az.AddMember(ctx, s.tn.id, team, authz.MemberUser, s.bob); err != nil {
		t.Fatal(err)
	}
	pb := e.principal(t, s.tn, s.bob)

	// The same item shared two ways appears once, with the union.
	e.share(t, s.pa, s.docs, userSubject(s.bob), authz.RoleViewer)
	e.share(t, s.pa, s.docs, groupSubject(team), authz.RoleFullEditor)
	got, _ := e.az.SharedWithMe(ctx, pb, 50, "")
	if len(got) != 1 || !got[0].Caps.Has(authz.CapCreate) || got[0].Role != authz.RoleFullEditor {
		t.Fatalf("merged entry = %+v", got)
	}

	// Items in the principal's own drive are not "shared with" them.
	bobDrive := e.drive(t, s.tn, s.bob)
	e.share(t, pb, bobDrive, userSubject(s.bob), authz.RoleViewer)
	if got, _ := e.az.SharedWithMe(ctx, pb, 50, ""); len(got) != 1 {
		t.Fatalf("own drive must be excluded, got %+v", got)
	}

	// Expired grants drop out.
	g := e.share(t, s.pa, s.private, userSubject(s.bob), authz.RoleViewer)
	e.expire(t, g.ID)
	if got, _ := e.az.SharedWithMe(ctx, pb, 50, ""); len(got) != 1 {
		t.Fatalf("expired grants must be excluded, got %+v", got)
	}

	// Nothing for anonymous callers or for other tenants' users.
	if got, err := e.az.SharedWithMe(ctx, authz.Anonymous(), 50, ""); err != nil || len(got) != 0 {
		t.Fatalf("anonymous: %+v %v", got, err)
	}
	other := e.tenant(t, "other")
	if got, _ := e.az.SharedWithMe(ctx, e.principal(t, other, e.user(t, other, "x")), 50, ""); len(got) != 0 {
		t.Fatalf("another tenant must see nothing, got %+v", got)
	}
}

func TestSharedWithMePagination(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	s := newScene(t, e)
	for i := 0; i < 7; i++ {
		f := e.folder(t, s.tn, s.drive, s.drive, fmt.Sprintf("shared%d", i))
		e.share(t, s.pa, f, userSubject(s.bob), authz.RoleViewer)
	}

	seen := map[string]bool{}
	after := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("the cursor does not advance")
		}
		page, err := e.az.SharedWithMe(ctx, s.pb, 3, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 3 {
			t.Fatalf("page of %d exceeds the limit", len(page))
		}
		for _, it := range page {
			if seen[it.ItemID] {
				t.Fatalf("%s returned twice", it.ItemID)
			}
			seen[it.ItemID] = true
		}
		after = page[len(page)-1].ItemID
	}
	if len(seen) != 7 {
		t.Fatalf("paged %d items, want 7", len(seen))
	}
}
