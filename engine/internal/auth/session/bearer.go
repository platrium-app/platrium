package session

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"platrium/internal/auth/token"
)

// AuthKind says how the current request authenticated.
type AuthKind string

const (
	AuthKindSession AuthKind = "SESSION" // browser cookie
	AuthKindDevice  AuthKind = "DEVICE"  // bearer token owned by a registered device
	AuthKindApp     AuthKind = "APP"     // bearer token owned by an app (CLI, FUSE, script)
)

// AuthInfo describes the credential behind the request's PlatriumSession.
type AuthInfo struct {
	Kind     AuthKind
	TokenID  string // empty for browser sessions
	DeviceID string // set for AuthKindDevice only
}

const authInfoContextKey contextKey = "auth_info"

// WithAuthInfo records how the request authenticated.
func WithAuthInfo(ctx context.Context, info AuthInfo) context.Context {
	return context.WithValue(ctx, authInfoContextKey, info)
}

// AuthInfoFromContext reports how the request authenticated. Requests with a
// session but no recorded info came from the browser cookie. ok is false for
// anonymous requests.
func AuthInfoFromContext(ctx context.Context) (AuthInfo, bool) {
	if info, ok := ctx.Value(authInfoContextKey).(AuthInfo); ok {
		return info, true
	}
	if _, ok := FromContext(ctx); ok {
		return AuthInfo{Kind: AuthKindSession}, true
	}
	return AuthInfo{}, false
}

// BearerFromHeader extracts the secret from an "Authorization: Bearer <secret>"
// header value.
func BearerFromHeader(h string) (string, bool) {
	scheme, secret, found := strings.Cut(strings.TrimSpace(h), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	secret = strings.TrimSpace(secret)
	return secret, secret != ""
}

// WithBearer validates a bearer secret and returns a context carrying the
// session and auth info it resolves to. It returns token.ErrInvalidToken for
// any secret that must be rejected.
func WithBearer(ctx context.Context, tokens *token.Store, secret string) (context.Context, error) {
	p, err := tokens.Validate(ctx, secret)
	if err != nil {
		return ctx, err
	}
	kind := AuthKindApp
	if p.DeviceID != "" {
		kind = AuthKindDevice
	}
	ctx = WithSession(ctx, &PlatriumSession{UserID: p.UserID, TenantID: p.TenantID, Email: p.Email})
	return WithAuthInfo(ctx, AuthInfo{Kind: kind, TokenID: p.TokenID, DeviceID: p.DeviceID}), nil
}

// Bearer authenticates requests that carry "Authorization: Bearer <token>" and
// resolves them to the same PlatriumSession a browser cookie would, so
// everything downstream is unchanged. An invalid token is rejected with 401;
// it never falls back to a cookie. Requests without a bearer header pass
// through untouched. Place it after Middleware.
func Bearer(tokens *token.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			secret, ok := BearerFromHeader(r.Header.Get("Authorization"))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			ctx, err := WithBearer(r.Context(), tokens, secret)
			if err != nil {
				status, msg := http.StatusUnauthorized, "Invalid or expired token"
				if !errors.Is(err, token.ErrInvalidToken) {
					log.Printf("bearer validation failed: %v", err)
					status, msg = http.StatusInternalServerError, "Internal error"
				}
				w.Header().Set("Content-Type", "application/json")
				if status == http.StatusUnauthorized {
					w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
