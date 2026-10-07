// Package sqlauthz is the default authz.Authorizer. It stores grants and group
// membership in the relational database and evaluates them on Postgres, MySQL,
// MariaDB and SQLite. Checking a batch of items costs a few statements per
// chunk, however many items it holds: the items, their drives, one recursive
// ancestor walk for all of them, and one indexed grant lookup (plus a tenant
// lookup when a public grant turns up).
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
