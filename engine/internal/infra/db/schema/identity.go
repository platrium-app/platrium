package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IdpProvider is the structural definition of an identity provider (OIDC,
// SAML or LOCAL) owned by exactly one tenant.
type IdpProvider struct{ ent.Schema }

func (IdpProvider) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (IdpProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.Enum("type").Values("OIDC", "SAML", "LOCAL"),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		// Protocol-specific JSON configuration; may hold client secrets.
		// Never queried into, never logged.
		field.Text("proto_config").Default("{}").Sensitive(),
	}
}

func (IdpProvider) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("idp_providers").
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("users", User.Type),
		edge.To("groups", Group.Type),
	}
}

func (IdpProvider) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id")}
}

// User is an identity belonging to exactly one tenant, bound to the IdP that
// authenticated it.
type User struct{ ent.Schema }

func (User) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("idp_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("external_id").MaxLen(nameLen).SchemaType(binaryType(nameLen)).NotEmpty().Immutable(),
		field.String("email").MaxLen(emailLen),
		field.String("display_name").MaxLen(nameLen),
		// Free-form so new roles don't need a schema change. The permissions
		// layer owns the vocabulary.
		field.String("role").MaxLen(32).Default("MEMBER"),
		// Set when an administrator disables the account; NULL while active.
		// A disabled user keeps their data and identity but cannot sign in,
		// and every credential they hold stops working at once.
		field.Time("disabled_at").SchemaType(timestampType).Optional().Nillable(),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("users").
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("idp", IdpProvider.Type).Ref("users").
			Field("idp_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("devices", Device.Type),
		edge.To("auth_tokens", AuthToken.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		// owner_id is nullable (tenant-owned drives), so the foreign key must be
		// declared here with an explicit action: the default SET NULL would also
		// forbid the owner_id CHECK on MySQL and MariaDB.
		edge.To("owned_drives", Drive.Type).
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.To("credential", LocalCredential.Type).Unique(),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		// One platform identity per (IdP, subject).
		index.Fields("idp_id", "external_id").Unique(),
		index.Fields("tenant_id", "email"),
	}
}

// Group mirrors a group from an IdP within a tenant.
type Group struct{ ent.Schema }

func (Group) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Group) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("idp_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("external_id").MaxLen(nameLen).SchemaType(binaryType(nameLen)).NotEmpty().Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
	}
}

func (Group) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("groups").
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		edge.From("idp", IdpProvider.Type).Ref("groups").
			Field("idp_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Group) Indexes() []ent.Index {
	return []ent.Index{index.Fields("idp_id", "external_id").Unique()}
}

// Domain is an email domain claimed by a tenant. Names are globally unique
// and stored lowercase.
type Domain struct{ ent.Schema }

func (Domain) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Domain) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		field.Bool("is_verified").Default(false),
	}
}

func (Domain) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("tenant", Tenant.Type).Ref("domains").
			Field("tenant_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
	}
}

func (Domain) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("tenant_id"),
	}
}

func (Domain) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Checks: map[string]string{
			"domain_name_lowercase": "name = LOWER(name)",
		}},
	}
}

// Device is a user's registered native client (iOS, macOS, Android app). It is
// created together with the AuthToken the client signs in with, and deleted
// with it. Push delivery details live here.
type Device struct{ ent.Schema }

func (Device) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (Device) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("user_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("name").MaxLen(nameLen).NotEmpty(),
		// IOS, MACOS, ANDROID, ... Free-form so new platforms need no migration.
		field.String("platform").MaxLen(32).NotEmpty(),
		field.String("app_version").MaxLen(64).Default(""),
		// Maps to notifications.TransportType (APNS or FCM). Null until the
		// client registers a push token.
		field.String("push_transport").MaxLen(32).Optional().Nillable(),
		field.String("push_token").MaxLen(1024).Optional().Nillable().Sensitive(),
	}
}

func (Device) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("devices").
			Field("user_id").Unique().Required().Immutable().
			Annotations(entsql.OnDelete(entsql.Restrict)),
		// The token cascades with its device, so deleting a device signs it out.
		edge.To("token", AuthToken.Type).Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Device) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "user_id")}
}
