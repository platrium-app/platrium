// Package dbtest provides isolated databases for tests in other packages.
//
// By default each call gets a private in-memory SQLite database. Set
// TEST_DB_DRIVER and TEST_DB_DSN to run against a real Postgres, MySQL or
// MariaDB instead; tables are wiped before each test.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"platrium/internal/infra/db"
)

// tables lists every table, used to wipe a shared real database between tests.
var tables = []string{"drive_items", "drives", "devices", "domains", "groups", "local_credentials", "users", "idp_providers", "tenants"}

// New opens a migrated, empty database that is closed when the test ends.
func New(t testing.TB) *db.DB {
	t.Helper()

	cfg := db.Config{Driver: "sqlite", AutoMigrate: true,
		DSN: fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()))}
	if drv := os.Getenv("TEST_DB_DRIVER"); drv != "" {
		cfg = db.Config{Driver: drv, DSN: os.Getenv("TEST_DB_DSN"), MaxOpenConns: 10, MaxIdleConns: 2, AutoMigrate: true}
	}

	d, err := db.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	if cfg.Driver != "sqlite" {
		if err := d.Reset(context.Background(), tables); err != nil {
			t.Fatal(err)
		}
	}
	return d
}
