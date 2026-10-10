package graphql

import (
	"errors"
	"fmt"
	"sort"

	"platrium/internal/auth/protocol/oidc"
	"platrium/internal/authz"
	"platrium/internal/orchestrator"
)

func mapIdentityProvider(m *orchestrator.ManagedIdp) *IdentityProvider {
	out := &IdentityProvider{
		ID:                  m.ID,
		Name:                m.Name,
		Type:                string(m.Type),
		IsLocal:             m.IsLocal(),
		Enabled:             m.Enabled,
		JitUsers:            m.JITUsers,
		DefaultRole:         m.DefaultRole,
		AllowedEmailDomains: nonNil(m.AllowedEmailDomains),
		UserCount:           m.UserCount,
	}
	if m.OIDC != nil {
		out.Config = &OidcConfig{
			Issuer:               m.OIDC.Issuer,
			ClientID:             m.OIDC.ClientID,
			Scopes:               nonNil(m.OIDC.Scopes),
			EmailClaim:           m.OIDC.EmailClaim,
			NameClaim:            m.OIDC.NameClaim,
			PictureClaim:         m.OIDC.PictureClaim,
			RequireEmailVerified: m.OIDC.RequireEmailVerified,
			ExtraAuthParams:      authParamsOut(m.OIDC.ExtraAuthParams),
			RedirectURI:          m.OIDCRedirectURI,
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func authParamsOut(m map[string]string) []*AuthParam {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]*AuthParam, 0, len(names))
	for _, k := range names {
		out = append(out, &AuthParam{Name: k, Value: m[k]})
	}
	return out
}

func authParamsIn(in []*AuthParamInput) (map[string]string, error) {
	m := make(map[string]string, len(in))
	for _, p := range in {
		if p.Name == "" {
			return nil, fmt.Errorf("%w: an authorization parameter needs a name", authz.ErrInvalid)
		}
		if _, dup := m[p.Name]; dup {
			return nil, fmt.Errorf("%w: authorization parameter %q is given twice", authz.ErrInvalid, p.Name)
		}
		m[p.Name] = p.Value
	}
	return m, nil
}

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// oidcConfigFromInput maps a create input to the config the store validates.
func oidcConfigFromInput(in *OidcConfigInput) (oidc.Config, error) {
	extra, err := authParamsIn(in.ExtraAuthParams)
	if err != nil {
		return oidc.Config{}, err
	}
	return oidc.Config{
		Issuer:               in.Issuer,
		ClientID:             in.ClientID,
		ClientSecret:         in.ClientSecret,
		Scopes:               in.Scopes,
		EmailClaim:           deref(in.EmailClaim, ""),
		NameClaim:            deref(in.NameClaim, ""),
		PictureClaim:         deref(in.PictureClaim, ""),
		RequireEmailVerified: deref(in.RequireEmailVerified, true),
		ExtraAuthParams:      extra,
	}, nil
}

func oidcUpdateFromInput(in UpdateOidcConfigInput) (oidc.UpdateParams, error) {
	p := oidc.UpdateParams{
		ClientID:             in.ClientID,
		ClientSecret:         in.ClientSecret,
		EmailClaim:           in.EmailClaim,
		NameClaim:            in.NameClaim,
		PictureClaim:         in.PictureClaim,
		RequireEmailVerified: in.RequireEmailVerified,
	}
	if in.Scopes != nil {
		p.Scopes = &in.Scopes
	}
	if in.ExtraAuthParams != nil {
		extra, err := authParamsIn(in.ExtraAuthParams)
		if err != nil {
			return p, err
		}
		p.ExtraAuthParams = &extra
	}
	return p, nil
}

func mapDiscovery(d *oidc.Discovery, err error) (*OidcDiscoveryResult, error) {
	if errors.Is(err, oidc.ErrDiscovery) {
		msg := err.Error()
		return &OidcDiscoveryResult{Ok: false, Message: &msg}, nil
	}
	if err != nil {
		return nil, err
	}
	return &OidcDiscoveryResult{
		Ok:                    true,
		AuthorizationEndpoint: &d.AuthorizationEndpoint,
		TokenEndpoint:         &d.TokenEndpoint,
		JwksURI:               &d.JWKSURI,
		ScopesSupported:       d.ScopesSupported,
		ClaimsSupported:       d.ClaimsSupported,
		SupportsPkce:          &d.SupportsPKCE,
	}, nil
}
