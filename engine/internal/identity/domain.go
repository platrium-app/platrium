package identity

import (
	"context"
	"fmt"
	"strings"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/domain"
	"platrium/internal/infra/db/ent/tenant"
)

// Domain represents a verified or unverified email domain belonging to a Tenant.
type Domain struct {
	Name       string `json:"name"`       // e.g., "acme.com"
	IsVerified bool   `json:"isVerified"` // Soft block flag
}

// DomainStore manages Domain records.
type DomainStore struct {
	db *db.DB
}

func NewDomainStore(d *db.DB) *DomainStore {
	return &DomainStore{db: d}
}

// normalizeDomain lowercases a domain name; names are stored lowercase.
func normalizeDomain(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// AddDomainToTenant claims a new Domain for the Tenant.
// It ensures that the domain name is globally unique across all tenants.
func (r *DomainStore) AddDomainToTenant(ctx context.Context, tenantID, domainName string) error {
	name := normalizeDomain(domainName)
	if name == "" {
		return fmt.Errorf("domain name cannot be empty")
	}

	_, err := r.db.Domain.Create().SetTenantID(tenantID).SetName(name).Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return fmt.Errorf("%w: domain %s is already in use or tenant not found: %v", ErrConflict, name, err)
		}
		return fmt.Errorf("failed to add domain: %w", err)
	}
	return nil
}

// RemoveDomainFromTenant deletes a domain if the organization stops using it.
// It enforces the rule that a Tenant must always have at least one Domain.
func (r *DomainStore) RemoveDomainFromTenant(ctx context.Context, tenantID, domainName string) error {
	name := normalizeDomain(domainName)

	return r.db.WithTx(ctx, func(tx *ent.Tx) error {
		// Serialize concurrent removals for this tenant so the "at least one
		// domain" rule cannot be raced past.
		lock := tx.Tenant.Query().Where(tenant.ID(tenantID))
		if r.db.RowLocks() {
			lock = lock.ForUpdate()
		}
		if _, err := lock.Only(ctx); err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("tenant not found")
			}
			return err
		}

		count, err := tx.Domain.Query().Where(domain.TenantID(tenantID)).Count(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return fmt.Errorf("cannot delete domain '%s': a tenant must have at least one domain", name)
		}

		n, err := tx.Domain.Delete().Where(domain.TenantID(tenantID), domain.Name(name)).Exec(ctx)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("domain '%s' not found for tenant", name)
		}
		return nil
	})
}
