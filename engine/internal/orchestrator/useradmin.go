package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"platrium/internal/auth"
	"platrium/internal/auth/actor"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
)

const maxNameLen = 255

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
	// EditableFields is what this viewer may change on this user. It is the one
	// definition of that: the API reports it and UpdateUser enforces the same
	// rules, so a client never has to guess.
	EditableFields []UserField
}

// UserField is something about a user an administrator may change.
type UserField string

const (
	// UserFieldDisplayName is the user's name. Only a built-in user's: for an
	// external user the provider owns it and refreshes it at every sign-in.
	UserFieldDisplayName UserField = "DISPLAY_NAME"
	// UserFieldRole is the role. It is Platrium's own, whatever the provider.
	UserFieldRole UserField = "ROLE"
	// UserFieldPassword is the password. Only a built-in user has one here.
	UserFieldPassword UserField = "PASSWORD"
)

// isLocalUser reports whether Platrium itself owns the user's profile and
// password, as opposed to the identity provider they sign in through.
func isLocalUser(u *identity.UserListItem) bool { return auth.IdpType(u.IdpType).IsLocal() }

// isSelf reports whether the target is the actor. Nobody changes their own
// access (role) or switches themselves off: with one click that would lock out
// the only person who could have undone it. Another administrator does it.
func isSelf(ac *actor.Identity, target *identity.UserListItem) bool {
	return target.ID == ac.UserID
}

// assignableRoleFor reports whether the actor could give the user a role other
// than the one they have.
func assignableRoleFor(ac *actor.Identity, target *identity.UserListItem) bool {
	if isSelf(ac, target) || !ac.Perms.Has(authz.PermRolesAssign) {
		return false
	}
	for _, r := range authz.TenantRoles() {
		if r != target.Role && authz.CanAssignRole(ac.Perms, r, ac.Native) {
			return true
		}
	}
	return false
}

// editableFields is what the actor may change on the target.
func editableFields(ac *actor.Identity, target *identity.UserListItem) []UserField {
	fields := []UserField{}
	if !canManage(ac, target) || !ac.Perms.Has(authz.PermUsersUpdate) {
		return fields
	}
	if isLocalUser(target) {
		fields = append(fields, UserFieldDisplayName)
	}
	if assignableRoleFor(ac, target) {
		fields = append(fields, UserFieldRole)
	}
	if isLocalUser(target) {
		fields = append(fields, UserFieldPassword)
	}
	return fields
}

// require reads the administrator's permissions now and checks they include
// need. It is deliberately not the identity a transport resolved for the
// request: this is the authoritative gate (see the type's comment).
func (a *UserAdmin) require(ctx context.Context, p authz.Principal, need authz.Permission) (*actor.Identity, error) {
	return a.requireWith(ctx, p, need, a.users.Access)
}

// requireTx is require read inside a transaction.
func (a *UserAdmin) requireTx(ctx context.Context, tx *ent.Tx, p authz.Principal, need authz.Permission) (*actor.Identity, error) {
	return a.requireWith(ctx, p, need, func(ctx context.Context, tenantID, userID string) (authz.PermissionSet, bool, error) {
		return a.users.AccessTx(ctx, tx, tenantID, userID)
	})
}

func (a *UserAdmin) requireWith(ctx context.Context, p authz.Principal, need authz.Permission, access accessFunc) (*actor.Identity, error) {
	return requirePermission(ctx, p, need, access)
}

// accessFunc reads a user's permissions (and whether they are in the native tenant).
type accessFunc = func(ctx context.Context, tenantID, userID string) (authz.PermissionSet, bool, error)

// requirePermission is the authoritative gate shared by every administration
// orchestrator: it reads the caller's permissions now and checks need.
func requirePermission(ctx context.Context, p authz.Principal, need authz.Permission, access accessFunc) (*actor.Identity, error) {
	if err := requireSignedIn(p); err != nil {
		return nil, err
	}
	perms, native, err := access(ctx, p.TenantID, p.UserID)
	if err != nil {
		return nil, err
	}
	if err := needPermission(perms, need); err != nil {
		return nil, err
	}
	return &actor.Identity{Principal: authz.Principal{TenantID: p.TenantID, UserID: p.UserID}, Perms: perms, Native: native}, nil
}

// canManage reports whether the administrator holds everything the target's role does.
func canManage(ac *actor.Identity, target *identity.UserListItem) bool {
	return ac.Perms.Covers(authz.EffectivePermissions(target.Role, ac.Native))
}

// view is a user as the administrator sees them.
func view(ac *actor.Identity, it *identity.UserListItem) *ManagedUser {
	return &ManagedUser{UserListItem: it, Manageable: canManage(ac, it), EditableFields: editableFields(ac, it)}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{authz.ErrInvalid}, args...)...)
}

func forbidden(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{authz.ErrForbidden}, args...)...)
}

// ListUsers returns a page of the organization's users and the total matching.
func (a *UserAdmin) ListUsers(ctx context.Context, p authz.Principal, f identity.UserFilter, after *identity.UserCursor, limit int) (items []*ManagedUser, hasNext bool, total int, err error) {
	ac, err := a.require(ctx, p, authz.PermUsersRead)
	if err != nil {
		return nil, false, 0, err
	}
	rows, err := a.users.List(ctx, ac.TenantID, f, after, limit+1) // one extra says whether there is more
	if err != nil {
		return nil, false, 0, err
	}
	if hasNext = len(rows) > limit; hasNext {
		rows = rows[:limit]
	}
	if total, err = a.users.Count(ctx, ac.TenantID, f); err != nil {
		return nil, false, 0, err
	}
	items = make([]*ManagedUser, 0, len(rows))
	for _, r := range rows {
		items = append(items, view(ac, r))
	}
	return items, hasNext, total, nil
}

// IdentitySources lists the identity providers the organization's users can come from.
func (a *UserAdmin) IdentitySources(ctx context.Context, p authz.Principal) ([]*auth.IdpProvider, error) {
	ac, err := a.require(ctx, p, authz.PermUsersRead)
	if err != nil {
		return nil, err
	}
	return a.idps.ListForTenant(ctx, ac.TenantID)
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
	ac, err := a.require(ctx, p, authz.PermUsersCreate)
	if err != nil {
		return nil, err
	}
	email, err := local.NormalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	name, err := normalizeName(in.DisplayName)
	if err != nil {
		return nil, err
	}
	if err := local.ValidatePassword(in.Password); err != nil {
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
	if role != authz.TenantRoleMember && !authz.CanAssignRole(ac.Perms, role, ac.Native) {
		return nil, forbidden("you cannot give the %s role", role)
	}

	idp, err := a.idps.LocalForTenant(ctx, ac.TenantID)
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
			TenantID:    ac.TenantID,
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
	it, err := a.users.GetItem(ctx, ac.TenantID, created.ID)
	if err != nil {
		return nil, err
	}
	return view(ac, it), nil
}

// UpdateUser changes a user's display name and/or role.
//
// The role is Platrium's own for every user, so it can be changed whichever
// provider they sign in through (promoting someone who arrived through SSO is
// how they become an administrator). The name belongs to whoever owns the
// profile: Platrium for a built-in user, the provider for an external one, so
// changing an external user's name is refused. See EditableFields.
func (a *UserAdmin) UpdateUser(ctx context.Context, p authz.Principal, id string, displayName, role *string) (*ManagedUser, error) {
	if displayName != nil {
		n, err := normalizeName(*displayName)
		if err != nil {
			return nil, err
		}
		displayName = &n
	}

	var out *ManagedUser
	apply := func(tx *ent.Tx, ac *actor.Identity) error {
		target, err := a.manageableTarget(ctx, tx, ac, id)
		if err != nil {
			return err
		}
		if displayName != nil && *displayName == target.DisplayName {
			displayName = nil // unchanged is not an edit
		}
		if displayName != nil && !isLocalUser(target) {
			return invalid("%s manages this user's name; it updates when they sign in", target.IdpName)
		}
		if role != nil && *role != target.Role {
			if isSelf(ac, target) {
				return forbidden("you cannot change your own role; ask another administrator")
			}
			if !ac.Perms.Has(authz.PermRolesAssign) {
				return forbidden("you do not have the %s permission", authz.PermRolesAssign)
			}
			if !authz.CanAssignRole(ac.Perms, *role, ac.Native) {
				return forbidden("you cannot give the %s role", *role)
			}
			if err := a.keepAnAdmin(ctx, tx, ac, target, authz.EffectivePermissions(*role, ac.Native)); err != nil {
				return err
			}
		} else {
			role = nil
		}
		if err := a.users.UpdateProfileTx(ctx, tx, ac.TenantID, id, displayName, role); err != nil {
			return err
		}
		it, err := a.users.GetItemTx(ctx, tx, ac.TenantID, id)
		if err != nil {
			return err
		}
		out = view(ac, it)
		return nil
	}

	var err error
	if role != nil {
		// A role change can remove an administrator, so it takes the tenant lock.
		err = a.serialized(ctx, p, authz.PermUsersUpdate, apply)
	} else {
		// A name changes nobody's permissions, so it needs no lock.
		err = a.db.WithTx(ctx, func(tx *ent.Tx) error {
			ac, err := a.requireTx(ctx, tx, p, authz.PermUsersUpdate)
			if err != nil {
				return err
			}
			return apply(tx, ac)
		})
	}
	if err == nil {
		actor.Invalidate(ctx)
	}
	return out, err
}

// ResetLocalUserPassword sets a new password for a built-in user and signs them
// out everywhere: a reset usually means the old password is no longer trusted,
// so no session or token issued before it survives.
func (a *UserAdmin) ResetLocalUserPassword(ctx context.Context, p authz.Principal, id, password string) error {
	if err := local.ValidatePassword(password); err != nil {
		return err
	}
	// Hash before the transaction: bcrypt is slow and must not hold a connection.
	hash, err := local.HashPassword(password)
	if err != nil {
		return err
	}
	// A password does not change anyone's permissions, so this needs no lock.
	err = a.db.WithTx(ctx, func(tx *ent.Tx) error {
		ac, err := a.requireTx(ctx, tx, p, authz.PermUsersUpdate)
		if err != nil {
			return err
		}
		if _, err := a.localTarget(ctx, tx, ac, id); err != nil {
			return err
		}
		if err := a.local.SetPasswordHashTx(ctx, tx, id, hash); err != nil {
			return err
		}
		return a.users.RevokeSessionsTx(ctx, tx, ac.TenantID, id)
	})
	if err == nil {
		actor.Invalidate(ctx)
	}
	return err
}

// SetUserDisabled blocks or restores sign-in for a user of any provider.
func (a *UserAdmin) SetUserDisabled(ctx context.Context, p authz.Principal, id string, disabled bool) (*ManagedUser, error) {
	var out *ManagedUser
	err := a.serialized(ctx, p, authz.PermUsersDisable, func(tx *ent.Tx, ac *actor.Identity) error {
		target, err := a.users.GetItemTx(ctx, tx, ac.TenantID, id)
		if err != nil {
			return err
		}
		if !canManage(ac, target) {
			return forbidden("you cannot manage a user with more permissions than you hold")
		}
		if disabled {
			if isSelf(ac, target) {
				return forbidden("you cannot disable your own account")
			}
			if err := a.keepAnAdmin(ctx, tx, ac, target, authz.NewPermissionSet()); err != nil {
				return err
			}
		}
		if err := a.users.SetDisabledTx(ctx, tx, ac.TenantID, id, disabled); err != nil {
			return err
		}
		it, err := a.users.GetItemTx(ctx, tx, ac.TenantID, id)
		if err != nil {
			return err
		}
		out = view(ac, it)
		return nil
	})
	if err == nil {
		actor.Invalidate(ctx)
	}
	return out, err
}

// serialized runs a change that may remove an administrator. It locks the
// tenant first, so two such changes never run at once: without that, two super
// admins demoting each other would each see the other still in place, both
// checks would pass, and nobody would be left. The actor's own permissions are
// read after the lock, so a concurrent demotion of the actor counts too.
func (a *UserAdmin) serialized(ctx context.Context, p authz.Principal, need authz.Permission, fn func(tx *ent.Tx, ac *actor.Identity) error) error {
	if err := requireSignedIn(p); err != nil {
		return err
	}
	return a.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *ent.Tx) error {
		if err := a.users.LockTenantTx(ctx, tx, p.TenantID); err != nil {
			return err
		}
		ac, err := a.requireTx(ctx, tx, p, need)
		if err != nil {
			return err
		}
		return fn(tx, ac)
	})
}

// manageableTarget loads a user the actor wants to change: one they may manage.
func (a *UserAdmin) manageableTarget(ctx context.Context, tx *ent.Tx, ac *actor.Identity, id string) (*identity.UserListItem, error) {
	target, err := a.users.GetItemTx(ctx, tx, ac.TenantID, id)
	if err != nil {
		return nil, err
	}
	if !canManage(ac, target) {
		return nil, forbidden("you cannot manage a user with more permissions than you hold")
	}
	return target, nil
}

// localTarget is manageableTarget for something only a built-in user has (a
// password): an external user's is the provider's.
func (a *UserAdmin) localTarget(ctx context.Context, tx *ent.Tx, ac *actor.Identity, id string) (*identity.UserListItem, error) {
	target, err := a.manageableTarget(ctx, tx, ac, id)
	if err != nil {
		return nil, err
	}
	if !isLocalUser(target) {
		return nil, invalid("this user is managed by %s and cannot be edited here", target.IdpName)
	}
	return target, nil
}

// keepAnAdmin refuses a change that would leave the organization with nobody
// who can assign roles: afterwards the target would hold `after` instead of
// what their role carries today. Callers hold the tenant lock.
func (a *UserAdmin) keepAnAdmin(ctx context.Context, tx *ent.Tx, ac *actor.Identity, target *identity.UserListItem, after authz.PermissionSet) error {
	if target.Disabled() {
		return nil // already not counted
	}
	if !authz.EffectivePermissions(target.Role, ac.Native).Has(authz.PermRolesAssign) || after.Has(authz.PermRolesAssign) {
		return nil
	}
	others, err := a.users.CountActiveWithRolesTx(ctx, tx, ac.TenantID, authz.RolesHolding(authz.PermRolesAssign, ac.Native), target.ID)
	if err != nil {
		return err
	}
	if others == 0 {
		return forbidden("this is the last administrator who can assign roles")
	}
	return nil
}

func normalizeName(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > maxNameLen {
		return "", invalid("a name is required, up to %d characters", maxNameLen)
	}
	return s, nil
}
