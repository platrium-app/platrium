package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Relationship struct {
	ent.Schema
}

func (Relationship) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("parent_id", uuid.UUID{}),

		field.UUID("child_id", uuid.UUID{}),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),

		// Soft delete
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

func (Relationship) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("parent", Item.Type).
			Field("parent_id").
			Unique().
			Required(),

		edge.To("child", Item.Type).
			Field("child_id").
			Unique(). // One parent only
			Required(),
	}
}

func (Relationship) Indexes() []ent.Index {
	return []ent.Index{
		// Enforce "One Parent Per Child" for active items only.
		// A child can have multiple parents in history (deleted), but only one active parent.
		index.Fields("child_id").
			Unique().
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),

		// Fast parent lookups
		index.Fields("parent_id"),

		index.Fields("deleted_at"),
	}
}
