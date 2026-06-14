package kvstore_adapter

import "time"

// SetOptions holds optional parameters for Set operations.
type SetOptions struct {
	TTL time.Duration
}

// SetOption represents a functional option for Set.
type SetOption func(*SetOptions)

// WithTTL sets a time-to-live for the key.
func WithTTL(ttl time.Duration) SetOption {
	return func(o *SetOptions) {
		o.TTL = ttl
	}
}
