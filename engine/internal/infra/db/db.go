// Package db provides the relational (ent) store used by the engine.
//
// Supported drivers: postgres, mysql, mariadb (via the mysql dialect) and
// sqlite (dev and tests only).
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/caarlos0/env/v11"
	"github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx"
	_ "modernc.org/sqlite"             // registers "sqlite"

	"platrium/internal/infra/db/ent"
	_ "platrium/internal/infra/db/ent/runtime" // schema defaults and hooks
)

type Config struct {
	Driver string `env:"DB_DRIVER,required"` // postgres | mysql | mariadb | sqlite
	DSN    string `env:"DB_DSN,required"`

	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS" envDefault:"5"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" envDefault:"30m"`

	// AutoMigrate creates/updates tables on startup.
	// TODO: Alpha only: replace with versioned migrations before the first deployment.
	AutoMigrate bool `env:"DB_AUTO_MIGRATE" envDefault:"true"`
}

// DB wraps the ent client and the underlying pool.
type DB struct {
	*ent.Client
	sqlDB   *sql.DB
	dialect string
}

// NewFromEnv parses the environment and opens the configured database.
func NewFromEnv(ctx context.Context) (*DB, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse db config: %w", err)
	}
	return Open(ctx, cfg)
}

// Open connects, verifies connectivity and (optionally) migrates the schema.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	driverName, dialectName, dsn, err := resolve(cfg)
	if err != nil {
		return nil, err
	}

	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s database: %w", cfg.Driver, err)
	}

	if dialectName == dialect.SQLite {
		// A single connection keeps in-memory databases coherent and avoids
		// SQLITE_BUSY under concurrent writers.
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to connect to %s database: %w", cfg.Driver, err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialectName, sqlDB)))
	d := &DB{Client: client, sqlDB: sqlDB, dialect: dialectName}

	if cfg.AutoMigrate {
		if err := client.Schema.Create(ctx); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to migrate schema: %w", err)
		}
	}
	return d, nil
}

// resolve maps the configured driver to a database/sql driver name, ent
// dialect and a normalized DSN.
func resolve(cfg Config) (driverName, dialectName, dsn string, err error) {
	switch strings.ToLower(cfg.Driver) {
	case "postgres", "postgresql":
		return "pgx", dialect.Postgres, cfg.DSN, nil
	case "mysql", "mariadb":
		my, perr := mysql.ParseDSN(cfg.DSN)
		if perr != nil {
			return "", "", "", fmt.Errorf("invalid mysql dsn: %w", perr)
		}
		// Required so DATETIME columns scan into time.Time, in UTC.
		my.ParseTime = true
		my.Loc = time.UTC
		return "mysql", dialect.MySQL, my.FormatDSN(), nil
	case "sqlite":
		d := cfg.DSN
		if !strings.Contains(d, "foreign_keys") {
			sep := "?"
			if strings.Contains(d, "?") {
				sep = "&"
			}
			d += sep + "_pragma=foreign_keys(1)"
		}
		return "sqlite", dialect.SQLite, d, nil
	default:
		return "", "", "", fmt.Errorf("unsupported db driver: %s", cfg.Driver)
	}
}

// Close releases the connection pool.
func (d *DB) Close() error {
	return d.Client.Close()
}

// WithTx runs fn inside a transaction, committing on success and rolling back
// on error or panic. Pass tx.Client() to stores so the same code runs inside
// or outside a transaction.
func (d *DB) WithTx(ctx context.Context, fn func(tx *ent.Tx) error) error {
	return d.runTx(ctx, nil, fn)
}

// WithTxOpts is WithTx with explicit transaction options. Use
// sql.LevelReadCommitted for transactions that take a row lock and then must
// observe other transactions' committed work: under MySQL/MariaDB's default
// REPEATABLE READ, reads after the lock would still see the pre-lock snapshot.
// SQLite ignores the requested isolation (it serializes writers).
func (d *DB) WithTxOpts(ctx context.Context, opts *sql.TxOptions, fn func(tx *ent.Tx) error) error {
	if opts != nil && d.dialect == dialect.SQLite {
		opts = nil
	}
	return d.runTx(ctx, opts, fn)
}

func (d *DB) runTx(ctx context.Context, opts *sql.TxOptions, fn func(tx *ent.Tx) error) (err error) {
	tx, err := d.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			err = fmt.Errorf("%w (rollback failed: %v)", err, rerr)
		}
		return err
	}
	return tx.Commit()
}

// Reset empties the given tables, ignoring foreign keys. It exists for test
// harnesses that share one real database between tests and must never be
// called outside tests.
func (d *DB) Reset(ctx context.Context, tables []string) error {
	conn, err := d.sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	switch d.dialect {
	case dialect.Postgres:
		quoted := make([]string, len(tables))
		for i, t := range tables {
			quoted[i] = `"` + t + `"`
		}
		_, err = conn.ExecContext(ctx, "TRUNCATE "+strings.Join(quoted, ", ")+" CASCADE")
		return err
	case dialect.MySQL:
		if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
			return err
		}
		defer conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")
		for _, t := range tables {
			if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE `"+t+"`"); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("reset is not supported for dialect %s", d.dialect)
	}
}

// RowLocks reports whether SELECT ... FOR UPDATE is available. SQLite has no
// row locks, but it serializes writers, so skipping the lock there is safe.
// Stores use it to take the one portable concurrency primitive (locking a
// parent row) only where it exists:
//
//	q := tx.Tenant.Query().Where(...)
//	if d.RowLocks() { q = q.ForUpdate() }
func (d *DB) RowLocks() bool {
	return d.dialect != dialect.SQLite
}

// Rebind rewrites '?' placeholders to the backend's native form ($1, $2, ...
// on Postgres). Use it for the few raw SQL queries; ent's builders already
// handle placeholders. Only use it on queries that contain no literal '?'.
func (d *DB) Rebind(query string) string {
	if d.dialect != dialect.Postgres {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
