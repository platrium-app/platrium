package oidc

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/apperr"
	"platrium/internal/auth"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpoidcconfig"
	"platrium/internal/secrets"
)

// Standard claim names, used when a provider does not say otherwise.
const (
	DefaultEmailClaim   = "email"
	DefaultNameClaim    = "name"
	DefaultPictureClaim = "picture"
)

// Config is the OpenID Connect side of an OIDC identity provider. The
// provider's endpoints are found by discovery from Issuer.
type Config struct {
	IdpID    string
	TenantID string

	Issuer string
	// ClientID and ClientSecret identify Platrium to the provider. The secret
	// is only ever in memory here; it is sealed in the database.
	ClientID     string
	ClientSecret string
	// Scopes are requested in addition to "openid".
	Scopes []string

	EmailClaim           string
	NameClaim            string
	PictureClaim         string
	RequireEmailVerified bool
	ExtraAuthParams      map[string]string
}

// normalize validates the config and fills the optional claim names with the
// OIDC standard ones.
func (c *Config) normalize() error {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%w: %s", apperr.ErrInvalid, fmt.Sprintf(format, a...))
	}

	issuer, err := ValidateIssuer(c.Issuer)
	if err != nil {
		return err
	}
	c.Issuer = issuer

	c.ClientID = strings.TrimSpace(c.ClientID)
	if c.ClientID == "" {
		return bad("client ID is required")
	}

	scopes := make([]string, 0, len(c.Scopes))
	for _, s := range c.Scopes {
		s = strings.TrimSpace(s)
		if s == "" || strings.ContainsAny(s, " \t\n") {
			return bad("scope %q is not valid", s)
		}
		if s != "openid" && !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}
	c.Scopes = scopes

	for k, def := range map[*string]string{&c.EmailClaim: DefaultEmailClaim, &c.NameClaim: DefaultNameClaim, &c.PictureClaim: DefaultPictureClaim} {
		if *k = strings.TrimSpace(*k); *k == "" {
			*k = def
		}
	}
	return nil
}

// ValidateIssuer checks an issuer URL and returns it trimmed. Providers must
// be reached over https; plain http is for a provider on the same machine
// (development) only.
func ValidateIssuer(raw string) (string, error) {
	issuer := strings.TrimSpace(raw)
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("%w: issuer must be a URL such as https://login.example.com/", apperr.ErrInvalid)
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopbackHost(u.Hostname())) {
		return "", fmt.Errorf("%w: issuer must use https", apperr.ErrInvalid)
	}
	return issuer, nil
}

func isLoopbackHost(h string) bool { return h == "localhost" || h == "127.0.0.1" || h == "::1" }

// CreateParams are the inputs for creating an OIDC identity provider.
type CreateParams struct {
	ID       string // optional; generated when empty
	TenantID string
	Name     string
	auth.ProvisioningPolicy
	Config
}

// Store manages OIDC identity providers: the provider row and its config.
type Store struct {
	db   *db.DB
	idps *auth.IdpStore
	keys *secrets.Keyring
}

func NewStore(d *db.DB, idps *auth.IdpStore, keys *secrets.Keyring) *Store {
	return &Store{db: d, idps: idps, keys: keys}
}

// Create makes an OIDC provider and its config in one transaction. The client
// secret is sealed before it is written.
func (s *Store) Create(ctx context.Context, p CreateParams) (*auth.IdpProvider, *Config, error) {
	cfg := p.Config
	if err := cfg.normalize(); err != nil {
		return nil, nil, err
	}
	if cfg.ClientSecret == "" {
		return nil, nil, fmt.Errorf("%w: client secret is required", apperr.ErrInvalid)
	}
	if p.ID == "" {
		p.ID = nanoid.Must() // the secret is bound to the ID, so it is minted first
	}
	sealed, err := s.keys.Seal(cfg.ClientSecret, p.ID)
	if err != nil {
		return nil, nil, err
	}

	var idp *auth.IdpProvider
	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		idp, err = s.idps.CreateTx(ctx, tx, auth.CreateIdpParams{
			ID: p.ID, TenantID: p.TenantID, Type: auth.IdpTypeOIDC,
			Name: p.Name, ProvisioningPolicy: p.ProvisioningPolicy,
		})
		if err != nil {
			return err
		}
		_, err = tx.IdpOIDCConfig.Create().
			SetTenantID(p.TenantID).
			SetIdpID(idp.ID).
			SetIssuer(cfg.Issuer).
			SetClientID(cfg.ClientID).
			SetClientSecret(sealed).
			SetScopes(cfg.Scopes).
			SetEmailClaim(cfg.EmailClaim).
			SetNameClaim(cfg.NameClaim).
			SetPictureClaim(cfg.PictureClaim).
			SetRequireEmailVerified(cfg.RequireEmailVerified).
			SetExtraAuthParams(cfg.ExtraAuthParams).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create oidc config: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	cfg.IdpID, cfg.TenantID = idp.ID, idp.TenantID
	return idp, &cfg, nil
}

// UpdateParams are the changes to an OIDC provider's connection settings; nil
// leaves a setting as it is. The issuer is deliberately absent: users are known
// by the (provider, subject) pair, and a subject only means something to the
// issuer that minted it. Pointing a provider at a different issuer could hand
// one of its users' accounts to someone else, so that is a new provider.
type UpdateParams struct {
	ClientID             *string
	ClientSecret         *string // nil keeps the current secret
	Scopes               *[]string
	EmailClaim           *string
	NameClaim            *string
	PictureClaim         *string
	RequireEmailVerified *bool
	ExtraAuthParams      *map[string]string
}

// Update changes the connection settings of an OIDC provider of the tenant.
func (s *Store) Update(ctx context.Context, tenantID, idpID string, p UpdateParams) (*Config, error) {
	idp, err := s.idps.GetInTenant(ctx, tenantID, idpID)
	if err != nil {
		return nil, err
	}
	if idp.Type != auth.IdpTypeOIDC {
		return nil, fmt.Errorf("%w: not an OIDC identity provider", apperr.ErrInvalid)
	}

	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		row, err := tx.IdpOIDCConfig.Query().Where(idpoidcconfig.IdpID(idpID), idpoidcconfig.TenantID(tenantID)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("%w: oidc config", apperr.ErrNotFound)
			}
			return fmt.Errorf("failed to fetch oidc config: %w", err)
		}

		c := Config{
			Issuer: row.Issuer, ClientID: row.ClientID, Scopes: row.Scopes,
			EmailClaim: row.EmailClaim, NameClaim: row.NameClaim, PictureClaim: row.PictureClaim,
			RequireEmailVerified: row.RequireEmailVerified, ExtraAuthParams: row.ExtraAuthParams,
		}
		if p.ClientID != nil {
			c.ClientID = *p.ClientID
		}
		if p.Scopes != nil {
			c.Scopes = *p.Scopes
		}
		if p.EmailClaim != nil {
			c.EmailClaim = *p.EmailClaim
		}
		if p.NameClaim != nil {
			c.NameClaim = *p.NameClaim
		}
		if p.PictureClaim != nil {
			c.PictureClaim = *p.PictureClaim
		}
		if p.RequireEmailVerified != nil {
			c.RequireEmailVerified = *p.RequireEmailVerified
		}
		if p.ExtraAuthParams != nil {
			c.ExtraAuthParams = *p.ExtraAuthParams
		}
		if err := c.normalize(); err != nil {
			return err
		}

		upd := tx.IdpOIDCConfig.UpdateOne(row).
			SetClientID(c.ClientID).SetScopes(c.Scopes).
			SetEmailClaim(c.EmailClaim).SetNameClaim(c.NameClaim).SetPictureClaim(c.PictureClaim).
			SetRequireEmailVerified(c.RequireEmailVerified).SetExtraAuthParams(c.ExtraAuthParams)
		if p.ClientSecret != nil {
			if *p.ClientSecret == "" {
				return fmt.Errorf("%w: client secret cannot be empty", apperr.ErrInvalid)
			}
			sealed, err := s.keys.Seal(*p.ClientSecret, idpID)
			if err != nil {
				return err
			}
			upd.SetClientSecret(sealed)
		}
		if _, err := upd.Save(ctx); err != nil {
			return fmt.Errorf("failed to update oidc config: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, idpID)
}

// Get loads the config of an OIDC provider with its secret opened.
func (s *Store) Get(ctx context.Context, idpID string) (*Config, error) {
	row, err := s.db.IdpOIDCConfig.Query().Where(idpoidcconfig.IdpID(idpID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: oidc config", apperr.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch oidc config: %w", err)
	}
	secret, err := s.keys.Open(row.ClientSecret, row.IdpID)
	if err != nil {
		return nil, fmt.Errorf("oidc config %s: %w", row.IdpID, err)
	}
	return &Config{
		IdpID: row.IdpID, TenantID: row.TenantID,
		Issuer: row.Issuer, ClientID: row.ClientID, ClientSecret: secret,
		Scopes:     row.Scopes,
		EmailClaim: row.EmailClaim, NameClaim: row.NameClaim, PictureClaim: row.PictureClaim,
		RequireEmailVerified: row.RequireEmailVerified,
		ExtraAuthParams:      row.ExtraAuthParams,
	}, nil
}
