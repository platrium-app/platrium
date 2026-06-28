package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/caarlos0/env/v11"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type BoltConfig struct {
	// TODO: Make Required, Remove ENV Defaults!
	URI  string `env:"GRAPH_BOLT_URI" envDefault:"bolt://localhost:7687"`
	User string `env:"GRAPH_BOLT_USER" envDefault:"neo4j"`
	Pass string `env:"GRAPH_BOLT_PASSWORD" envDefault:"12345678"`
}

type BoltGraph struct {
	driver neo4j.DriverWithContext
}

var _ Graph = (*BoltGraph)(nil)

func NewBoltGraph() (*BoltGraph, error) {
	var cfg BoltConfig
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse bolt graph config: %w", err)
	}

	auth := neo4j.BasicAuth(cfg.User, cfg.Pass, "")
	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth)
	if err != nil {
		return nil, err
	}
	// Verify connectivity
	if err := driver.VerifyConnectivity(context.Background()); err != nil {
		return nil, err
	}
	return &BoltGraph{driver: driver}, nil
}

func (g *BoltGraph) ReadTx(ctx context.Context, fn func(tx Tx) error) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	_, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		boltTx := &BoltTx{tx: tx}
		return nil, fn(boltTx)
	})
	return err
}

func (g *BoltGraph) WriteTx(ctx context.Context, fn func(tx Tx) error) error {
	session := g.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		boltTx := &BoltTx{tx: tx}
		return nil, fn(boltTx)
	})
	return err
}

func (g *BoltGraph) Close(ctx context.Context) error {
	return g.driver.Close(ctx)
}

type BoltTx struct {
	tx neo4j.ManagedTransaction
}

var _ Tx = (*BoltTx)(nil)

func (t *BoltTx) Query(ctx context.Context, cypher string, params map[string]any) (Result, error) {
	res, err := t.tx.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}
	return &BoltResult{res: res, ctx: ctx}, nil
}

func (t *BoltTx) Exec(ctx context.Context, cypher string, params map[string]any) error {
	_, err := t.tx.Run(ctx, cypher, params)
	return err
}

type BoltResult struct {
	res neo4j.ResultWithContext
	ctx context.Context
	err error
}

var _ Result = (*BoltResult)(nil)

func (r *BoltResult) Next() bool {
	return r.res.Next(r.ctx)
}

func (r *BoltResult) Scan(target any) error {
	record := r.res.Record()
	if record == nil {
		return fmt.Errorf("no current record to scan")
	}

	recordMap := record.AsMap()

	// If the user explicitly asks for a raw map, pass it back directly
	if outMap, ok := target.(*map[string]any); ok {
		*outMap = recordMap
		return nil
	}

	// Ensure the target passed in is actually a pointer to a struct
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("Scan target must be a pointer to a struct or *map[string]any, got %T", target)
	}

	// Convert the generic map to intermediate JSON bytes
	jsonBytes, err := json.Marshal(recordMap)
	if err != nil {
		return fmt.Errorf("failed to marshal record map to json: %w", err)
	}

	// Unmarshal those bytes directly into the target struct pointer.
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal record into target struct %T: %w", target, err)
	}

	return nil
}

func (r *BoltResult) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.res.Err()
}

func (r *BoltResult) Close() error {
	// Not strictly required for Neo4j unless closing cursor early, but implemented for interface compliance
	return nil
}
