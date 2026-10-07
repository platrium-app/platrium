package identity

import "platrium/internal/apperr"

// Sentinel errors returned (wrapped) by stores so callers can map them to
// HTTP/GraphQL statuses with errors.Is instead of matching strings.
var (
	// ErrConflict means a uniqueness constraint rejected the write.
	ErrConflict = apperr.ErrConflict
	// ErrNotFound means a referenced record does not exist (or is not visible
	// in the caller's tenant).
	ErrNotFound = apperr.ErrNotFound
)
