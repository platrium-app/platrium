package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IdpOIDCConfig is the OpenID Connect settings of an OIDC identity provider.
// The provider is found by discovery from the issuer, so nothing else about
// its endpoints is stored. It is isolated from IdpProvider so provider reads
// (the login picker, user listings) never select the client secret.
type IdpOIDCConfig struct{ ent.Schema }

func (IdpOIDCConfig) Mixin() []ent.Mixin { return []ent.Mixin{IDMixin{}, TimeMixin{}} }

func (IdpOIDCConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("tenant_id").MaxLen(idLen).SchemaType(idType).Immutable(),
		field.String("idp_id").MaxLen(idLen).SchemaType(idType).Immutable().Unique(),
		// The issuer URL exactly as the provider reports it (Auth0 ends in "/").
		field.String("issuer").MaxLen(512).NotEmpty(),
		field.String("client_id").MaxLen(nameLen).NotEmpty(),
		// Sealed with secrets.Keyring (enc:v1:...), bound to idp_id. Never the
		// plaintext, never returned by an API, never logged.
		field.Text("client_secret").NotEmpty().Sensitive(),
		// Scopes requested in addition to "openid".
		field.JSON("scopes", []string{}).Optional(),
		// Which ID-token / userinfo claims carry each attribute.
		field.String("email_claim").MaxLen(64).Default("email"),
		field.String("name_claim").MaxLen(64).Default("name"),
		field.String("picture_claim").MaxLen(64).Default("picture"),
		// Refuse sign-ins whose email the provider has not verified.
		field.Bool("require_email_verified").Default(true),
		// Extra authorization-request parameters, e.g. Auth0's "audience".
		field.JSON("extra_auth_params", map[string]string{}).Optional(),
	}
}

func (IdpOIDCConfig) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("idp", IdpProvider.Type).Ref("oidc_config").
			Field("idp_id").Unique().Required().Immutable(),
	}
}

func (IdpOIDCConfig) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id")}
}

func (IdpOIDCConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "idp_oidc_configs"}}
}
