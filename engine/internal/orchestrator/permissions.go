package orchestrator

import (
	"fmt"

	"platrium/internal/authz"
)

// requireSignedIn refuses an anonymous principal.
func requireSignedIn(p authz.Principal) error {
	if p.IsAnonymous() || p.TenantID == "" {
		return fmt.Errorf("%w: sign in required", authz.ErrForbidden)
	}
	return nil
}

// needPermission is the one place a missing permission becomes an error.
//
// Orchestrators are the authoritative gate: they read the caller's permissions
// themselves instead of trusting what a transport resolved, because they are
// also reached by code with no request to remember anything (a SCIM endpoint, a
// CLI), and because a change made earlier in the same request must count.
func needPermission(perms authz.PermissionSet, need authz.Permission) error {
	if !perms.Has(need) {
		return fmt.Errorf("%w: you do not have the %s permission", authz.ErrForbidden, need)
	}
	return nil
}
