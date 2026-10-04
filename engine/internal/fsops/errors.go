package fsops

import (
	"errors"

	"platrium/internal/authz"
)

var (
	// ErrNotFound means the item does not exist or is not visible to the
	// caller. The two are deliberately indistinguishable.
	ErrNotFound = errors.New("not found")
	// ErrForbidden means the caller can see the item but lacks the capability
	// the operation needs.
	ErrForbidden = authz.ErrForbidden
	// ErrConflict means a uniqueness rule rejected the request.
	ErrConflict = authz.ErrConflict
	// ErrInvalid means the request is well-formed but not allowed, such as
	// moving a folder into its own subtree.
	ErrInvalid = errors.New("invalid operation")
)
