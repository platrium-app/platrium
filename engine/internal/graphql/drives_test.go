package graphql

import (
	"errors"
	"slices"
	"testing"

	"platrium/internal/authz"
)

func TestCreateSharedDriveResolver(t *testing.T) {
	h := newHarness(t)

	// A plain member may not; the UI learns that from canCreateSharedDrive.
	if ok, err := h.q().CanCreateSharedDrive(h.as(h.alice)); err != nil || ok {
		t.Fatalf("member: %v %v", ok, err)
	}
	if _, err := h.m().CreateSharedDrive(h.as(h.alice), "Nope"); !errors.Is(err, authz.ErrForbidden) || code(err) != "FORBIDDEN" {
		t.Fatalf("member create: %v", err)
	}
	if _, err := h.m().CreateSharedDrive(anonymous(), "Nope"); code(err) != "UNAUTHENTICATED" {
		t.Fatalf("signed out: %v", err)
	}

	if ok, _ := h.q().CanCreateSharedDrive(h.as(h.admin)); !ok {
		t.Fatal("admins may")
	}
	d, err := h.m().CreateSharedDrive(h.as(h.admin), "Finance")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Finance" || d.DriveMetadata == nil || d.DriveMetadata.DriveType != DriveTypeShared || !slices.Contains(d.MyCapabilities, "DELETE_DRIVE") {
		t.Fatalf("drive = %+v", d)
	}
	if _, err := h.m().CreateSharedDrive(h.as(h.admin), "finance"); code(err) != "CONFLICT" {
		t.Fatalf("duplicate: %v", err)
	}

	// It shows up in the creator's drives, and in nobody else's until shared.
	drives, _ := h.q().Drives(h.as(h.admin))
	if len(drives) != 1 || drives[0].ID != d.ID {
		t.Fatalf("admin drives: %+v", drives)
	}
	if drives, _ := h.q().Drives(h.as(h.bob)); len(drives) != 0 {
		t.Fatalf("bob: %+v", drives)
	}
	if _, err := h.m().ShareItem(h.as(h.admin), ShareInput{ItemID: d.ID, SubjectType: "USER", SubjectID: h.bob, Role: "FULL_EDITOR"}); err != nil {
		t.Fatal(err)
	}
	if drives, _ := h.q().Drives(h.as(h.bob)); len(drives) != 1 || !slices.Contains(drives[0].MyCapabilities, "MOVE") {
		t.Fatalf("bob after being added: %+v", drives)
	}
}

func TestSharedDriveCreatorGroups(t *testing.T) {
	h := newHarness(t)
	cfg, _ := h.r.TenantStore.GetPublicTenantAuthConfig(t.Context(), "acme")
	creators := mustGroup(t, h, cfg.Providers[0].ID, "Drive Creators")
	if err := h.r.Authz.AddMember(t.Context(), h.tenant, creators, authz.MemberUser, h.bob); err != nil {
		t.Fatal(err)
	}

	// Only admins manage the list.
	if _, err := h.m().SetSharedDriveCreators(h.as(h.bob), []string{creators}); code(err) != "FORBIDDEN" {
		t.Fatalf("member: %v", err)
	}
	if _, err := h.q().SharedDriveCreators(h.as(h.bob)); code(err) != "FORBIDDEN" {
		t.Fatalf("member read: %v", err)
	}

	got, err := h.m().SetSharedDriveCreators(h.as(h.admin), []string{creators})
	if err != nil || len(got) != 1 || got[0].Type != "GROUP" || got[0].Name != "Drive Creators" {
		t.Fatalf("set: %+v %v", got, err)
	}
	if read, _ := h.q().SharedDriveCreators(h.as(h.admin)); len(read) != 1 {
		t.Fatalf("read: %+v", read)
	}

	// Bob, in the group, may now create; carol, who is not, may not.
	if ok, _ := h.q().CanCreateSharedDrive(h.as(h.bob)); !ok {
		t.Error("a group member may create")
	}
	if ok, _ := h.q().CanCreateSharedDrive(h.as(h.carol)); ok {
		t.Error("carol is not in the group")
	}
	if _, err := h.m().CreateSharedDrive(h.as(h.bob), "Design"); err != nil {
		t.Fatalf("bob creates: %v", err)
	}

	if _, err := h.m().SetSharedDriveCreators(h.as(h.admin), []string{"nope"}); code(err) != "NOT_FOUND" {
		t.Fatalf("unknown group: %v", err)
	}
}
