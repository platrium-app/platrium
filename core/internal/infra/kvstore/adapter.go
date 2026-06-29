package kvstore

import (
	"context"
)

// Namespace defines a strongly-typed enum for KV store prefixes.
type Namespace string

// These constants are used to globally guarantee collision-free prefixes.
const (
	NSAttachedFSWrites Namespace = "afsw"
	NSManifest         Namespace = "mfst"
	NSInstanceConfig   Namespace = "icfg"
)

// Key strictly enforces that every KV operation is Namespaced, completely
// preventing global namespace pollution at compile-time.
type Key struct {
	Namespace Namespace
	ID        string
}

// String returns the formatted key string for underlying DBs.
func (k Key) String() string {
	return string(k.Namespace) + ":" + k.ID
}

// Tx defines atomic transactional operations.
type Tx interface {
	Get(key Key) ([]byte, error)
	MultiGet(keys []Key) (map[string][]byte, error)
	Set(key Key, value []byte, opts ...SetOption) error
	MultiSet(kvs map[Key][]byte, opts ...SetOption) error
	Delete(key Key) error
	MultiDelete(keys []Key) error
}

// KVStore is the entrypoint. It forces all operations into secure transactional closures.
type KVStore interface {
	ReadTx(ctx context.Context, fn func(tx Tx) error) error
	WriteTx(ctx context.Context, fn func(tx Tx) error) error
	Close() error
}
