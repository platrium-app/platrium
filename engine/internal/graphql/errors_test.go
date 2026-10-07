package graphql

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"platrium/internal/apperr"
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
		{"disabled", authz.ErrDisabled, "UNAUTHENTICATED"},
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
