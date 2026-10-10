package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"platrium/internal/apperr"
	"platrium/internal/auth/session"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
)

// AuthManager is the core domain service that processes successful logins.
// It acts as the bridge between the protocol handlers and the database / Session layer.
type AuthManager interface {
	// HandleFederatedLogin takes the normalized claims of a sign-in an external
	// provider has already verified, ensures the user exists (creating them when
	// the provider allows it), and starts the browser session.
	HandleFederatedLogin(ctx context.Context, handoff IdpAuthHandoff) error
}

// UserProvisioner creates the platform identity (user row and private drive)
// that every IdP's users have. The orchestrator implements it; it is an
// interface here only because the orchestrator imports this package.
type UserProvisioner interface {
	ProvisionUserTx(ctx context.Context, tx *ent.Tx, p identity.CreateUserParams) (*identity.User, error)
}

// Manager is the AuthManager.
type Manager struct {
	db          *db.DB
	idps        *IdpStore
	users       *identity.UserStore
	provisioner UserProvisioner
	sessions    *scs.SessionManager
	now         func() time.Time
}

var _ AuthManager = (*Manager)(nil)

func NewManager(d *db.DB, idps *IdpStore, users *identity.UserStore, p UserProvisioner, sessions *scs.SessionManager) *Manager {
	return &Manager{db: d, idps: idps, users: users, provisioner: p, sessions: sessions, now: time.Now}
}

// HandleFederatedLogin signs in the user a provider vouched for. Identity is
// the pair (provider, subject): an account is never found or merged by email,
// so one provider can not sign in as a user of another.
func (m *Manager) HandleFederatedLogin(ctx context.Context, h IdpAuthHandoff) error {
	idp, err := m.idps.GetIdpById(ctx, h.IdpProviderID)
	if err != nil {
		return fmt.Errorf("%w: identity provider", apperr.ErrNotFound)
	}
	if idp.IsLocal() {
		return fmt.Errorf("%w: the built-in provider does not sign in through federation", apperr.ErrInvalid)
	}
	email := strings.TrimSpace(h.Email)
	if h.SubjectID == "" || email == "" {
		return fmt.Errorf("%w: a subject and an email address are required", apperr.ErrInvalid)
	}
	name := strings.TrimSpace(h.DisplayName)
	if name == "" {
		name = email
	}

	user, err := m.resolveUser(ctx, idp, h.SubjectID, email, name)
	if err != nil {
		return err
	}

	// A fresh session ID on privilege change defeats session fixation.
	m.sessions.Put(ctx, session.StoreKey, &session.PlatriumSession{
		UserID:   user.ID,
		TenantID: idp.TenantID,
		Email:    user.Email,
		IssuedAt: m.now().UTC(),
	})
	return m.sessions.RenewToken(ctx)
}

// resolveUser returns the user for (idp, subject), admitted and with their
// profile current. It creates the user on a first sign-in when the provider
// allows it.
func (m *Manager) resolveUser(ctx context.Context, idp *IdpProvider, subject, email, name string) (*identity.User, error) {
	user, _, err := m.users.GetUserByExternalId(ctx, idp.ID, subject)
	if errors.Is(err, identity.ErrNotFound) {
		user, err = m.firstSignIn(ctx, idp, subject, email, name)
		if errors.Is(err, identity.ErrConflict) {
			// Lost a race with this user's other first sign-in: theirs won.
			user, _, err = m.users.GetUserByExternalId(ctx, idp.ID, subject)
		} else {
			return user, err
		}
	}
	if err != nil {
		return nil, err
	}

	if err := idp.Admit(email, user); err != nil {
		return nil, err
	}
	// The provider is the source of truth for who this person is.
	if user.Email != email || user.DisplayName != name {
		if err := m.users.SyncProfile(ctx, idp.TenantID, user.ID, email, name); err != nil {
			return nil, err
		}
		user.Email, user.DisplayName = email, name
	}
	return user, nil
}

func (m *Manager) firstSignIn(ctx context.Context, idp *IdpProvider, subject, email, name string) (*identity.User, error) {
	if err := idp.Admit(email, nil); err != nil {
		return nil, err
	}
	if !idp.JITUsers {
		return nil, ErrNotProvisioned
	}
	// The policy was checked when it was saved; the database is not trusted to
	// still hold a safe one.
	if err := ValidateJITRole(idp.DefaultRole); err != nil {
		return nil, err
	}

	var user *identity.User
	err := m.db.WithTx(ctx, func(tx *ent.Tx) error {
		var err error
		user, err = m.provisioner.ProvisionUserTx(ctx, tx, identity.CreateUserParams{
			TenantID:    idp.TenantID,
			IdpID:       idp.ID,
			ExternalID:  subject,
			Email:       email,
			DisplayName: name,
			Role:        idp.DefaultRole,
		})
		return err
	})
	return user, err
}
