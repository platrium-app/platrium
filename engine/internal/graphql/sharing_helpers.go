package graphql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"platrium/internal/auth/actor"
	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
)

// ErrorPresenter adds a stable machine-readable code to errors, so clients can
// react to "not found" and "forbidden" without parsing messages.
func ErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	gerr := gqlgraphql.DefaultErrorPresenter(ctx, err)

	code := ""
	switch {
	case errors.Is(err, actor.ErrUnauthenticated):
		code = "UNAUTHENTICATED"
	case errors.Is(err, authz.ErrForbidden): // fsops.ErrForbidden is the same error
		code = "FORBIDDEN"
	case errors.Is(err, authz.ErrNotFound), errors.Is(err, fsops.ErrNotFound), errors.Is(err, identity.ErrNotFound):
		code = "NOT_FOUND"
	case errors.Is(err, authz.ErrConflict), errors.Is(err, identity.ErrConflict):
		code = "CONFLICT"
	case errors.Is(err, authz.ErrInvalid), errors.Is(err, fsops.ErrInvalid):
		code = "BAD_REQUEST"
	}
	if code != "" {
		if gerr.Extensions == nil {
			gerr.Extensions = map[string]any{}
		}
		gerr.Extensions["code"] = code
	}
	return gerr
}

const (
	publicSubjectName = "Anyone with the link"
	maxSharedPage     = 100

	minSearchLength   = 2
	defaultSearchPage = 8
	maxSearchPage     = 25
)

// subjectNames resolves display names for the subjects of a set of grants:
// people, groups and the organization. One batched lookup per kind.
func (r *Resolver) subjectNames(ctx context.Context, tenantID string, grants []authz.Grant) (map[string]string, error) {
	var userIDs, groupIDs []string
	wantTenant := false
	for _, g := range grants {
		switch g.Subject.Type {
		case authz.SubjectUser:
			userIDs = append(userIDs, g.Subject.ID)
		case authz.SubjectGroup:
			groupIDs = append(groupIDs, g.Subject.ID)
		case authz.SubjectTenant:
			wantTenant = true
		}
	}

	names := map[string]string{}
	users, err := r.UserStore.GetByIDs(ctx, tenantID, userIDs)
	if err != nil {
		return nil, err
	}
	for id, u := range users {
		name := u.DisplayName
		if name == "" {
			name = u.Email
		}
		names[subjectKey(authz.SubjectUser, id)] = name
	}
	groups, err := r.GroupStore.GetByIDs(ctx, tenantID, groupIDs)
	if err != nil {
		return nil, err
	}
	for id, g := range groups {
		names[subjectKey(authz.SubjectGroup, id)] = g.Name
	}
	if wantTenant {
		t, err := r.TenantStore.GetTenant(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		names[subjectKey(authz.SubjectTenant, tenantID)] = t.Name
	}
	return names, nil
}

func subjectKey(t authz.SubjectType, id string) string { return string(t) + ":" + id }

func mapGrant(g authz.Grant, names map[string]string) *AccessGrant {
	role, noDownload := authz.Describe(g.Caps)

	name := names[subjectKey(g.Subject.Type, g.Subject.ID)]
	if g.Subject.Type == authz.SubjectPublic {
		name = publicSubjectName
	}
	return &AccessGrant{
		ID:           g.ID,
		SubjectType:  string(g.Subject.Type),
		SubjectID:    g.Subject.ID,
		SubjectName:  name,
		Role:         string(role),
		NoDownload:   noDownload,
		Capabilities: g.Caps.Verbs(),
		ExpiresAt:    g.ExpiresAt,
		CreatedAt:    g.CreatedAt,
	}
}

// mapGeneralAccess describes an item's general access from the grant behind it.
func mapGeneralAccess(level authz.GeneralAccessLevel, g *authz.Grant) *GeneralAccess {
	out := &GeneralAccess{Level: string(level)}
	if g != nil {
		role, noDownload := authz.Describe(g.Caps)
		r := string(role)
		out.Role = &r
		out.NoDownload = noDownload
		out.ExpiresAt = g.ExpiresAt
	}
	return out
}

// parseRole parses a role name from a client.
func parseRole(s string) (authz.Role, error) {
	r, ok := authz.ParseRole(strings.ToUpper(strings.TrimSpace(s)))
	if !ok || !r.Grantable() {
		return "", fmt.Errorf("%w: %q is not a role that can be granted", authz.ErrInvalid, s)
	}
	return r, nil
}

// ownerSubject describes who owns a drive: a person for a private drive, or
// the organization when ownerUserID is empty (a shared drive).
func (r *Resolver) ownerSubject(ctx context.Context, tenantID, ownerUserID string) (*DirectorySubject, error) {
	if ownerUserID == "" {
		t, err := r.TenantStore.GetTenant(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		return &DirectorySubject{Type: string(authz.SubjectTenant), ID: t.ID, Name: t.Name}, nil
	}
	users, err := r.UserStore.GetByIDs(ctx, tenantID, []string{ownerUserID})
	if err != nil {
		return nil, err
	}
	u, ok := users[ownerUserID]
	if !ok {
		return &DirectorySubject{Type: string(authz.SubjectUser), ID: ownerUserID, Name: "Unknown user"}, nil
	}
	name, email := u.DisplayName, u.Email
	if name == "" {
		name = email
	}
	return &DirectorySubject{Type: string(authz.SubjectUser), ID: u.ID, Name: name, Email: &email}, nil
}

// creatorGroups lists the groups allowed to create shared drives, with names.
func (r *Resolver) creatorGroups(ctx context.Context, p authz.Principal) ([]*DirectorySubject, error) {
	ids, err := r.DriveOrch.SharedDriveCreators(ctx, p)
	if err != nil {
		return nil, err
	}
	groups, err := r.GroupStore.GetByIDs(ctx, p.TenantID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*DirectorySubject, 0, len(ids))
	for _, id := range ids {
		if g, ok := groups[id]; ok {
			out = append(out, &DirectorySubject{Type: string(authz.SubjectGroup), ID: g.ID, Name: g.Name})
		}
	}
	return out, nil
}
