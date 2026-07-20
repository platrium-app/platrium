package queue

import "context"

// Category defines the application namespace for distinct queue pools.
type Category string

const (
	// CatStorageBackendEvents handles asynchronous block check-ins from storage.
	CatStorageBackendEvents Category = "cat_storage_backend_events"
)

// Key acts as a type-safe, comparable struct key for our manager lookups.
type Key struct {
	Category Category
	Name     string // e.g., "s3-east-bucket" or "local-attached-disk"
}

type RawMessage struct {
	ID      string
	Payload []byte
}

// Adapter defines the direct interface to our messaging provider (SQS, RabbitMQ, local channels).
type Adapter interface {
	// FetchNext blocks and retrieves the next task from the broker.
	FetchNext(ctx context.Context) (*RawMessage, error)
	// Ack deletes or acknowledges the task after a successful consumer run.
	Ack(ctx context.Context, msgID string) error
	// Push injects a fresh payload into the queue (Producer mode).
	Push(ctx context.Context, payload []byte) error
	// Close cleanly severs the network or memory connection.
	Close() error
}
