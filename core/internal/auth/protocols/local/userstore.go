package local

import (
	"context"
)

// LocalUserStore manages the storage and verification of passwords for "Local" users.
// While the GraphDB stores the User node for Authorization, this KV store holds 
// the actual Argon2/Bcrypt hash required for Authentication.
type LocalUserStore struct {
	// kvStore kv.Store (e.g. BadgerDB instance)
}

func NewLocalUserStore() *LocalUserStore {
	return &LocalUserStore{}
}

// SetPassword hashes a raw plaintext password and securely writes it to the KV Store.
func (s *LocalUserStore) SetPassword(ctx context.Context, userID, rawPassword string) error {
	// 1. Hash password using bcrypt/argon2
	// 2. Write {"hash": "$argon2id$v=19$..."} to KV store at key: "auth:local:{userID}"
	return nil
}

// VerifyPassword checks if a plaintext password matches the hash in the KV Store.
func (s *LocalUserStore) VerifyPassword(ctx context.Context, userID, rawPassword string) (bool, error) {
	// 1. Get hash from KV store using key "auth:local:{userID}"
	// 2. return bcrypt.CompareHashAndPassword()
	return false, nil
}
