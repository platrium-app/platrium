// Package actor turns the signed-in session into the identity a request runs as:
// who they are, what groups they are in, and what they may administer. It is
// the one place that decides whether a session is still good.
package actor

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"platrium/internal/auth/session"
	"platrium/internal/authz"
	"platrium/internal/identity"
)

// ErrUnauthenticated means the request carries no usable session: none at all,
// or one whose user has since been disabled, removed or signed out everywhere.
var ErrUnauthenticated = errors.New("unauthorized")

// Identity is the signed-in user a request runs as. It is resolved once per
// request, and every caller gets its own copy, so changing what you were handed
// affects nobody else. Perms is an immutable set and is shared safely.
type Identity struct {
	// Principal is who is acting for item checks, with their current groups.
	authz.Principal
	// Role is the user's tenant role. Check Perms, not the role.
	Role string
	// Perms is what the user may administer, with cluster permissions already
	// withheld unless they belong to the native tenant.
	Perms authz.PermissionSet
	// Native says the user belongs to the installation's native tenant.
	Native bool
}

// clone is a copy that shares nothing mutable with the original.
func (i *Identity) clone() *Identity {
	c := *i
	c.GroupIDs = slices.Clone(i.GroupIDs)
	return &c
}

// Resolver builds Identities from sessions.
type Resolver struct {
	az    authz.Resolver
	users *identity.UserStore
}

func NewResolver(az authz.Resolver, users *identity.UserStore) *Resolver {
	return &Resolver{az: az, users: users}
}

// Identity resolves the session's user, with their current role and group
// memberships. It reads them fresh, so a disabled, demoted or removed user
// loses access on their very next request. Inside a request that went through
// Middleware, the first call does the work and the rest reuse it.
func (r *Resolver) Identity(ctx context.Context) (*Identity, error) {
	s, ok := ctx.Value(slotKey{}).(*slot)
	if !ok {
		return r.resolve(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity == nil {
		c, err := r.resolve(ctx)
		if err != nil {
			return nil, err
		}
		s.identity = c
	}
	return s.identity.clone(), nil
}

// Principal is Identity for code that only checks items.
func (r *Resolver) Principal(ctx context.Context) (authz.Principal, error) {
	c, err := r.Identity(ctx)
	if err != nil {
		return authz.Principal{}, err
	}
	return c.Principal, nil
}

// PrincipalOrAnonymous is Principal for operations open to visitors who are not
// signed in, such as opening a publicly shared item. Without a session the
// caller is anonymous and can only see what is shared publicly.
func (r *Resolver) PrincipalOrAnonymous(ctx context.Context) (authz.Principal, error) {
	if _, ok := session.FromContext(ctx); !ok {
		return authz.Anonymous(), nil
	}
	p, err := r.Principal(ctx)
	if errors.Is(err, ErrUnauthenticated) {
		// A stale session (its user was disabled) is just a visitor.
		return authz.Anonymous(), nil
	}
	return p, err
}

// ForUser resolves a user who has no session on this request, such as the
// holder of a signed upload passport, and applies the same rules: a disabled or
// removed user is ErrUnauthenticated. Nothing is remembered.
func (r *Resolver) ForUser(ctx context.Context, tenantID, userID string) (*Identity, error) {
	return r.load(ctx, tenantID, userID, nil)
}

func (r *Resolver) resolve(ctx context.Context) (*Identity, error) {
	sess, ok := session.FromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	issued := sess.IssuedAt
	return r.load(ctx, sess.TenantID, sess.UserID, &issued)
}

// load reads the user's account and groups. issuedAt, when given, is when the
// session being resolved began; a revocation after it ends the session.
func (r *Resolver) load(ctx context.Context, tenantID, userID string, issuedAt *time.Time) (*Identity, error) {
	acc, err := r.users.Account(ctx, tenantID, userID)
	if errors.Is(err, identity.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if acc.Disabled() {
		return nil, ErrUnauthenticated
	}
	if issuedAt != nil && acc.SessionsValidAfter != nil && issuedAt.Before(*acc.SessionsValidAfter) {
		return nil, ErrUnauthenticated
	}
	p, err := r.az.Principal(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	return &Identity{
		Principal: p,
		Role:      acc.Role,
		Perms:     authz.EffectivePermissions(acc.Role, acc.Native),
		Native:    acc.Native,
	}, nil
}

type slotKey struct{}

// slot holds one request's resolved identity.
type slot struct {
	mu       sync.Mutex
	identity *Identity
}

// Middleware gives each request a place to remember its identity, so a GraphQL
// operation that checks a directive and then runs a resolver resolves once.
// Place it after the session middleware. WebSocket connections are left out:
// they outlive any one request, so every call there resolves fresh.
func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(w, req)
			return
		}
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), slotKey{}, &slot{})))
	})
}

// Invalidate forgets the request's remembered identity. Code that changes a
// user's role, disabled state or groups calls it after committing, so a later
// field of the same GraphQL operation sees the change.
func Invalidate(ctx context.Context) {
	if s, ok := ctx.Value(slotKey{}).(*slot); ok {
		s.mu.Lock()
		s.identity = nil
		s.mu.Unlock()
	}
}
