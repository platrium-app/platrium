package fsops

import "errors"

var (
	// ErrNotFound means the item does not exist or is not visible in the
	// caller's tenant. The two are deliberately indistinguishable.
	ErrNotFound = errors.New("not found")
	// ErrInvalid means the request is well-formed but not allowed, such as
	// moving a folder into its own subtree.
	ErrInvalid = errors.New("invalid operation")
)
