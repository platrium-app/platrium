package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"platrium/internal/apperr"
	"platrium/internal/authz"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/tenant"
	"platrium/internal/infra/db/ent/user"
)

// ProvisioningPolicy says how users arriving through an IdP are created. It is protocol-agnostic: OIDC and SAML both fill an IdpAuthHandoff
// and the same provisioner applies the policy to it.
type ProvisioningPolicy struct {
	// JITUsers creates the user on first sign-in. When false, only users that
	// already exist can sign in.
	JITUsers bool `json:"jit_users"`
	// DefaultRole is the role a JIT user starts with. It is never an
	// administrative one, see ValidateJITRole.
	DefaultRole string `json:"default_role"`
	// AllowedEmailDomains, when non-empty, limits sign-in to these (lowercase)
	// email domains.
	AllowedEmailDomains []string `json:"allowed_email_domains"`
}

// ErrInvalidPolicy means a provisioning policy cannot be saved.
var ErrInvalidPolicy = apperr.ErrInvalid

// ValidateJITRole reports whether role may be the default role of JIT users.
// Only a role that carries no permission qualifies: an external provider must
// never be able to mint administrators just by sending someone through it.
// Administrator access through an IdP will be an explicit, audited mapping.
func ValidateJITRole(role string) error {
	if !authz.KnownTenantRole(role) {
		return fmt.Errorf("%w: unknown role %q", ErrInvalidPolicy, role)
	}
	if authz.EffectivePermissions(role, true).Len() > 0 {
		return fmt.Errorf("%w: role %q is administrative and cannot be a default role", ErrInvalidPolicy, role)
	}
	return nil
}

// Normalize validates the policy and canonicalizes its domains (lowercase,
// no duplicates).
func (p ProvisioningPolicy) Normalize() (ProvisioningPolicy, error) {
	if err := ValidateJITRole(p.DefaultRole); err != nil {
		return p, err
	}
	seen := map[string]bool{}
	domains := make([]string, 0, len(p.AllowedEmailDomains))
	for _, d := range p.AllowedEmailDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || strings.ContainsAny(d, "@ /") || !strings.Contains(d, ".") {
			return p, fmt.Errorf("%w: %q is not an email domain", ErrInvalidPolicy, d)
		}
		if !seen[d] {
			seen[d] = true
			domains = append(domains, d)
		}
	}
	p.AllowedEmailDomains = domains
	return p, nil
}

// AllowsEmail reports whether the policy lets this email address sign in.
func (p ProvisioningPolicy) AllowsEmail(email string) bool {
	if len(p.AllowedEmailDomains) == 0 {
		return true
	}
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	return slices.Contains(p.AllowedEmailDomains, strings.ToLower(email[at+1:]))
}

// IdpProvider is the protocol-agnostic definition of an Identity Provider.
// Protocol settings (an OIDC issuer and client, ...) live beside it, keyed by
// ID, in the protocol's own package.
type IdpProvider struct {
	ID       string  `json:"id"` // NanoID (e.g., "idp_google_1")
	TenantID string  `json:"tenant_id"`
	Type     IdpType `json:"type"`
	Name     string  `json:"name"` // Display name for the Login Picker (e.g., "Acme Azure AD")
	Enabled  bool    `json:"enabled"`

	ProvisioningPolicy
}

// IdpAuthHandoff represents the normalized claims parsed by any authentication flow (OIDC, SAML, Local).
// It contains exactly what the Platrium domain layer needs to issue a session.
type IdpAuthHandoff struct {
	IdpProviderID string `json:"idp_connection_id"` // The NanoID of the IdP connection (e.g., "idp_okta_1")
	SubjectID     string `json:"subject_id"`        // The unique user ID from the IdP (e.g., OIDC "sub" or SAML "NameID")
	Email         string
	// EmailVerified is whether the provider vouches for Email.
	EmailVerified bool
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
}

// Sentinel errors for a sign-in that is refused. They apply to every protocol:
// the decision is made by Admit and the Manager, never inside a protocol.
var (
	ErrProviderDisabled = errors.New("identity provider is disabled")
	ErrUserDisabled     = errors.New("account is disabled")
	ErrEmailNotAllowed  = errors.New("email address is not allowed for this identity provider")
	// ErrNotProvisioned means the identity has no account and the provider does
	// not create accounts on first sign-in.
	ErrNotProvisioned = errors.New("no account exists for this identity")
)

// Admit is the one gate every sign-in passes, whatever the protocol (local,
// OIDC, SAML, ...). email is the address being signed in with; user is nil for
// a first sign-in.
func (i *IdpProvider) Admit(email string, user *identity.User) error {
	if !i.Enabled {
		return ErrProviderDisabled
	}
	if !i.AllowsEmail(email) {
		return ErrEmailNotAllowed
	}
	if user != nil && user.Disabled() {
		return ErrUserDisabled
	}
	return nil
}

// IsLocal reports whether this is the built-in provider.
func (i *IdpProvider) IsLocal() bool { return i.Type == IdpTypeLocal }

func idpFromEnt(i *ent.IdpProvider) *IdpProvider {
	return &IdpProvider{
		ID:       i.ID,
		TenantID: i.TenantID,
		Type:     IdpType(i.Type),
		Name:     i.Name,
		Enabled:  i.Enabled,
		ProvisioningPolicy: ProvisioningPolicy{
			JITUsers:            i.JitUsers,
			DefaultRole:         i.DefaultRole,
			AllowedEmailDomains: i.AllowedEmailDomains,
		},
	}
}

// IdpStore manages IdpProvider records.
type IdpStore struct {
	db *db.DB
}

func NewIdpStore(d *db.DB) *IdpStore {
	return &IdpStore{db: d}
}

// CreateLocalIdpTx creates the built-in LOCAL identity provider for a tenant
// within the provided transaction.
func (r *IdpStore) CreateLocalIdpTx(ctx context.Context, tx *ent.Tx, tenantID, idpID string) (*IdpProvider, error) {
	create := tx.IdpProvider.Create().
		SetTenantID(tenantID).
		SetType(idpprovider.TypeLOCAL).
		SetName("Platrium Authentication")
	if idpID != "" {
		create.SetID(idpID)
	}

	i, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create local idp: %w", err)
	}
	return idpFromEnt(i), nil
}

// CreateIdpParams are the inputs for creating an external identity provider.
type CreateIdpParams struct {
	ID       string // optional; generated when empty
	TenantID string
	Type     IdpType
	Name     string
	ProvisioningPolicy
}

// CreateTx creates an external (OIDC or SAML) provider within the provided
// transaction; the caller adds the protocol's config row in the same
// transaction. The built-in provider is made with CreateLocalIdpTx.
func (r *IdpStore) CreateTx(ctx context.Context, tx *ent.Tx, p CreateIdpParams) (*IdpProvider, error) {
	if p.Type == IdpTypeLocal {
		return nil, errBuiltIn("created")
	}
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", apperr.ErrInvalid)
	}
	policy, err := p.ProvisioningPolicy.Normalize()
	if err != nil {
		return nil, err
	}
	create := tx.IdpProvider.Create().
		SetTenantID(p.TenantID).
		SetType(idpprovider.Type(p.Type)).
		SetName(strings.TrimSpace(p.Name)).
		SetJitUsers(policy.JITUsers).
		SetDefaultRole(policy.DefaultRole).
		SetAllowedEmailDomains(policy.AllowedEmailDomains)
	if p.ID != "" {
		create.SetID(p.ID)
	}
	i, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create idp: %w", err)
	}
	return idpFromEnt(i), nil
}

// GetIdpsForTenant looks up all configured IdPs for a specific tenant alias.
func (r *IdpStore) GetIdpsForTenant(ctx context.Context, alias string) ([]*IdpProvider, error) {
	rows, err := r.db.IdpProvider.Query().
		Where(idpprovider.HasTenantWith(tenant.AliasEQ(identity.NormalizeAlias(alias)))).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch idps by alias: %w", err)
	}

	idps := make([]*IdpProvider, 0, len(rows))
	for _, i := range rows {
		idps = append(idps, idpFromEnt(i))
	}
	return idps, nil
}

// ListForTenant returns every IdP of a tenant, the built-in one first.
func (r *IdpStore) ListForTenant(ctx context.Context, tenantID string) ([]*IdpProvider, error) {
	rows, err := r.db.IdpProvider.Query().
		Where(idpprovider.TenantID(tenantID)).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list idps: %w", err)
	}
	idps := make([]*IdpProvider, 0, len(rows))
	for _, i := range rows {
		idps = append(idps, idpFromEnt(i))
	}
	return idps, nil
}

// LocalForTenant returns the tenant's built-in (LOCAL) identity provider.
func (r *IdpStore) LocalForTenant(ctx context.Context, tenantID string) (*IdpProvider, error) {
	i, err := r.db.IdpProvider.Query().
		Where(idpprovider.TenantID(tenantID), idpprovider.TypeEQ(idpprovider.TypeLOCAL)).
		Order(idpprovider.ByCreatedAt(), idpprovider.ByID()).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: local identity provider", identity.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch local idp: %w", err)
	}
	return idpFromEnt(i), nil
}

// ErrBuiltIn means the operation is not allowed on the built-in (LOCAL)
// identity provider. It is created once, with its tenant, and stays exactly as
// it is: it can not be created again, edited, disabled or deleted. These are
// the only places that rule lives; everything above the store just reports it.
func errBuiltIn(action string) error {
	return fmt.Errorf("%w: the built-in identity provider cannot be %s", apperr.ErrInvalid, action)
}

// editable loads a provider of the tenant that may be changed or removed.
func (r *IdpStore) editable(ctx context.Context, c *ent.Client, tenantID, id, action string) (*ent.IdpProvider, error) {
	row, err := c.IdpProvider.Query().Where(idpprovider.ID(id), idpprovider.TenantID(tenantID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: identity provider", apperr.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch idp: %w", err)
	}
	if IdpType(row.Type) == IdpTypeLocal {
		return nil, errBuiltIn(action)
	}
	return row, nil
}

// GetInTenant returns a provider of the tenant; one of another tenant is
// ErrNotFound.
func (r *IdpStore) GetInTenant(ctx context.Context, tenantID, id string) (*IdpProvider, error) {
	row, err := r.db.IdpProvider.Query().Where(idpprovider.ID(id), idpprovider.TenantID(tenantID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: identity provider", apperr.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch idp: %w", err)
	}
	return idpFromEnt(row), nil
}

// IdpSummary is a provider with the number of users that sign in through it.
type IdpSummary struct {
	*IdpProvider
	UserCount int
}

// ListWithCounts returns the tenant's providers, the built-in one first.
func (r *IdpStore) ListWithCounts(ctx context.Context, tenantID string) ([]*IdpSummary, error) {
	idps, err := r.ListForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var counts []struct {
		IdpID string `json:"idp_id"`
		Count int    `json:"count"`
	}
	if err := r.db.User.Query().Where(user.TenantID(tenantID)).
		GroupBy(user.FieldIdpID).Aggregate(ent.Count()).Scan(ctx, &counts); err != nil {
		return nil, fmt.Errorf("failed to count idp users: %w", err)
	}
	byIdp := make(map[string]int, len(counts))
	for _, c := range counts {
		byIdp[c.IdpID] = c.Count
	}
	out := make([]*IdpSummary, 0, len(idps))
	for _, i := range idps {
		out = append(out, &IdpSummary{IdpProvider: i, UserCount: byIdp[i.ID]})
	}
	return out, nil
}

// CountUsers is how many users sign in through the provider.
func (r *IdpStore) CountUsers(ctx context.Context, tenantID, id string) (int, error) {
	return r.db.User.Query().Where(user.TenantID(tenantID), user.IdpID(id)).Count(ctx)
}

// UpdateIdpParams are the changes to the common settings of a provider; nil
// leaves a setting as it is.
type UpdateIdpParams struct {
	Name                *string
	JITUsers            *bool
	DefaultRole         *string
	AllowedEmailDomains *[]string
}

// Update changes the common settings of an external provider. The resulting
// policy is validated as a whole.
func (r *IdpStore) Update(ctx context.Context, tenantID, id string, p UpdateIdpParams) (*IdpProvider, error) {
	var out *IdpProvider
	err := r.db.WithTx(ctx, func(tx *ent.Tx) error {
		row, err := r.editable(ctx, tx.Client(), tenantID, id, "edited")
		if err != nil {
			return err
		}
		upd := tx.IdpProvider.UpdateOne(row)
		if p.Name != nil {
			name := strings.TrimSpace(*p.Name)
			if name == "" {
				return fmt.Errorf("%w: name is required", apperr.ErrInvalid)
			}
			upd.SetName(name)
		}

		policy := idpFromEnt(row).ProvisioningPolicy
		if p.JITUsers != nil {
			policy.JITUsers = *p.JITUsers
		}
		if p.DefaultRole != nil {
			policy.DefaultRole = *p.DefaultRole
		}
		if p.AllowedEmailDomains != nil {
			policy.AllowedEmailDomains = *p.AllowedEmailDomains
		}
		if policy, err = policy.Normalize(); err != nil {
			return err
		}
		upd.SetJitUsers(policy.JITUsers).SetDefaultRole(policy.DefaultRole).SetAllowedEmailDomains(policy.AllowedEmailDomains)

		saved, err := upd.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update idp: %w", err)
		}
		out = idpFromEnt(saved)
		return nil
	})
	return out, err
}

// SetEnabled enables or disables an external provider. A disabled provider
// refuses sign-ins; its users and their data stay.
func (r *IdpStore) SetEnabled(ctx context.Context, tenantID, id string, enabled bool) (*IdpProvider, error) {
	var out *IdpProvider
	err := r.db.WithTx(ctx, func(tx *ent.Tx) error {
		row, err := r.editable(ctx, tx.Client(), tenantID, id, "disabled")
		if err != nil {
			return err
		}
		saved, err := tx.IdpProvider.UpdateOne(row).SetEnabled(enabled).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update idp: %w", err)
		}
		out = idpFromEnt(saved)
		return nil
	})
	return out, err
}

// Delete removes an external provider and its protocol config. A provider with
// users is kept (ErrConflict): their accounts and files would be orphaned, so
// the provider is disabled instead.
func (r *IdpStore) Delete(ctx context.Context, tenantID, id string) error {
	return r.db.WithTx(ctx, func(tx *ent.Tx) error {
		if _, err := r.editable(ctx, tx.Client(), tenantID, id, "deleted"); err != nil {
			return err
		}
		n, err := tx.User.Query().Where(user.TenantID(tenantID), user.IdpID(id)).Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count idp users: %w", err)
		}
		if n > 0 {
			return fmt.Errorf("%w: %d users sign in through this provider; disable it instead", apperr.ErrConflict, n)
		}
		if err := tx.IdpProvider.DeleteOneID(id).Exec(ctx); err != nil {
			if ent.IsConstraintError(err) {
				return fmt.Errorf("%w: this provider is still referenced (for example by groups)", apperr.ErrConflict)
			}
			return fmt.Errorf("failed to delete idp: %w", err)
		}
		return nil
	})
}

// GetIdpById fetches a specific IdP connection by its unique NanoID.
func (r *IdpStore) GetIdpById(ctx context.Context, id string) (*IdpProvider, error) {
	i, err := r.db.IdpProvider.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("idp not found")
		}
		return nil, fmt.Errorf("failed to fetch idp by id: %w", err)
	}
	return idpFromEnt(i), nil
}
