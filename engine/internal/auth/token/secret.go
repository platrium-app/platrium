// Package token issues and validates the bearer credentials held by native
// clients (registered devices) and apps (CLI, FUSE, scripts).
//
// A token is a high-entropy opaque secret. Only its SHA-256 is stored, and
// revoking a token deletes its row. Tokens that belong to a device carry the
// device with them: deleting either deletes both.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// Prefix marks platrium bearer tokens so they are recognisable in logs and by
// secret scanners.
const Prefix = "plt_"

const prefixDisplayLen = len(Prefix) + 4

// Generate returns a new random secret and the values to persist for it.
func Generate() (secret, hash, display string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", err
	}
	secret = Prefix + base64.RawURLEncoding.EncodeToString(b)
	return secret, Hash(secret), secret[:prefixDisplayLen], nil
}

// Hash returns the hex SHA-256 of a secret. Secrets are 256 random bits, so a
// fast unsalted hash is sufficient and keeps per-request lookups cheap.
func Hash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// LooksLikeToken reports whether a bearer value has the platrium token shape.
func LooksLikeToken(v string) bool { return strings.HasPrefix(v, Prefix) }
