package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionContext(t *testing.T) {
	ctx := context.Background()

	// Initially, context should have no session
	if _, ok := FromContext(ctx); ok {
		t.Fatal("expected no session in context")
	}

	expected := &PlatriumSession{
		UserID:   "user-123",
		TenantID: "tenant-456",
		Email:    "test@example.com",
	}

	ctx = WithSession(ctx, expected)
	actual, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected session in context")
	}

	if actual.UserID != expected.UserID || actual.TenantID != expected.TenantID || actual.Email != expected.Email {
		t.Fatalf("session mismatch: got %+v, want %+v", actual, expected)
	}
}

func TestMiddleware_DevFallback(t *testing.T) {
	sm := NewManager()
	fallback := &PlatriumSession{
		UserID:   "admin-user",
		TenantID: "native-tenant",
		Email:    "admin@example.com",
	}

	var capturedSession *PlatriumSession
	handler := sm.LoadAndSave(Middleware(sm, fallback)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if ok {
			capturedSession = sess
		}
	})))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedSession == nil {
		t.Fatal("expected capturedSession to be populated by fallback")
	}
	if capturedSession.UserID != fallback.UserID || capturedSession.TenantID != fallback.TenantID {
		t.Fatalf("expected fallback session data, got %+v", capturedSession)
	}
}

func TestMiddleware_ActiveSession(t *testing.T) {
	sm := NewManager()
	activeSess := &PlatriumSession{
		UserID:   "custom-user",
		TenantID: "custom-tenant",
		Email:    "user@custom.com",
	}

	fallback := &PlatriumSession{
		UserID:   "fallback-user",
		TenantID: "fallback-tenant",
	}

	// First handler populates the session
	loginHandler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		PutSession(sm, r, activeSess)
		w.WriteHeader(http.StatusOK)
	}))

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/login", nil)
	loginHandler.ServeHTTP(rec1, req1)

	// Second handler reads the session via cookie
	cookie := rec1.Result().Header.Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("expected Set-Cookie header")
	}

	var captured *PlatriumSession
	readHandler := sm.LoadAndSave(Middleware(sm, fallback)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := FromContext(r.Context())
		if ok {
			captured = sess
		}
	})))

	req2 := httptest.NewRequest("GET", "/protected", nil)
	req2.Header.Set("Cookie", cookie)
	rec2 := httptest.NewRecorder()
	readHandler.ServeHTTP(rec2, req2)

	if captured == nil {
		t.Fatal("expected captured session from cookie")
	}
	if captured.UserID != activeSess.UserID || captured.TenantID != activeSess.TenantID {
		t.Fatalf("expected active session data, got %+v", captured)
	}
}
