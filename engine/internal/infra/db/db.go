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
	sqlDB *sql.DB
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
	d := &DB{Client: client, sqlDB: sqlDB}

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
	return d.runTx(ctx, fn)
}

func (d *DB) runTx(ctx context.Context, fn func(tx *ent.Tx) error) (err error) {
	tx, err := d.Tx(ctx)
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
