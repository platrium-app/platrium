package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Tenant is an organization or isolated billing unit.
type Tenant struct{ ent.Schema }

func (Tenant) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		// Globally unique, always stored lowercase (enforced by a CHECK) so
		// uniqueness behaves identically on case-sensitive and
		// case-insensitive collations.
		field.String("alias").MaxLen(nameLen).NotEmpty(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		// native_slot is 1 for the native (cluster) tenant and NULL otherwise.
		// UNIQUE ignores NULLs on every supported backend, so this is a
		// portable "at most one native tenant" constraint without partial
		// indexes. Use IsNative() semantics (native_slot != nil) in stores.
		field.Int("native_slot").Optional().Nillable().Unique(),
		// Whether items may be shared with "anyone with the link". Checked when
		// such a grant is created and again whenever it is evaluated, so turning
		// it off ends existing public access at once.
		field.Bool("allow_public_sharing").Default(true),
	}
}

func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("users", User.Type),
		edge.To("groups", Group.Type),
		edge.To("domains", Domain.Type),
		edge.To("idp_providers", IdpProvider.Type),
		edge.To("drives", Drive.Type),
	}
}

func (Tenant) Indexes() []ent.Index {
	return []ent.Index{index.Fields("alias").Unique()}
}

func (Tenant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{
			"tenant_alias_lowercase": "alias = LOWER(alias)",
			"tenant_native_slot":     "native_slot IS NULL OR native_slot = 1",
		}},
	}
}
