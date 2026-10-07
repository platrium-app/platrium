package authz

import (
	"errors"

	"platrium/internal/apperr"
)

var (
	// ErrForbidden means the principal lacks the required capability.
	ErrForbidden = apperr.ErrForbidden
	// ErrNotFound means the item, grant or group does not exist or is not
	// visible to the caller. The two are deliberately indistinguishable.
	ErrNotFound = apperr.ErrNotFound
	// ErrInvalid means the request is malformed or not allowed by the model.
	ErrInvalid = apperr.ErrInvalid
	// ErrConflict means a uniqueness rule rejected the write.
	ErrConflict = apperr.ErrConflict

	// ErrDisabled means the principal's account has been disabled. Callers
	// treat it as "not signed in": the session is no longer good.
	ErrDisabled = errors.New("account disabled")
)
