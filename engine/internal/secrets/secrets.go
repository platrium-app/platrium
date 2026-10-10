// Package secrets is the engine's one source of key material. A single
// operator-supplied PLATRIUM_SECRET_KEY, identical on every replica, is
// expanded into independent per-purpose keys, so HTTP signing and at-rest
// encryption never share raw key bytes.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Env is the operator-facing configuration. There is deliberately no default:
// a built-in key would make everything sealed under it effectively plaintext.
type Env struct {
	Key string `env:"PLATRIUM_SECRET_KEY,required,notEmpty"`
}

// Purpose names what a derived key is for. Each purpose gets an unrelated key.
type Purpose string

const (
	// PurposeHTTPSigning keys upload/download passports and chunk receipts.
	PurposeHTTPSigning Purpose = "http-signing"
	// PurposeAtRest keys the encryption of secrets stored in the database.
	PurposeAtRest Purpose = "at-rest"
	// PurposeAuthFlow keys the sealed cookies that carry a sign-in in progress.
	PurposeAuthFlow Purpose = "auth-flow"
)

// sealPrefix marks a sealed value and its format version, so the format (or
// the key) can change later while old values stay readable.
const sealPrefix = "enc:v1:"

// ErrNotSealed means a value does not carry the sealed-value prefix.
var ErrNotSealed = errors.New("secrets: value is not sealed")

// Keyring derives purpose keys from the master key.
type Keyring struct {
	master []byte
}

// New builds a keyring from a master key.
func New(master string) *Keyring { return &Keyring{master: []byte(master)} }

// FromEnv builds the keyring from PLATRIUM_SECRET_KEY, which every instance of
// a cluster must share. Like the other env readers it panics when the
// environment is incomplete.
func FromEnv() *Keyring {
	var cfg Env
	if err := env.Parse(&cfg); err != nil {
		panic(fmt.Sprintf("failed to parse secrets env: %v", err))
	}
	return New(cfg.Key)
}

func (k *Keyring) derive(p Purpose) []byte {
	key, err := hkdf.Key(sha256.New, k.master, nil, "platrium/"+string(p), 32)
	if err != nil { // only possible for an absurd output length
		panic(fmt.Sprintf("secrets: derive %s: %v", p, err))
	}
	return key
}

// SigningSecret is the key for HMAC signing, hex encoded.
func (k *Keyring) SigningSecret() string {
	return hex.EncodeToString(k.derive(PurposeHTTPSigning))
}

// Sealer encrypts and authenticates values under the key of one purpose, so a
// value sealed for one purpose never opens for another.
type Sealer struct {
	gcm cipher.AEAD
}

// Sealer returns the sealer for a purpose.
func (k *Keyring) Sealer(p Purpose) *Sealer {
	block, err := aes.NewCipher(k.derive(p))
	if err != nil {
		panic(fmt.Sprintf("secrets: cipher: %v", err))
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(fmt.Sprintf("secrets: gcm: %v", err))
	}
	return &Sealer{gcm: gcm}
}

// Seal encrypts plaintext for storage in the database. context is authenticated
// but not stored (use the owning row's ID): a sealed value copied onto another
// row will not open there.
func (k *Keyring) Seal(plaintext, context string) (string, error) {
	return k.Sealer(PurposeAtRest).Seal(plaintext, context)
}

// Open decrypts a value produced by Seal with the same context.
func (k *Keyring) Open(sealed, context string) (string, error) {
	return k.Sealer(PurposeAtRest).Open(sealed, context)
}

// Seal encrypts plaintext. context is authenticated but not stored.
func (s *Sealer) Seal(plaintext, context string) (string, error) {
	gcm := s.gcm
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	out := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(context))
	return sealPrefix + base64.RawURLEncoding.EncodeToString(out), nil
}

// Open decrypts a value produced by Seal with the same context.
func (s *Sealer) Open(sealed, context string) (string, error) {
	body, ok := strings.CutPrefix(sealed, sealPrefix)
	if !ok {
		return "", ErrNotSealed
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	gcm := s.gcm
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", errors.New("secrets: malformed sealed value")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(context))
	if err != nil {
		return "", errors.New("secrets: cannot open sealed value (wrong key or context)")
	}
	return string(plain), nil
}
