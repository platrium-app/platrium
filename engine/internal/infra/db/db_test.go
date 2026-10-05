package db

import (
	"context"
	"errors"
	"os"
	"testing"

	"platrium/internal/infra/db/ent"
)

// testConfig returns an in-memory SQLite config, or a real backend when
// TEST_DB_DRIVER/TEST_DB_DSN are set.
func testConfig() Config {
	if drv := os.Getenv("TEST_DB_DRIVER"); drv != "" {
		return Config{Driver: drv, DSN: os.Getenv("TEST_DB_DSN"), MaxOpenConns: 10, MaxIdleConns: 2, AutoMigrate: true}
	}
	return Config{Driver: "sqlite", DSN: "file:memdb?mode=memory&cache=shared", AutoMigrate: true}
}

func TestOpenAndMigrate(t *testing.T) {
	d := newTestDB(t)

	if _, err := d.Tenant.Create().SetID("t1").SetAlias("acme").SetName("acme").Save(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWithTxCommitAndRollback(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)

	if err := d.WithTx(ctx, func(tx *ent.Tx) error {
		return tx.Tenant.Create().SetID("commit").SetAlias("commit").SetName("c").Exec(ctx)
	}); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("boom")
	err := d.WithTx(ctx, func(tx *ent.Tx) error {
		if err := tx.Tenant.Create().SetID("rollback").SetAlias("rollback").SetName("r").Exec(ctx); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}

	if ok, _ := d.Tenant.Get(ctx, "commit"); ok == nil {
		t.Fatal("committed row missing")
	}
	if _, err := d.Tenant.Get(ctx, "rollback"); !ent.IsNotFound(err) {
		t.Fatalf("rolled back row should not exist, err=%v", err)
	}
}

func TestResolveRejectsUnknownDriver(t *testing.T) {
	if _, err := Open(context.Background(), Config{Driver: "oracle"}); err == nil {
		t.Fatal("expected error")
	}
}
