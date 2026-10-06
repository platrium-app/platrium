package local

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/idpprovider"
	"platrium/internal/infra/db/ent/localcredential"
	"platrium/internal/infra/db/ent/user"
)

// LocalUserStore manages the passwords (and, later, second factors) of users
// authenticated by the built-in LOCAL identity provider.
type LocalUserStore struct {
	db          *db.DB
	provisioner UserProvisioner
}

// UserProvisioner creates the platform identity (user row and private drive)
// that every IdP's users have. The orchestrator implements it; it is an
// interface here only because the orchestrator imports this package.
type UserProvisioner interface {
	ProvisionUserTx(ctx context.Context, tx *ent.Tx, p identity.CreateUserParams) (*identity.User, error)
}

func NewLocalUserStore(d *db.DB, p UserProvisioner) *LocalUserStore {
	return &LocalUserStore{db: d, provisioner: p}
}

// NormalizeLogin is the form a local user's sign-in name is stored and looked
// up in. Email addresses are case-insensitive to people, but the column they
// live in is compared byte for byte (OIDC subjects must be), so local logins
// are lowercased on the way in and on the way to the lookup.
func NormalizeLogin(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// HashPassword hashes a plaintext password with bcrypt. It is deliberately
// separate from the write methods: hashing takes ~100ms, so callers hash
// before opening a transaction instead of holding a connection while it runs.
func HashPassword(rawPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CreateUserTx creates a user on a LOCAL identity provider together with their
// password, within the provided transaction. It is the only way a local user
// comes into being: the platform identity is provisioned like any other IdP's
// user, then the credential row is attached to it. p.IdpID must be a LOCAL
// provider of p.TenantID.
func (s *LocalUserStore) CreateUserTx(ctx context.Context, tx *ent.Tx, p identity.CreateUserParams, passwordHash string) (*identity.User, error) {
	u, err := s.provisioner.ProvisionUserTx(ctx, tx, p)
	if err != nil {
		return nil, err
	}
	if err := s.createCredentialTx(ctx, tx, u.TenantID, u.ID, passwordHash); err != nil {
		return nil, err
	}
	return u, nil
}

// createCredentialTx stores a password hash for a user within the provided
// transaction. The user must belong to tenantID and be bound to a LOCAL
// identity provider.
func (s *LocalUserStore) createCredentialTx(ctx context.Context, tx *ent.Tx, tenantID, userID, passwordHash string) error {
	// Tenant isolation, and only LOCAL-IdP users may hold a local credential.
	ok, err := tx.User.Query().
		Where(user.ID(userID), user.TenantID(tenantID), user.HasIdpWith(idpprovider.TypeEQ(idpprovider.TypeLOCAL))).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to look up user: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: local user not found in tenant", identity.ErrNotFound)
	}

	if err := tx.LocalCredential.Create().
		SetTenantID(tenantID).
		SetUserID(userID).
		SetPasswordHash(passwordHash).
		Exec(ctx); err != nil {
		if ent.IsConstraintError(err) {
			return fmt.Errorf("%w: user already has a local credential: %v", identity.ErrConflict, err)
		}
		return fmt.Errorf("failed to create local credential: %w", err)
	}
	return nil
}

// SetPassword handles password resets. It preserves existing 2FA settings.
func (s *LocalUserStore) SetPassword(ctx context.Context, userID, rawPassword string) error {
	hash, err := HashPassword(rawPassword)
	if err != nil {
		return err
	}
	return setPasswordHash(ctx, s.db.Client, userID, hash)
}

// SetPasswordHashTx stores an already hashed password within a transaction
// (see HashPassword for why hashing happens before one opens).
func (s *LocalUserStore) SetPasswordHashTx(ctx context.Context, tx *ent.Tx, userID, passwordHash string) error {
	return setPasswordHash(ctx, tx.Client(), userID, passwordHash)
}

func setPasswordHash(ctx context.Context, c *ent.Client, userID, hash string) error {
	n, err := c.LocalCredential.Update().
		Where(localcredential.UserID(userID)).
		SetPasswordHash(hash).
		SetPasswordChangedAt(time.Now().UTC()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: local credential", identity.ErrNotFound)
	}
	return nil
}

// VerifyPassword checks a plaintext password against the stored hash. A wrong
// password returns (false, nil); a missing credential returns an ErrNotFound.
func (s *LocalUserStore) VerifyPassword(ctx context.Context, userID, rawPassword string) (bool, error) {
	cred, err := s.db.LocalCredential.Query().Where(localcredential.UserID(userID)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, fmt.Errorf("%w: local credential", identity.ErrNotFound)
		}
		return false, fmt.Errorf("failed to load local credential: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(rawPassword)); err != nil {
		return false, nil // Invalid password
	}
	return true, nil
}
