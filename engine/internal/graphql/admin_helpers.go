package graphql

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	gqlgraphql "github.com/99designs/gqlgen/graphql"

	"platrium/internal/authz"
	"platrium/internal/identity"
	"platrium/internal/orchestrator"
)

const (
	defaultAdminPage = 50
	maxAdminPage     = 200
)

// Requires implements the @requires directive: the field runs only for a
// signed-in user whose role carries the permission. It is the declared gate;
// the orchestrators re-check, and also enforce the rules a directive cannot,
// such as which users the caller may act on.
func (r *Resolver) Requires(ctx context.Context, _ any, next gqlgraphql.Resolver, permission authz.Permission) (any, error) {
	// Identity also rejects a user who has been disabled since signing in.
	c, err := r.Actors.Identity(ctx)
	if err != nil {
		return nil, err
	}
	if !c.Perms.Has(permission) {
		return nil, fmt.Errorf("%w: requires the %s permission", authz.ErrForbidden, permission)
	}
	return next(ctx)
}

// Directives wires the schema's directives to their implementations.
func (r *Resolver) Directives() DirectiveRoot {
	return DirectiveRoot{Requires: r.Requires}
}

func mapAdminUser(m *orchestrator.ManagedUser) *AdminUser {
	u := m.UserListItem
	return &AdminUser{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		Disabled:    u.Disabled(),
		DisabledAt:  u.DisabledAt,
		CreatedAt:   u.CreatedAt,
		Source:      &IdentitySource{ID: u.IdpID, Name: u.IdpName, Type: u.IdpType, IsLocal: u.IsLocal()},
		Editable:    u.IsLocal(),
		Manageable:  m.Manageable,
	}
}

// The cursor is opaque to clients: the position of the last user returned.
func encodeUserCursor(u *orchestrator.ManagedUser) string {
	b, _ := json.Marshal(identity.UserCursor{DisplayName: u.DisplayName, ID: u.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeUserCursor(s *string) (*identity.UserCursor, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(*s)
	var c identity.UserCursor
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	if err != nil || c.ID == "" {
		return nil, fmt.Errorf("%w: invalid cursor", authz.ErrInvalid)
	}
	return &c, nil
}
