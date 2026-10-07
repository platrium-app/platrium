package fsops

import "platrium/internal/apperr"

var (
	// ErrNotFound means the item does not exist or is not visible to the
	// caller. The two are deliberately indistinguishable.
	ErrNotFound = apperr.ErrNotFound
	// ErrForbidden means the caller can see the item but lacks the capability
	// the operation needs.
	ErrForbidden = apperr.ErrForbidden
	// ErrConflict means a uniqueness rule rejected the request.
	ErrConflict = apperr.ErrConflict
	// ErrInvalid means the request is well-formed but not allowed, such as
	// moving a folder into its own subtree.
	ErrInvalid = apperr.ErrInvalid
)
