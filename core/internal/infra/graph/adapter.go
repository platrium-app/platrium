package graph

import "context"

// Graph defines the master database handle for GraphDB connections.
type Graph interface {
	ReadTx(ctx context.Context, fn func(tx Tx) error) error
	WriteTx(ctx context.Context, fn func(tx Tx) error) error
	Close(ctx context.Context) error
}

// Tx represents an isolated transactional context.
type Tx interface {
	Query(ctx context.Context, cypher string, params map[string]any) (Result, error)
	Exec(ctx context.Context, cypher string, params map[string]any) error
}

// Result handles rows streaming safely over the network to prevent memory spikes.
type Result interface {
	Next() bool
	// Scan maps the current row into a generic map or struct
	Scan(target any) error
	Err() error
	Close() error
}
