package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LocalCredential holds the password and second-factor material for a user
// authenticated by the built-in LOCAL identity provider. It is isolated from
// User so identity reads never touch secrets.
type LocalCredential struct{ ent.Schema }

func (LocalCredential) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (LocalCredential) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("user_id").MaxLen(idLen).SchemaType(idType).Immutable().Unique(),
		field.String("password_hash").MaxLen(255).NotEmpty().Sensitive(),
		field.Time("password_changed_at").SchemaType(timestampType).Default(utcNow),
		field.String("totp_secret").MaxLen(255).Optional().Nillable().Sensitive(),
		field.JSON("backup_codes", []string{}).Optional().Sensitive(),
	}
}

func (LocalCredential) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("credential").
			Field("user_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (LocalCredential) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id")}
}
