package local

import (
	"context"
	"fmt"
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
	db *db.DB
}

func NewLocalUserStore(d *db.DB) *LocalUserStore {
	return &LocalUserStore{db: d}
}

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

// CreateTx stores a password hash for a user within the provided transaction.
// The user must belong to tenantID and be bound to a LOCAL identity provider.
func (s *LocalUserStore) CreateTx(ctx context.Context, tx *ent.Tx, tenantID, userID, passwordHash string) error {
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

	n, err := s.db.LocalCredential.Update().
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
