// Package schema holds the ent entity definitions for the relational store.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Tenant is a bootstrap placeholder so codegen has an entity to build.
// The full schema lands in Phase 1 of the SQL migration.
type Tenant struct {
	ent.Schema
}

func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").MaxLen(64).Immutable(),
		field.String("name").MaxLen(255),
	}
}
