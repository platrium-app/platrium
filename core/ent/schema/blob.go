package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Blob struct {
	ent.Schema
}

func (Blob) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Immutable().
			Unique(),

		field.String("sha256").
			NotEmpty().
			Unique(),

		field.Int64("size").
			NonNegative(),

		field.Int32("ref_count").
			Default(1).
			NonNegative(),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),

		field.Time("last_accessed_at").
			Optional().
			Nillable(),
	}
}

func (Blob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sha256"),
		index.Fields("ref_count"),
		index.Fields("ref_count", "last_accessed_at"),
	}
}
