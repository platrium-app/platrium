// Package sqlauthz is the default authz.Authorizer. It stores grants and group
// membership in the relational database and evaluates them with one recursive
// ancestor query plus one indexed grant lookup, on Postgres, MySQL, MariaDB and
// SQLite.
package sqlauthz

import (
	"platrium/internal/authz"
	"platrium/internal/infra/db"
)

// Authorizer implements authz.Authorizer on SQL.
type Authorizer struct {
	db *db.DB
}

var _ authz.Authorizer = (*Authorizer)(nil)

// New returns the SQL authorizer.
func New(d *db.DB) *Authorizer { return &Authorizer{db: d} }
