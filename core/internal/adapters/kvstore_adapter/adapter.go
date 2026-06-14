package kvstore_adapter

import (
	"context"
	"strings"
)


// KVStore exposes the absolute minimal set of single-key operations.
type KVStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, opts ...SetOption) error
	Delete(ctx context.Context, key string) error
	Close() error
}

// BuildKey constructs a standard "namespace:part1:part2" key.
func BuildKey(namespace string, parts ...string) string {
	if len(parts) == 0 {
		return namespace
	}
	return namespace + ":" + strings.Join(parts, ":")
}
