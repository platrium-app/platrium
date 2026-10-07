package graphql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"platrium/internal/apperr"
	"platrium/internal/auth/actor"
	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
)

// Every domain package's sentinel must reach the client with a code. The
// domain packages alias apperr, so this fails if one is ever redefined.
func TestErrorPresenterCodesEverySentinel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"apperr forbidden", apperr.ErrForbidden, "FORBIDDEN"},
		{"authz forbidden", authz.ErrForbidden, "FORBIDDEN"},
		{"fsops forbidden", fsops.ErrForbidden, "FORBIDDEN"},
		{"apperr not found", apperr.ErrNotFound, "NOT_FOUND"},
		{"authz not found", authz.ErrNotFound, "NOT_FOUND"},
		{"fsops not found", fsops.ErrNotFound, "NOT_FOUND"},
		{"identity not found", identity.ErrNotFound, "NOT_FOUND"},
		{"apperr conflict", apperr.ErrConflict, "CONFLICT"},
		{"authz conflict", authz.ErrConflict, "CONFLICT"},
		{"fsops conflict", fsops.ErrConflict, "CONFLICT"},
		{"identity conflict", identity.ErrConflict, "CONFLICT"},
		{"apperr invalid", apperr.ErrInvalid, "BAD_REQUEST"},
		{"authz invalid", authz.ErrInvalid, "BAD_REQUEST"},
		{"fsops invalid", fsops.ErrInvalid, "BAD_REQUEST"},
		{"unauthenticated", actor.ErrUnauthenticated, "UNAUTHENTICATED"},
		{"wrapped", fmt.Errorf("%w: user", identity.ErrNotFound), "NOT_FOUND"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ErrorPresenter(context.Background(), c.err)
			if got.Extensions["code"] != c.want {
				t.Fatalf("code = %v, want %s", got.Extensions["code"], c.want)
			}
		})
	}
	if got := ErrorPresenter(context.Background(), errors.New("boom")); got.Extensions["code"] != nil {
		t.Fatalf("unmapped error got code %v", got.Extensions["code"])
	}
}

// Opening a subscription needs a user who is still enabled and still signed
// in, not merely a session that exists.
func TestSubscriptionRefusesStaleSessions(t *testing.T) {
	h := newHarness(t)
	s := &subscriptionResolver{h.r}

	if _, err := s.DriveItemChanged(anonymous()); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Errorf("no session: %v", err)
	}
	if err := h.db.User.UpdateOneID(h.bob).SetDisabledAt(time.Now()).Exec(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DriveItemChanged(h.as(h.bob)); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Errorf("disabled user: %v", err)
	}
	h.db.User.UpdateOneID(h.alice).SetSessionsValidAfter(time.Now().Add(time.Hour)).ExecX(context.Background())
	if _, err := s.DriveItemChanged(h.as(h.alice)); !errors.Is(err, actor.ErrUnauthenticated) {
		t.Errorf("signed out everywhere: %v", err)
	}
}
