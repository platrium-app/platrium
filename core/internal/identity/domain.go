package identity

import (
	"context"
	"fmt"
	"platrium/internal/infra/graph"
)

// Domain represents a verified or unverified email domain belonging to a Tenant.
type Domain struct {
	Name       string `json:"name"`       // e.g., "acme.com"
	IsVerified bool   `json:"isVerified"` // Soft block flag
}

// DomainStore manages Domain nodes in the GraphDB.
type DomainStore struct {
	store graph.Graph
}

func NewDomainStore(store graph.Graph) *DomainStore {
	return &DomainStore{store: store}
}

// AddDomainToTenant links a new Domain node to the Tenant.
// It ensures that the domain name is globally unique across all tenants.
func (r *DomainStore) AddDomainToTenant(ctx context.Context, tenantID, domainName string) error {
	query := `
		// 1. Check if domain already exists anywhere
		OPTIONAL MATCH (existingDomain:Domain {name: $domainName})
		WITH existingDomain
		WHERE existingDomain IS NULL

		// 2. Create the Domain and link it to the Tenant
		MATCH (t:Tenant {id: $tenantID})
		CREATE (d:Domain {name: $domainName, isVerified: false})
		CREATE (t)-[:OWNS_DOMAIN]->(d)
		RETURN d.name
	`
	return r.store.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, map[string]interface{}{
			"tenantID":   tenantID,
			"domainName": domainName,
		})
		if err != nil {
			return err
		}
		defer res.Close()

		if !res.Next() {
			return fmt.Errorf("domain %s is already in use by another tenant", domainName)
		}

		return nil
	})
}

// RemoveDomainFromTenant unlinks and deletes a domain if the organization stops using it.
// It enforces the rule that a Tenant must always have at least one Domain.
func (r *DomainStore) RemoveDomainFromTenant(ctx context.Context, tenantID, domainName string) error {
	query := `
		// 1. Count how many domains the tenant has
		MATCH (t:Tenant {id: $tenantID})-[:OWNS_DOMAIN]->(d:Domain)
		WITH t, count(d) AS domainCount
		
		// 2. Abort if this is their only domain (must have >1 to safely delete one)
		WHERE domainCount > 1
		
		// 3. Find the specific domain and delete it
		MATCH (t)-[:OWNS_DOMAIN]->(target:Domain {name: $domainName})
		DETACH DELETE target
		
		// 4. Return something so the Go driver knows the WHERE clause passed
		RETURN target.name
	`
	return r.store.WriteTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, map[string]interface{}{
			"tenantID":   tenantID,
			"domainName": domainName,
		})
		if err != nil {
			return err
		}
		defer res.Close()

		// If the query returns 0 rows, it means the `WHERE domainCount > 1` clause blocked it!
		if !res.Next() {
			return fmt.Errorf("cannot delete domain '%s': a tenant must have at least one domain", domainName)
		}

		return nil
	})
}
