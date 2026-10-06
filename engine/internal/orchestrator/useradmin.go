package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores (or rejects) anything longer
	maxNameLen     = 255
)

// UserAdmin is how an organization's administrators manage its people. It
// composes identity (who the users are), the built-in provider (passwords) and
// the permission model (who may do what), which is why it is not a method on
// any one of them.
//
// Every method re-checks the permission it needs: the GraphQL @requires
// directive is the declared gate, this is the authoritative one. Beyond that,
// three rules apply however the caller got in:
//   - you manage only users whose permissions you hold, so an administrator
//     cannot disable, edit or take over someone more powerful;
//   - you give out no more than you hold (authz.CanAssignRole);
//   - the last administrator who can assign roles is never removed.
type UserAdmin struct {
	db    *db.DB
	users *identity.UserStore
	idps  *auth.IdpStore
	local *local.LocalUserStore
}

func NewUserAdmin(d *db.DB, us *identity.UserStore, is *auth.IdpStore, lus *local.LocalUserStore) *UserAdmin {
	return &UserAdmin{db: d, users: us, idps: is, local: lus}
}

// ManagedUser is a user as one particular administrator sees them.
type ManagedUser struct {
	*identity.UserListItem
	// Manageable is false for a user the viewer outranks nothing of: someone
	// holding permissions the viewer does not.
	Manageable bool
}

func (ac *actor) view(it *identity.UserListItem) *ManagedUser {
	return &ManagedUser{UserListItem: it, Manageable: ac.canManage(it)}
}

// actor is the administrator making a call, with what they may do.
type actor struct {
	tenantID string
	userID   string
	perms    authz.PermissionSet
	native   bool
}

func (a *UserAdmin) actor(ctx context.Context, p authz.Principal, need authz.Permission) (*actor, error) {
	return a.actorWith(ctx, p, need, a.users.Access)
}

// actorTx is actor read inside a transaction.
func (a *UserAdmin) actorTx(ctx context.Context, tx *ent.Tx, p authz.Principal, need authz.Permission) (*actor, error) {
	return a.actorWith(ctx, p, need, func(ctx context.Context, tenantID, userID string) (authz.PermissionSet, bool, error) {
		return a.users.AccessTx(ctx, tx, tenantID, userID)
	})
}

func (a *UserAdmin) actorWith(ctx context.Context, p authz.Principal, need authz.Permission, access func(context.Context, string, string) (authz.PermissionSet, bool, error)) (*actor, error) {
	if err := requireSignedIn(p); err != nil {
		return nil, err
	}
	perms, native, err := access(ctx, p.TenantID, p.UserID)
	if err != nil {
		return nil, err
	}
	if !perms.Has(need) {
		return nil, fmt.Errorf("%w: you do not have the %s permission", authz.ErrForbidden, need)
	}
	return &actor{tenantID: p.TenantID, userID: p.UserID, perms: perms, native: native}, nil
}

// canManage reports whether the actor holds everything the target's role does.
func (ac *actor) canManage(target *identity.UserListItem) bool {
	return ac.perms.Covers(authz.EffectivePermissions(target.Role, ac.native))
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{authz.ErrInvalid}, args...)...)
}

func forbidden(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{authz.ErrForbidden}, args...)...)
}

// ListUsers returns a page of the organization's users and the total matching.
func (a *UserAdmin) ListUsers(ctx context.Context, p authz.Principal, f identity.UserFilter, after *identity.UserCursor, limit int) (items []*ManagedUser, hasNext bool, total int, err error) {
	ac, err := a.actor(ctx, p, authz.PermUsersRead)
	if err != nil {
		return nil, false, 0, err
	}
	rows, err := a.users.List(ctx, ac.tenantID, f, after, limit+1) // one extra says whether there is more
	if err != nil {
		return nil, false, 0, err
	}
	if hasNext = len(rows) > limit; hasNext {
		rows = rows[:limit]
	}
	if total, err = a.users.Count(ctx, ac.tenantID, f); err != nil {
		return nil, false, 0, err
	}
	items = make([]*ManagedUser, 0, len(rows))
	for _, r := range rows {
		items = append(items, ac.view(r))
	}
	return items, hasNext, total, nil
}

// IdentitySources lists the identity providers the organization's users can come from.
func (a *UserAdmin) IdentitySources(ctx context.Context, p authz.Principal) ([]*auth.IdpProvider, error) {
	ac, err := a.actor(ctx, p, authz.PermUsersRead)
	if err != nil {
		return nil, err
	}
	return a.idps.ListForTenant(ctx, ac.tenantID)
}

// CreateLocalUserInput describes a user to create on the built-in provider.
type CreateLocalUserInput struct {
	Email       string
	DisplayName string
	Password    string
	Role        string // defaults to MEMBER
}

// CreateLocalUser creates a user who signs in with an email and password.
func (a *UserAdmin) CreateLocalUser(ctx context.Context, p authz.Principal, in CreateLocalUserInput) (*ManagedUser, error) {
	ac, err := a.actor(ctx, p, authz.PermUsersCreate)
	if err != nil {
		return nil, err
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(in.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}
	role := in.Role
	if role == "" {
		role = authz.TenantRoleMember
	}
	if !authz.KnownTenantRole(role) {
		return nil, invalid("unknown role %q", role)
	}
	// Anything above an ordinary member is a promotion, and needs the right to give it.
	if role != authz.TenantRoleMember && !authz.CanAssignRole(ac.perms, role, ac.native) {
		return nil, forbidden("you cannot give the %s role", role)
	}

	idp, err := a.idps.LocalForTenant(ctx, ac.tenantID)
	if err != nil {
		return nil, err
	}
	// Hash before the transaction: bcrypt is slow and must not hold a connection.
	hash, err := local.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	var created *identity.User
	err = a.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		created, err = a.local.CreateUserTx(ctx, tx, identity.CreateUserParams{
			TenantID:    ac.tenantID,
			IdpID:       idp.ID,
			ExternalID:  email, // the sign-in name
			Email:       email,
			DisplayName: name,
			Role:        role,
		}, hash)
		return err
	})
	if errors.Is(err, identity.ErrConflict) {
		return nil, fmt.Errorf("%w: a user with the email %s already exists", identity.ErrConflict, email)
	}
	if err != nil {
		return nil, err
	}
	it, err := a.users.GetItem(ctx, ac.tenantID, created.ID)
	if err != nil {
		return nil, err
	}
	return ac.view(it), nil
}

// UpdateLocalUser changes a built-in user's display name and/or role. A user
// from an external provider is managed there, so it is refused.
func (a *UserAdmin) UpdateLocalUser(ctx context.Context, p authz.Principal, id string, displayName, role *string) (*ManagedUser, error) {
	if displayName != nil {
		n, err := normalizeName(*displayName)
		if err != nil {
			return nil, err
		}
		displayName = &n
	}

	var out *ManagedUser
	err := a.serialized(ctx, p, authz.PermUsersUpdate, func(tx *ent.Tx, ac *actor) error {
		target, err := a.localTarget(ctx, tx, ac, id)
		if err != nil {
			return err
		}
		if role != nil && *role != target.Role {
			if !ac.perms.Has(authz.PermRolesAssign) {
				return forbidden("you do not have the %s permission", authz.PermRolesAssign)
			}
			if !authz.CanAssignRole(ac.perms, *role, ac.native) {
				return forbidden("you cannot give the %s role", *role)
			}
			if err := a.keepAnAdmin(ctx, tx, ac, target, authz.EffectivePermissions(*role, ac.native)); err != nil {
				return err
			}
		} else {
			role = nil
		}
		if err := a.users.UpdateProfileTx(ctx, tx, ac.tenantID, id, displayName, role); err != nil {
			return err
		}
		it, err := a.users.GetItemTx(ctx, tx, ac.tenantID, id)
		if err != nil {
			return err
		}
		out = ac.view(it)
		return nil
	})
	return out, err
}

// ResetLocalUserPassword sets a new password for a built-in user.
func (a *UserAdmin) ResetLocalUserPassword(ctx context.Context, p authz.Principal, id, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	// Hash before the transaction: bcrypt is slow and must not hold a connection.
	hash, err := local.HashPassword(password)
	if err != nil {
		return err
	}
	// A password does not change anyone's permissions, so this needs no lock.
	return a.db.WithTx(ctx, func(tx *ent.Tx) error {
		ac, err := a.actorTx(ctx, tx, p, authz.PermUsersUpdate)
		if err != nil {
			return err
		}
		if _, err := a.localTarget(ctx, tx, ac, id); err != nil {
			return err
		}
		return a.local.SetPasswordHashTx(ctx, tx, id, hash)
	})
}

// SetUserDisabled blocks or restores sign-in for a user of any provider.
func (a *UserAdmin) SetUserDisabled(ctx context.Context, p authz.Principal, id string, disabled bool) (*ManagedUser, error) {
	var out *ManagedUser
	err := a.serialized(ctx, p, authz.PermUsersDisable, func(tx *ent.Tx, ac *actor) error {
		target, err := a.users.GetItemTx(ctx, tx, ac.tenantID, id)
		if err != nil {
			return err
		}
		if !ac.canManage(target) {
			return forbidden("you cannot manage a user with more permissions than you hold")
		}
		if disabled {
			if id == ac.userID {
				return forbidden("you cannot disable your own account")
			}
			if err := a.keepAnAdmin(ctx, tx, ac, target, authz.NewPermissionSet()); err != nil {
				return err
			}
		}
		if _, err := a.users.SetDisabledTx(ctx, tx, ac.tenantID, id, disabled); err != nil {
			return err
		}
		it, err := a.users.GetItemTx(ctx, tx, ac.tenantID, id)
		if err != nil {
			return err
		}
		out = ac.view(it)
		return nil
	})
	return out, err
}

// serialized runs a change that may remove an administrator. It locks the
// tenant first, so two such changes never run at once: without that, two super
// admins demoting each other would each see the other still in place, both
// checks would pass, and nobody would be left. The actor's own permissions are
// read after the lock, so a concurrent demotion of the actor counts too.
func (a *UserAdmin) serialized(ctx context.Context, p authz.Principal, need authz.Permission, fn func(tx *ent.Tx, ac *actor) error) error {
	if err := requireSignedIn(p); err != nil {
		return err
	}
	return a.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *ent.Tx) error {
		if err := a.users.LockTenantTx(ctx, tx, p.TenantID); err != nil {
			return err
		}
		ac, err := a.actorTx(ctx, tx, p, need)
		if err != nil {
			return err
		}
		return fn(tx, ac)
	})
}

// localTarget loads a user the actor wants to edit: it must belong to the
// built-in provider and be one the actor may manage.
func (a *UserAdmin) localTarget(ctx context.Context, tx *ent.Tx, ac *actor, id string) (*identity.UserListItem, error) {
	target, err := a.users.GetItemTx(ctx, tx, ac.tenantID, id)
	if err != nil {
		return nil, err
	}
	if target.IdpType != "LOCAL" {
		return nil, invalid("this user is managed by %s and cannot be edited here", target.IdpName)
	}
	if !ac.canManage(target) {
		return nil, forbidden("you cannot manage a user with more permissions than you hold")
	}
	return target, nil
}

// keepAnAdmin refuses a change that would leave the organization with nobody
// who can assign roles: afterwards the target would hold `after` instead of
// what their role carries today. Callers hold the tenant lock.
func (a *UserAdmin) keepAnAdmin(ctx context.Context, tx *ent.Tx, ac *actor, target *identity.UserListItem, after authz.PermissionSet) error {
	if target.Disabled() {
		return nil // already not counted
	}
	if !authz.EffectivePermissions(target.Role, ac.native).Has(authz.PermRolesAssign) || after.Has(authz.PermRolesAssign) {
		return nil
	}
	others, err := a.users.CountActiveWithRolesTx(ctx, tx, ac.tenantID, authz.RolesHolding(authz.PermRolesAssign, ac.native), target.ID)
	if err != nil {
		return err
	}
	if others == 0 {
		return forbidden("this is the last administrator who can assign roles")
	}
	return nil
}

func normalizeEmail(s string) (string, error) {
	s = local.NormalizeLogin(s)
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > 255 {
		return "", invalid("%q is not a valid email address", s)
	}
	return s, nil
}

func normalizeName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxNameLen {
		return "", invalid("a name is required, up to %d characters", maxNameLen)
	}
	return s, nil
}

func validatePassword(s string) error {
	if len(s) < minPasswordLen || len(s) > maxPasswordLen {
		return invalid("a password must be %d to %d characters", minPasswordLen, maxPasswordLen)
	}
	return nil
}
