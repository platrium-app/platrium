package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Grant gives a subject capabilities on an item and, unless an item stops
// inheriting, on everything beneath it.
type Grant struct{ ent.Schema }

func (Grant) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Grant) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("drive_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("resource_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		// Strings, not enums: new subject kinds must not need a schema change.
		// Values are validated in code. USER, GROUP, TENANT, PUBLIC.
		field.String("subject_type").MaxLen(16).NotEmpty().Immutable(),
		// User, group or tenant ID; "*" for PUBLIC.
		field.String("subject_id").MaxLen(idLen).SchemaType(idType).NotEmpty().Immutable(),
		// Label for display. caps is authoritative.
		field.String("role").MaxLen(32).Default("CUSTOM"),
		// Capability bitmask snapshot taken when the grant was written. Never
		// re-derived from the role, so adding a capability to a role cannot
		// widen existing grants. See authz.Capability.
		field.Int64("caps"),
		field.Time("expires_at").SchemaType(timestampType).Optional().Nillable(),
		// Opaque future conditions (IP ranges, time windows). Never queried.
		field.JSON("conditions", map[string]any{}).Optional(),
		// Who granted it. Plain ID, no foreign key: it outlives the user.
		field.String("created_by").MaxLen(idLen).SchemaType(idType).Optional().Nillable(),
	}
}

func (Grant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("drive", Drive.Type).
			Field("drive_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("resource", DriveItem.Type).
			Field("resource_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Grant) Indexes() []ent.Index {
	return []ent.Index{
		// One grant per subject per item; sharing again replaces it.
		index.Fields("resource_id", "subject_type", "subject_id").Unique(),
		// Shared-with-me: everything granted to a subject.
		index.Fields("tenant_id", "subject_type", "subject_id"),
		// Drive teardown deletes every grant of a drive in one statement.
		index.Fields("drive_id"),
	}
}

func (Grant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{"grant_caps_nonneg": "caps >= 0"}},
	}
}

// GroupMember is one direct membership edge: a user or a group inside a group.
type GroupMember struct{ ent.Schema }

func (GroupMember) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (GroupMember) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("group_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("member_type").MaxLen(16).NotEmpty().Immutable(), // USER or GROUP
		// A user or group ID. Polymorphic, so validated in code, not by a foreign key.
		field.String("member_id").MaxLen(idLen).SchemaType(idType).NotEmpty().Immutable(),
	}
}

func (GroupMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("group", Group.Type).
			Field("group_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (GroupMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "member_type", "member_id").Unique(),
		// Which groups directly contain this user or group.
		index.Fields("member_type", "member_id"),
	}
}

// GroupClosure is the flattened membership: one row per (group, user) for every
// user that belongs to the group directly or through nested groups. It is
// derived from GroupMember in the same transaction as every membership change,
// and lets principal resolution be one indexed lookup.
type GroupClosure struct{ ent.Schema }

func (GroupClosure) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}} }

func (GroupClosure) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("group_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("user_id").MaxLen(idLen).SchemaType(idType).Immutable(),
	}
}

func (GroupClosure) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("group", Group.Type).
			Field("group_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("user", User.Type).
			Field("user_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (GroupClosure) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "user_id").Unique(),
		// Principal resolution: every group of a user.
		index.Fields("user_id"),
	}
}

// ShareLink is a capability URL for an item. Whoever holds the token gets the
// link's capabilities on the item and its subtree, without signing in. Only a
// hash of the token is stored. Link creation and resolution arrive with the
// sharing APIs; this is the storage shape.
type ShareLink struct{ ent.Schema }

func (ShareLink) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (ShareLink) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("drive_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("resource_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		// Hex SHA-256 of the random token; the token itself is never stored.
		field.String("token_hash").MaxLen(64).SchemaType(binaryType(64)).NotEmpty().Immutable().Sensitive(),
		field.String("role").MaxLen(32).Default("CUSTOM"),
		field.Int64("caps"),
		field.String("password_hash").MaxLen(255).Optional().Nillable().Sensitive(),
		field.Time("expires_at").SchemaType(timestampType).Optional().Nillable(),
		field.Int64("max_uses").Optional().Nillable(),
		field.Int64("use_count").Default(0),
		field.Bool("disabled").Default(false),
		field.JSON("conditions", map[string]any{}).Optional(),
		field.String("created_by").MaxLen(idLen).SchemaType(idType).Optional().Nillable(),
	}
}

func (ShareLink) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("drive", Drive.Type).
			Field("drive_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("resource", DriveItem.Type).
			Field("resource_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (ShareLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token_hash").Unique(),
		index.Fields("resource_id"),
		index.Fields("drive_id"),
	}
}

func (ShareLink) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{
			"share_link_caps_nonneg":  "caps >= 0",
			"share_link_uses_nonneg":  "use_count >= 0",
			"share_link_max_uses_pos": "max_uses IS NULL OR max_uses > 0",
		}},
	}
}
