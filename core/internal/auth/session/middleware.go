package session

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
)

const defaultSessionStoreKey = "default_sessioninfo"

// Middleware returns an HTTP middleware that extracts the PlatriumSession struct from SCS
// and injects it into the request context.
// If devFallback is non-nil and no active session exists in request, devFallback will be used instead.
func Middleware(sm *scs.SessionManager, devFallback *PlatriumSession) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			var sess *PlatriumSession
			if val := sm.Get(ctx, defaultSessionStoreKey); val != nil {
				if s, ok := val.(*PlatriumSession); ok {
					sess = s
				}
			}

			if sess == nil && devFallback != nil {
				sess = devFallback
				PutSession(sm, r, sess)
			}

			if sess != nil {
				ctx = WithSession(ctx, sess)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// PutSession saves a PlatriumSession object directly into the active SCS session.
func PutSession(sm *scs.SessionManager, r *http.Request, sess *PlatriumSession) {
	sm.Put(r.Context(), defaultSessionStoreKey, sess)
}
