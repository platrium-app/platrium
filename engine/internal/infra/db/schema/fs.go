package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Drive is the root container of a file tree. Its root folder is a DriveItem
// that shares the drive's ID, so a drive ID can be used anywhere a folder ID
// is accepted and no circular foreign key is needed.
//
// A drive is owned either by a user (a private drive: the owner holds every
// capability and the drive goes when the user does) or by its tenant (a shared
// drive: nobody owns it implicitly, access comes only from grants, and it
// outlives any member).
type Drive struct{ ent.Schema }

func (Drive) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Drive) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		// USER or TENANT. A string so new owner kinds need no schema change.
		field.String("owner_type").MaxLen(16).Default("USER").Immutable(),
		// The owning user; NULL for tenant-owned drives (enforced by a CHECK).
		field.String("owner_id").MaxLen(idLen).SchemaType(idType).Optional().Nillable().Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		field.Enum("type").Values("PRIVATE", "SHARED").Immutable(),
		field.Int64("storage_used").Default(0),
		// NULL means unlimited.
		field.Int64("storage_quota").Optional().Nillable(),
	}
}

func (Drive) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("drives").
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("owner", User.Type).Ref("owned_drives").
			Field("owner_id").Unique().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("items", DriveItem.Type),
	}
}

func (Drive) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "owner_id")}
}

func (Drive) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{
			"drive_owner_matches_type":   "owner_type = 'USER' AND owner_id IS NOT NULL OR owner_type = 'TENANT' AND owner_id IS NULL",
			"drive_storage_used_nonneg":  "storage_used >= 0",
			"drive_storage_quota_nonneg": "storage_quota IS NULL OR storage_quota >= 0",
		}},
	}
}

// DriveItem is a node in a drive's tree: a folder or a file. One table keeps
// the hot paths (list children, move, rename, ancestor walks) single-query.
type DriveItem struct{ ent.Schema }

func (DriveItem) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (DriveItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("drive_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		// NULL only for a drive's root folder (whose ID equals the drive ID).
		field.String("parent_id").MaxLen(idLen).SchemaType(idType).Optional().Nillable(),
		field.Enum("kind").Values("FOLDER", "FILE").Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		// When false, grants on ancestors stop applying here: the item is
		// restricted to its own grants and the owner.
		field.Bool("inherit_perms").Default(true),

		// File-only columns; NULL for folders (enforced by a CHECK).
		field.Int64("size").Optional().Nillable(),
		field.String("mime_type").MaxLen(255).Optional().Nillable(),
		// Hex chunk hashes for small files; larger files page their manifest
		// into the KV store. Opaque, never queried into.
		field.JSON("inline_chunks", []string{}).Optional(),
	}
}

func (DriveItem) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("drive", Drive.Type).Ref("items").
			Field("drive_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		// For self-referencing edges ent reads the FK action from the To side.
		edge.To("children", DriveItem.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)).
			From("parent").Field("parent_id").Unique(),
	}
}

func (DriveItem) Indexes() []ent.Index {
	return []ent.Index{
		// Folder listing and keyset pagination.
		index.Fields("tenant_id", "parent_id", "id"),
		// Change feeds.
		index.Fields("tenant_id", "parent_id", "updated_at"),
		index.Fields("drive_id"),
	}
}

func (DriveItem) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{
			// Only a drive's root has no parent, and its ID is the drive's ID.
			"drive_item_root": "parent_id IS NOT NULL OR id = drive_id",
			// File metadata exists only on files.
			"drive_item_file_columns": "kind = 'FILE' OR (size IS NULL AND mime_type IS NULL AND inline_chunks IS NULL)",
			"drive_item_size_nonneg":  "size IS NULL OR size >= 0",
		}},
	}
}
