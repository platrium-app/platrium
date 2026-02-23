package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

type Item struct {
	ent.Schema
}

func (Item) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("owner_id", uuid.UUID{}), // Dummy for MVP

		field.Enum("type").
			Values("file", "folder").
			Immutable(),

		field.String("name").
			NotEmpty().
			MaxLen(255),

		// File-only fields (null for folders)
		field.String("blob_id").
			Optional().
			Nillable(),

		field.Int64("size").
			Optional().
			Nillable().
			NonNegative(),

		field.String("mime_type").
			Optional().
			Nillable(),

		// Timestamps
		field.Time("created_at").
			Default(time.Now).
			Immutable(),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),

		// Soft delete
		field.Time("deleted_at").
			Optional().
			Nillable(),
	}
}

func (Item) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("deleted_at"),
		index.Fields("type", "owner_id"),
		index.Fields("blob_id"),
	}
}
