package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AuthToken is a bearer credential held by a native client or script: a
// registered device (device_id set) or a named app such as the CLI. Only the
// SHA-256 of the secret is stored. Revoking a token deletes the row.
type AuthToken struct{ ent.Schema }

func (AuthToken) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (AuthToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("user_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		field.String("device_id").MaxLen(idLen).SchemaType(idType).Immutable().Optional().Nillable().Unique(),
		// Hex SHA-256 of the secret. Tokens are high-entropy, so no salt or
		// slow hash is needed.
		field.String("token_hash").MaxLen(64).SchemaType(binaryType(64)).NotEmpty().Immutable().Unique().Sensitive(),
		// First characters of the secret, safe to show in the UI.
		field.String("token_prefix").MaxLen(16).Immutable(),
		// Absolute expiry; null for tokens that only expire when idle.
		field.Time("expires_at").Optional().Nillable().Immutable().SchemaType(timestampType),
		field.Time("last_used_at").Default(utcNow).SchemaType(timestampType),
	}
}

func (AuthToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("auth_tokens").
			Field("user_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("device", Device.Type).Ref("token").
			Field("device_id").Unique().Immutable(),
	}
}

func (AuthToken) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "user_id")}
}
