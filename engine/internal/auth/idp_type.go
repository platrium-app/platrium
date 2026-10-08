package auth

// IdpType is the protocol an identity provider speaks. The values are the ones
// of the idp_providers.type column.
type IdpType string

// IsLocal reports whether this is the built-in provider type.
func (t IdpType) IsLocal() bool { return t == IdpTypeLocal }

const (
	// IdpTypeLocal is the built-in provider, whose users Platrium itself
	// manages. The other types are managed by an external provider.
	IdpTypeLocal IdpType = "LOCAL"
	IdpTypeOIDC  IdpType = "OIDC"
	// IdpTypeSAML is only implemented in the enterprise edition.
	IdpTypeSAML IdpType = "SAML"
)
