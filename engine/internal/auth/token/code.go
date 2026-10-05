package token

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platrium/internal/infra/kvstore"
)

// CodeTTL is how long an authorization code can be redeemed.
const CodeTTL = 5 * time.Minute

// ErrInvalidCode means the code is unknown, expired, already used, or the PKCE
// verifier does not match.
var ErrInvalidCode = errors.New("invalid authorization code")

// Grant is what a user approved: who, and what kind of client gets the token.
// It is stored behind a one-time code until the client redeems it.
type Grant struct {
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id"`
	Name       string `json:"name"`
	Platform   string `json:"platform,omitempty"` // set for devices, empty for apps
	AppVersion string `json:"app_version,omitempty"`

	// Challenge is the base64url SHA-256 of the client's secret verifier (PKCE S256).
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CodeStore hands out single-use authorization codes.
type CodeStore struct {
	kv  kvstore.KVStore
	now func() time.Time
}

func NewCodeStore(kv kvstore.KVStore) *CodeStore {
	return &CodeStore{kv: kv, now: func() time.Time { return time.Now().UTC() }}
}

// Create stores a grant and returns the code that redeems it.
func (c *CodeStore) Create(ctx context.Context, g Grant) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(b)
	g.ExpiresAt = c.now().Add(CodeTTL)
	raw, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	// Keyed by the hash so a leaked KV dump does not expose redeemable codes.
	err = c.kv.WriteTx(ctx, func(tx kvstore.Tx) error {
		return tx.Set(codeKey(code), raw, kvstore.WithTTL(CodeTTL))
	})
	if err != nil {
		return "", fmt.Errorf("failed to store authorization code: %w", err)
	}
	return code, nil
}

// Redeem consumes a code. The code is burned on the first attempt, whether or
// not the verifier is right, so a guessed verifier cannot be retried.
func (c *CodeStore) Redeem(ctx context.Context, code, verifier string) (*Grant, error) {
	var raw []byte
	err := c.kv.WriteTx(ctx, func(tx kvstore.Tx) error {
		v, err := tx.Get(codeKey(code))
		if err != nil {
			return err
		}
		raw = v
		return tx.Delete(codeKey(code))
	})
	if errors.Is(err, kvstore.ErrNotFound) {
		return nil, ErrInvalidCode
	}
	if err != nil {
		return nil, fmt.Errorf("failed to redeem authorization code: %w", err)
	}

	var g Grant
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, ErrInvalidCode
	}
	if !c.now().Before(g.ExpiresAt) {
		return nil, ErrInvalidCode
	}
	if !VerifyPKCE(g.Challenge, verifier) {
		return nil, ErrInvalidCode
	}
	return &g, nil
}

func codeKey(code string) kvstore.Key {
	return kvstore.Key{Namespace: kvstore.NSAuthCode, ID: Hash(code)}
}

// VerifyPKCE checks a verifier against an S256 challenge.
func VerifyPKCE(challenge, verifier string) bool {
	if challenge == "" || len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}

// Challenge computes the S256 challenge for a verifier. Clients and tests use it.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
