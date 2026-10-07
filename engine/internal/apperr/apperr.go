// Package apperr holds the sentinel errors every domain package shares, so a
// transport (GraphQL, REST) maps them to a status once instead of listing each
// package's own copy. Domain packages return these wrapped with context, such
// as fmt.Errorf("%w: item", apperr.ErrNotFound), and callers test with
// errors.Is. Packages may re-export them under their own names (see authz).
package apperr

import "errors"

var (
	// ErrForbidden means the caller may not do this.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound means the record does not exist or is not visible to the
	// caller. The two are deliberately indistinguishable.
	ErrNotFound = errors.New("not found")
	// ErrInvalid means the request is malformed or not allowed by the model.
	ErrInvalid = errors.New("invalid")
	// ErrConflict means a uniqueness rule rejected the write.
	ErrConflict = errors.New("conflict")
)
