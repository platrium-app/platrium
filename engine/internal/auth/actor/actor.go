// Package actor turns the signed-in session into an authz.Principal.
package actor

import (
	"context"
	"errors"

	"platrium/internal/auth/session"
	"platrium/internal/authz"
)

// ErrUnauthenticated means the request carries no session.
var ErrUnauthenticated = errors.New("unauthorized")

// Principal resolves the session's user, with their current group
// memberships, into the principal that authorization checks run as. It reads
// membership fresh on every call, so a removed member loses access on their
// very next request. It is two indexed lookups.
func Principal(ctx context.Context, az authz.Authorizer) (authz.Principal, error) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return authz.Principal{}, ErrUnauthenticated
	}
	return az.Principal(ctx, sess.TenantID, sess.UserID)
}

// PrincipalOrAnonymous is Principal for operations open to visitors who are not
// signed in, such as opening a publicly shared item. Without a session the
// caller is anonymous and can only see what is shared publicly.
func PrincipalOrAnonymous(ctx context.Context, az authz.Authorizer) (authz.Principal, error) {
	if _, ok := session.FromContext(ctx); !ok {
		return authz.Anonymous(), nil
	}
	return Principal(ctx, az)
}
