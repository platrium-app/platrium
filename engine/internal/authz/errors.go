package authz

import "errors"

var (
	// ErrForbidden means the principal lacks the required capability.
	ErrForbidden = errors.New("forbidden")
	// ErrDisabled means the principal's account has been disabled. Callers
	// treat it as "not signed in": the session is no longer good.
	ErrDisabled = errors.New("account disabled")
	// ErrNotFound means the item, grant or group does not exist or is not
	// visible to the caller. The two are deliberately indistinguishable.
	ErrNotFound = errors.New("not found")
	// ErrInvalid means the request is malformed or not allowed by the model.
	ErrInvalid = errors.New("invalid")
	// ErrConflict means a uniqueness rule rejected the write.
	ErrConflict = errors.New("conflict")
)
