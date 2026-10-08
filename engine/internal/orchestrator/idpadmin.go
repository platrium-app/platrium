package orchestrator

import (
	"context"

	"platrium/internal/auth"
	"platrium/internal/auth/actor"
	"platrium/internal/auth/protocol/oidc"
	"platrium/internal/authz"
	"platrium/internal/identity"
)

// IdpAdmin is how an organization's administrators manage the identity
// providers its users sign in through. Everything here is type-agnostic except
// the connection settings, which each protocol owns (OIDC today; SAML in the
// enterprise edition adds its own Create/Update beside CreateOIDC/UpdateOIDC).
//
// Like UserAdmin, every method re-checks the permission it needs. The rules
// about what may happen to a provider (the built-in one is read-only and
// permanent, a provider with users is not deleted, the policy must be safe)
// live in the stores, so they hold for every caller; this layer does not
// repeat them.
type IdpAdmin struct {
	users     *identity.UserStore
	idps      *auth.IdpStore
	oidc      *oidc.Store
	oidcFlow  *oidc.Client
	endpoints oidc.Endpoints
}

func NewIdpAdmin(users *identity.UserStore, idps *auth.IdpStore, oidcStore *oidc.Store, oidcFlow *oidc.Client, endpoints oidc.Endpoints) *IdpAdmin {
	return &IdpAdmin{users: users, idps: idps, oidc: oidcStore, oidcFlow: oidcFlow, endpoints: endpoints}
}

// ManagedIdp is a provider as an administrator sees it.
type ManagedIdp struct {
	*auth.IdpProvider
	UserCount int
	// OIDC is the connection settings of an OIDC provider, without the secret
	// (the field is empty). Nil for other types.
	OIDC *oidc.Config
	// OIDCRedirectURI is the address to register with an OIDC provider.
	OIDCRedirectURI string
}

func (a *IdpAdmin) require(ctx context.Context, p authz.Principal) (*actor.Identity, error) {
	return requirePermission(ctx, p, authz.PermIdpManage, a.users.Access)
}

// view loads what an administrator sees of one provider.
func (a *IdpAdmin) view(ctx context.Context, idp *auth.IdpProvider, userCount int) (*ManagedIdp, error) {
	m := &ManagedIdp{IdpProvider: idp, UserCount: userCount}
	if idp.Type == auth.IdpTypeOIDC {
		cfg, err := a.oidc.Get(ctx, idp.ID)
		if err != nil {
			return nil, err
		}
		cfg.ClientSecret = "" // never leaves the store layer
		m.OIDC = cfg
		m.OIDCRedirectURI = a.endpoints.Callback(idp.ID)
	}
	return m, nil
}

// List returns the organization's providers, the built-in one first.
func (a *IdpAdmin) List(ctx context.Context, p authz.Principal) ([]*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	rows, err := a.idps.ListWithCounts(ctx, ac.TenantID)
	if err != nil {
		return nil, err
	}
	out := make([]*ManagedIdp, 0, len(rows))
	for _, r := range rows {
		m, err := a.view(ctx, r.IdpProvider, r.UserCount)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Get returns one provider of the organization.
func (a *IdpAdmin) Get(ctx context.Context, p authz.Principal, id string) (*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	return a.get(ctx, ac.TenantID, id)
}

func (a *IdpAdmin) get(ctx context.Context, tenantID, id string) (*ManagedIdp, error) {
	idp, err := a.idps.GetInTenant(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	n, err := a.idps.CountUsers(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	return a.view(ctx, idp, n)
}

// CreateOIDC adds an OpenID Connect provider to the organization.
func (a *IdpAdmin) CreateOIDC(ctx context.Context, p authz.Principal, name string, policy auth.ProvisioningPolicy, cfg oidc.Config) (*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	idp, _, err := a.oidc.Create(ctx, oidc.CreateParams{TenantID: ac.TenantID, Name: name, ProvisioningPolicy: policy, Config: cfg})
	if err != nil {
		return nil, err
	}
	return a.get(ctx, ac.TenantID, idp.ID)
}

// Update changes the settings every provider type shares.
func (a *IdpAdmin) Update(ctx context.Context, p authz.Principal, id string, in auth.UpdateIdpParams) (*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	if _, err := a.idps.Update(ctx, ac.TenantID, id, in); err != nil {
		return nil, err
	}
	return a.get(ctx, ac.TenantID, id)
}

// UpdateOIDC changes the connection settings of an OIDC provider.
func (a *IdpAdmin) UpdateOIDC(ctx context.Context, p authz.Principal, id string, in oidc.UpdateParams) (*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	if _, err := a.oidc.Update(ctx, ac.TenantID, id, in); err != nil {
		return nil, err
	}
	return a.get(ctx, ac.TenantID, id)
}

// SetEnabled enables or disables a provider.
func (a *IdpAdmin) SetEnabled(ctx context.Context, p authz.Principal, id string, enabled bool) (*ManagedIdp, error) {
	ac, err := a.require(ctx, p)
	if err != nil {
		return nil, err
	}
	if _, err := a.idps.SetEnabled(ctx, ac.TenantID, id, enabled); err != nil {
		return nil, err
	}
	return a.get(ctx, ac.TenantID, id)
}

// Delete removes a provider that has no users.
func (a *IdpAdmin) Delete(ctx context.Context, p authz.Principal, id string) error {
	ac, err := a.require(ctx, p)
	if err != nil {
		return err
	}
	return a.idps.Delete(ctx, ac.TenantID, id)
}

// CheckOIDC reads an issuer's discovery document, so the form can say whether
// the URL is a working OpenID provider before anything is saved. A URL that is
// not one is oidc.ErrDiscovery.
func (a *IdpAdmin) CheckOIDC(ctx context.Context, p authz.Principal, issuer string) (*oidc.Discovery, error) {
	if _, err := a.require(ctx, p); err != nil {
		return nil, err
	}
	return a.oidcFlow.Discover(ctx, issuer)
}
