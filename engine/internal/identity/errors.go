package identity

import "errors"

// Sentinel errors returned (wrapped) by stores so callers can map them to
// HTTP/GraphQL statuses with errors.Is instead of matching strings.
var (
	// ErrConflict means a uniqueness constraint rejected the write.
	ErrConflict = errors.New("conflict")
	// ErrNotFound means a referenced record does not exist (or is not visible
	// in the caller's tenant).
	ErrNotFound = errors.New("not found")
)
