package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	nanoid "github.com/matoous/go-nanoid/v2"

	"platrium/internal/authz"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/tenant"
	"platrium/internal/infra/db/ent/user"
)

// Tenant roles. A user's role says what they may administer in their tenant,
// separate from what they can do with any one file (see package authz).
//
// What each role may do is defined in package authz (see authz.Permission);
// check permissions, never role names.
const (
	RoleSuperAdmin = authz.TenantRoleSuperAdmin
	RoleAdmin      = authz.TenantRoleAdmin
	RoleMember     = authz.TenantRoleMember
)

// User represents an identity belonging to exactly one tenant.
type User struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	IdpID       string    `json:"idp_id"`
	ExternalID  string    `json:"external_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	// DisabledAt is when an administrator disabled the account; nil while active.
	DisabledAt *time.Time `json:"disabled_at,omitempty"`
}

// Disabled reports whether the account is blocked from signing in.
func (u *User) Disabled() bool { return u.DisabledAt != nil }

func userFromEnt(u *ent.User) *User {
	return &User{
		ID:          u.ID,
		TenantID:    u.TenantID,
		IdpID:       u.IdpID,
		ExternalID:  u.ExternalID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
		DisabledAt:  u.DisabledAt,
	}
}

// UserStore manages User records.
type UserStore struct {
	db       *db.DB
	nativeID atomic.Pointer[string] // the native tenant's ID, once seen
}

func NewUserStore(d *db.DB) *UserStore {
	return &UserStore{db: d}
}

// CreateUserParams are the inputs for creating a user.
type CreateUserParams struct {
	ID          string // optional; generated when empty
	TenantID    string
	IdpID       string
	ExternalID  string
	Email       string
	DisplayName string
	Role        string // optional; defaults to the schema default
}

// CreateUserTx creates a user within the provided transaction. The IdP must
// belong to the same tenant as the user.
func (r *UserStore) CreateUserTx(ctx context.Context, tx *ent.Tx, p CreateUserParams) (*User, error) {
	if p.ID == "" {
		p.ID = nanoid.Must()
	}

	// Tenant isolation: an IdP from another tenant must never back this user.
	// This also proves the tenant exists, so no separate tenant lookup is needed.
	if ok, err := tx.IdpProvider.Query().Where(idpprovider.ID(p.IdpID), idpprovider.TenantID(p.TenantID)).Exist(ctx); err != nil {
		return nil, fmt.Errorf("failed to look up idp: %w", err)
	} else if !ok {
		return nil, fmt.Errorf("%w: idp not found in tenant", ErrNotFound)
	}

	create := tx.User.Create().
		SetID(p.ID).
		SetTenantID(p.TenantID).
		SetIdpID(p.IdpID).
		SetExternalID(p.ExternalID).
		SetEmail(p.Email).
		SetDisplayName(p.DisplayName)
	if p.Role != "" {
		create.SetRole(p.Role)
	}

	u, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, fmt.Errorf("%w: a user with this externalId already exists for the idp: %v", ErrConflict, err)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return userFromEnt(u), nil
}

// SyncProfile updates the email address and display name of a user whose
// identity provider owns them. Nothing else about the user changes.
func (r *UserStore) SyncProfile(ctx context.Context, tenantID, userID, email, displayName string) error {
	n, err := r.db.User.Update().
		Where(user.ID(userID), user.TenantID(tenantID)).
		SetEmail(email).
		SetDisplayName(displayName).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: user", ErrNotFound)
	}
	return nil
}

// Account is the part of a user that decides whether and how they may act:
// what their role carries, whether they are blocked, and when their sessions
// were last revoked. It is one row read, whatever is asked of it.
type Account struct {
	Role string
	// Native says the user's organization is the installation's native tenant,
	// the only place cluster permissions apply.
	Native             bool
	DisabledAt         *time.Time
	SessionsValidAfter *time.Time
}

// Disabled reports whether the account is blocked from signing in.
func (a *Account) Disabled() bool { return a.DisabledAt != nil }

// Account loads a user's account. A user who is not in the tenant is ErrNotFound.
func (r *UserStore) Account(ctx context.Context, tenantID, userID string) (*Account, error) {
	return r.account(ctx, r.db.Client, tenantID, userID)
}

// AccountTx is Account within a transaction.
func (r *UserStore) AccountTx(ctx context.Context, tx *ent.Tx, tenantID, userID string) (*Account, error) {
	return r.account(ctx, tx.Client(), tenantID, userID)
}

func (r *UserStore) account(ctx context.Context, c *ent.Client, tenantID, userID string) (*Account, error) {
	u, err := c.User.Query().Where(user.ID(userID), user.TenantID(tenantID)).
		Select(user.FieldRole, user.FieldDisabledAt, user.FieldSessionsValidAfter).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	native, err := r.isNative(ctx, c, tenantID)
	if err != nil {
		return nil, err
	}
	return &Account{Role: u.Role, Native: native, DisabledAt: u.DisabledAt, SessionsValidAfter: u.SessionsValidAfter}, nil
}

// isNative reports whether the tenant is the native one. There is at most one
// and it is never replaced, so once found its ID is remembered and later
// checks cost nothing. Until it exists the answer is not remembered.
func (r *UserStore) isNative(ctx context.Context, c *ent.Client, tenantID string) (bool, error) {
	if id := r.nativeID.Load(); id != nil {
		return *id == tenantID, nil
	}
	id, err := c.Tenant.Query().Where(tenant.NativeSlotNotNil()).OnlyID(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch tenant: %w", err)
	}
	r.nativeID.Store(&id)
	return id == tenantID, nil
}

// Access returns what a user may do to their organization (and, in the native
// tenant, to the cluster), and whether that organization is the native tenant.
// A disabled or unknown user holds nothing.
func (r *UserStore) Access(ctx context.Context, tenantID, userID string) (authz.PermissionSet, bool, error) {
	return r.access(ctx, r.Account, tenantID, userID)
}

// AccessTx is Access within a transaction.
func (r *UserStore) AccessTx(ctx context.Context, tx *ent.Tx, tenantID, userID string) (authz.PermissionSet, bool, error) {
	return r.access(ctx, func(ctx context.Context, tenantID, userID string) (*Account, error) {
		return r.AccountTx(ctx, tx, tenantID, userID)
	}, tenantID, userID)
}

func (r *UserStore) access(ctx context.Context, load func(context.Context, string, string) (*Account, error), tenantID, userID string) (authz.PermissionSet, bool, error) {
	acc, err := load(ctx, tenantID, userID)
	if errors.Is(err, ErrNotFound) {
		native, nerr := r.isNative(ctx, r.db.Client, tenantID)
		return authz.NewPermissionSet(), native, nerr
	}
	if err != nil {
		return authz.PermissionSet{}, false, err
	}
	if acc.Disabled() {
		return authz.NewPermissionSet(), acc.Native, nil
	}
	return authz.EffectivePermissions(acc.Role, acc.Native), acc.Native, nil
}

// RevokeSessionsTx voids every session and token issued to the user so far;
// they must sign in again. It is the single sink for "sign out everywhere": a
// password reset calls it today, and an identity provider's logout notice or a
// revalidation failure would call it too. Re-enabling a disabled user does not
// call it, so their devices come back with them.
func (r *UserStore) RevokeSessionsTx(ctx context.Context, tx *ent.Tx, tenantID, userID string) error {
	n, err := tx.User.Update().Where(user.ID(userID), user.TenantID(tenantID)).
		SetSessionsValidAfter(time.Now().UTC()).Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke sessions: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: user", ErrNotFound)
	}
	return nil
}

// SetDisabled disables or re-enables a user of the tenant and returns them. It
// is idempotent: disabling an already disabled user keeps the original
// timestamp. Anything that authenticates the user checks the flag on every
// request, so this takes effect immediately.
func (r *UserStore) SetDisabled(ctx context.Context, tenantID, userID string, disabled bool) (*User, error) {
	if err := setDisabled(ctx, r.db.Client, tenantID, userID, disabled); err != nil {
		return nil, err
	}
	return r.Get(ctx, tenantID, userID)
}

// SetDisabledTx is SetDisabled within a transaction. It returns nothing, since
// its caller has the user in hand already and reads them again only if it must.
// A user who is not in the tenant is not an error here; check first.
func (r *UserStore) SetDisabledTx(ctx context.Context, tx *ent.Tx, tenantID, userID string, disabled bool) error {
	return setDisabled(ctx, tx.Client(), tenantID, userID, disabled)
}

func setDisabled(ctx context.Context, c *ent.Client, tenantID, userID string, disabled bool) error {
	q := c.User.Update().Where(user.ID(userID), user.TenantID(tenantID))
	if disabled {
		q = q.Where(user.DisabledAtIsNil()).SetDisabledAt(time.Now().UTC())
	} else {
		q = q.ClearDisabledAt()
	}
	if _, err := q.Save(ctx); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// Get fetches one user of the tenant. A user who is not in it is ErrNotFound.
func (r *UserStore) Get(ctx context.Context, tenantID, userID string) (*User, error) {
	u, err := r.db.User.Query().Where(user.ID(userID), user.TenantID(tenantID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user", ErrNotFound)
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}
	return userFromEnt(u), nil
}

// LockTenantTx takes a row lock on the tenant until the transaction ends.
// Changes that must not interleave, such as removing an administrator while
// another is being removed, take it first so they run one after the other.
// Run it in a ReadCommitted transaction (see db.WithTxOpts) so that what is
// read afterwards includes the previous holder's committed work. SQLite has no
// row locks but serializes writers, so there it only checks the tenant exists.
func (r *UserStore) LockTenantTx(ctx context.Context, tx *ent.Tx, tenantID string) error {
	q := tx.Tenant.Query().Where(tenant.ID(tenantID))
	if r.db.RowLocks() {
		q = q.ForUpdate()
	}
	if _, err := q.Only(ctx); err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: tenant", ErrNotFound)
		}
		return fmt.Errorf("failed to lock tenant: %w", err)
	}
	return nil
}

// GetUserByExternalId fetches a user and their tenant ID based on their IdP mapping.
func (r *UserStore) GetUserByExternalId(ctx context.Context, idpId, externalId string) (*User, string, error) {
	u, err := r.db.User.Query().
		Where(user.IdpID(idpId), user.ExternalID(externalId)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, "", fmt.Errorf("%w: user", ErrNotFound)
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}
	return userFromEnt(u), u.TenantID, nil
}

// GetByIDs returns the users with the given IDs that belong to the tenant, keyed
// by ID. Unknown IDs, and IDs from other tenants, are simply absent.
func (r *UserStore) GetByIDs(ctx context.Context, tenantID string, ids []string) (map[string]*User, error) {
	out := make(map[string]*User, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.User.Query().Where(user.IDIn(ids...), user.TenantID(tenantID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}
	for _, u := range rows {
		out[u.ID] = userFromEnt(u)
	}
	return out, nil
}

// Search finds users in a tenant by name or email, case-insensitively, ordered
// by name. Every member of a tenant may search its directory. It shares its
// match rule with List (see userPredicates) but skips loading each user's
// provider, which a typeahead does not show.
func (r *UserStore) Search(ctx context.Context, tenantID, query string, limit int) ([]*User, error) {
	query = strings.TrimSpace(query)
	if query == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.User.Query().
		Where(userPredicates(tenantID, UserFilter{Search: query})...).
		Order(user.ByDisplayName(), user.ByID()).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	out := make([]*User, 0, len(rows))
	for _, u := range rows {
		out = append(out, userFromEnt(u))
	}
	return out, nil
}
