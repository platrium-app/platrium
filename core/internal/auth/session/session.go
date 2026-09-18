package session

import (
	"context"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
)

func init() {
	gob.Register(&PlatriumSession{})
}

type contextKey string

const (
	sessionContextKey contextKey = "auth_session"
)

// PlatriumSession holds user session state.
type PlatriumSession struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
}

// NewManager initializes a new SCS SessionManager with platrium_sid cookie settings.
func NewManager() *scs.SessionManager {
	sm := scs.New()
	sm.Lifetime = 24 * time.Hour
	sm.Cookie.Name = "platrium_sid"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = false
	return sm
}

// WithSession injects a PlatriumSession into the context.
func WithSession(ctx context.Context, sess *PlatriumSession) context.Context {
	return context.WithValue(ctx, sessionContextKey, sess)
}

// FromContext retrieves a PlatriumSession from the context.
func FromContext(ctx context.Context) (*PlatriumSession, bool) {
	sess, ok := ctx.Value(sessionContextKey).(*PlatriumSession)
	return sess, ok && sess != nil
}
