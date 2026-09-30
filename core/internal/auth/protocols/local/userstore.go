package local

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"platrium/internal/infra/kvstore"

	"golang.org/x/crypto/bcrypt"
)

type LocalIdentityRecord struct {
	PasswordHash      string   `json:"passwordHash"`
	TOTPSecret        string   `json:"totpSecret,omitempty"`
	BackupCodes       []string `json:"backupCodes,omitempty"`
	PasswordChangedAt int64    `json:"passwordChangedAt"`
}

// LocalUserStore manages the storage and verification of passwords for "Local" users.
type LocalUserStore struct {
	kv kvstore.KVStore
}

func NewLocalUserStore(kv kvstore.KVStore) *LocalUserStore {
	return &LocalUserStore{kv: kv}
}

// CreateTemporaryUser creates a new local identity record with a strict TTL.
// This is strictly used for onboarding and dual-write flows.
func (s *LocalUserStore) CreateTemporaryUser(ctx context.Context, userID, rawPassword string, ttl time.Duration) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	record := LocalIdentityRecord{
		PasswordHash:      string(hash),
		PasswordChangedAt: time.Now().Unix(),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	key := kvstore.Key{Namespace: kvstore.NSAuthLocal, ID: userID}

	return s.kv.WriteTx(ctx, func(tx kvstore.Tx) error {
		opts := []kvstore.SetOption{}
		if ttl > 0 {
			opts = append(opts, kvstore.WithTTL(ttl))
		}
		return tx.Set(key, data, opts...)
	})
}

// CreateUser creates a permanent local identity record.
// This is typically used when an admin directly invites a user.
func (s *LocalUserStore) CreateUser(ctx context.Context, userID, rawPassword string) error {
	return s.CreateTemporaryUser(ctx, userID, rawPassword, 0)
}

// FinalizeTemporaryUser removes the TTL from a pre-provisioned user record, making it permanent.
func (s *LocalUserStore) FinalizeTemporaryUser(ctx context.Context, userID string) error {
	key := kvstore.Key{Namespace: kvstore.NSAuthLocal, ID: userID}

	return s.kv.WriteTx(ctx, func(tx kvstore.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return err
		}
		// Write the exact same data back, but without any TTL options!
		return tx.Set(key, val)
	})
}

// SetPassword handles password resets. It preserves existing 2FA/TOTP settings.
func (s *LocalUserStore) SetPassword(ctx context.Context, userID, rawPassword string) error {
	key := kvstore.Key{Namespace: kvstore.NSAuthLocal, ID: userID}

	return s.kv.WriteTx(ctx, func(tx kvstore.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return fmt.Errorf("user record not found: %w", err)
		}

		var record LocalIdentityRecord
		if err := json.Unmarshal(val, &record); err != nil {
			return err
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		record.PasswordHash = string(hash)
		record.PasswordChangedAt = time.Now().Unix()

		data, err := json.Marshal(record)
		if err != nil {
			return err
		}

		return tx.Set(key, data)
	})
}

// VerifyPassword checks if a plaintext password matches the hash in the KV Store.
func (s *LocalUserStore) VerifyPassword(ctx context.Context, userID, rawPassword string) (bool, error) {
	key := kvstore.Key{Namespace: kvstore.NSAuthLocal, ID: userID}

	var data []byte
	err := s.kv.ReadTx(ctx, func(tx kvstore.Tx) error {
		val, err := tx.Get(key)
		if err != nil {
			return err
		}
		data = val
		return nil
	})
	if err != nil {
		return false, err // e.g., kvstore.ErrKeyNotFound
	}

	var record LocalIdentityRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return false, fmt.Errorf("corrupt local identity record: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(rawPassword))
	if err != nil {
		return false, nil // Invalid password
	}

	return true, nil
}
