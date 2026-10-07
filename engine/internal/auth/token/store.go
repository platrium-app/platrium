package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/authtoken"
	"platrium/internal/infra/db/ent/user"
)

// ErrInvalidToken means the secret is unknown, expired, idle for too long, or
// was revoked. Callers must not distinguish between these.
var ErrInvalidToken = errors.New("invalid token")

// DefaultIdleTimeout is how long a token may go unused before it dies.
const DefaultIdleTimeout = 90 * 24 * time.Hour

// touchInterval throttles last_used_at writes so a busy client does not
// write on every request.
const touchInterval = 5 * time.Minute

// Store persists tokens and, for device sign-ins, the device they belong to.
type Store struct {
	db          *db.DB
	devices     *identity.EntDeviceStore
	idleTimeout time.Duration
	now         func() time.Time
}

func NewStore(d *db.DB, devices *identity.EntDeviceStore, idleTimeout time.Duration) *Store {
	if idleTimeout <= 0 {
		idleTimeout = DefaultIdleTimeout
	}
	return &Store{db: d, devices: devices, idleTimeout: idleTimeout, now: func() time.Time { return time.Now().UTC() }}
}

// IssueReq describes a token to create. Set Device to register a device with
// it; leave it nil for an app token.
type IssueReq struct {
	TenantID  string
	UserID    string
	Name      string
	Device    *identity.RegisterDeviceReq
	ExpiresAt *time.Time
}

// Issued is a freshly created token. Secret is only available here.
type Issued struct {
	ID        string
	Secret    string
	DeviceID  string // empty for app tokens
	ExpiresAt *time.Time
}

// Principal is who a valid token acts as.
type Principal struct {
	TokenID  string
	DeviceID string // empty for app tokens
	TenantID string
	UserID   string
	Email    string
	// IssuedAt is when the token was created. A user's sessions_valid_after
	// voids tokens issued before it.
	IssuedAt time.Time
}

// Client is one entry in a user's list of signed-in devices and apps.
type Client struct {
	ID         string // token ID; also what Delete takes
	Name       string
	DeviceID   string
	Platform   string
	AppVersion string
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  *time.Time
}

// IsDevice reports whether the client is a registered device rather than an app.
func (c Client) IsDevice() bool { return c.DeviceID != "" }

// Issue creates a token (and its device, if requested) for a user.
func (s *Store) Issue(ctx context.Context, req IssueReq) (*Issued, error) {
	secret, hash, display, err := Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	out := &Issued{Secret: secret, ExpiresAt: req.ExpiresAt}
	err = s.db.WithTx(ctx, func(tx *ent.Tx) error {
		ok, err := tx.User.Query().Where(user.ID(req.UserID), user.TenantID(req.TenantID)).Exist(ctx)
		if err != nil {
			return fmt.Errorf("failed to look up user: %w", err)
		}
		if !ok {
			return fmt.Errorf("%w: user not found in tenant", identity.ErrNotFound)
		}

		create := tx.AuthToken.Create().
			SetTenantID(req.TenantID).
			SetUserID(req.UserID).
			SetName(req.Name).
			SetTokenHash(hash).
			SetTokenPrefix(display).
			SetLastUsedAt(s.now()).
			SetNillableExpiresAt(req.ExpiresAt)
		if req.Device != nil {
			dev, err := s.devices.CreateTx(ctx, tx, req.TenantID, req.UserID, *req.Device)
			if err != nil {
				return err
			}
			create.SetDeviceID(dev.ID)
			out.DeviceID = dev.ID
		}
		row, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create token: %w", err)
		}
		out.ID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Validate resolves a secret to its principal, enforcing absolute expiry and
// the idle timeout, and records use.
func (s *Store) Validate(ctx context.Context, secret string) (*Principal, error) {
	if !LooksLikeToken(secret) {
		return nil, ErrInvalidToken
	}
	row, err := s.db.AuthToken.Query().
		Where(authtoken.TokenHash(Hash(secret))).
		WithUser().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("failed to look up token: %w", err)
	}

	now := s.now()
	if row.ExpiresAt != nil && !now.Before(*row.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	if now.Sub(row.LastUsedAt) > s.idleTimeout {
		return nil, ErrInvalidToken
	}
	if now.Sub(row.LastUsedAt) > touchInterval {
		// Best effort: failing to record use must not fail the request.
		_ = s.db.AuthToken.UpdateOneID(row.ID).SetLastUsedAt(now).Exec(ctx)
	}

	// A disabled user's tokens stop working at once, without being deleted, so
	// re-enabling the account restores their devices.
	if u := row.Edges.User; u != nil && u.DisabledAt != nil {
		return nil, ErrInvalidToken
	}

	p := &Principal{TokenID: row.ID, TenantID: row.TenantID, UserID: row.UserID, IssuedAt: row.CreatedAt}
	if row.DeviceID != nil {
		p.DeviceID = *row.DeviceID
	}
	if u := row.Edges.User; u != nil {
		p.Email = u.Email
	}
	return p, nil
}

// List returns a user's signed-in devices and apps, newest first.
func (s *Store) List(ctx context.Context, tenantID, userID string) ([]Client, error) {
	rows, err := s.db.AuthToken.Query().
		Where(authtoken.TenantID(tenantID), authtoken.UserID(userID)).
		WithDevice().
		Order(authtoken.ByCreatedAt(sql.OrderDesc()), authtoken.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}
	out := make([]Client, 0, len(rows))
	for _, r := range rows {
		c := Client{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, ExpiresAt: r.ExpiresAt}
		if d := r.Edges.Device; d != nil {
			c.DeviceID, c.Platform, c.AppVersion = d.ID, d.Platform, d.AppVersion
		}
		out = append(out, c)
	}
	return out, nil
}

// Delete revokes one of the user's tokens, and deletes its device with it.
func (s *Store) Delete(ctx context.Context, tenantID, userID, tokenID string) error {
	return s.db.WithTx(ctx, func(tx *ent.Tx) error {
		row, err := tx.AuthToken.Query().
			Where(authtoken.ID(tokenID), authtoken.TenantID(tenantID), authtoken.UserID(userID)).
			Only(ctx)
		if ent.IsNotFound(err) {
			return fmt.Errorf("%w: token not found", identity.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("failed to look up token: %w", err)
		}
		if row.DeviceID != nil {
			// The foreign key cascades to the token.
			return tx.Device.DeleteOneID(*row.DeviceID).Exec(ctx)
		}
		return tx.AuthToken.DeleteOneID(row.ID).Exec(ctx)
	})
}
