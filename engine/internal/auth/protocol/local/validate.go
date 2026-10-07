package local

import (
	"fmt"
	"net/mail"

	"golang.org/x/crypto/bcrypt"

	"platrium/internal/apperr"
)

const (
	// MinPasswordLen and MaxPasswordLen bound a password. bcrypt ignores (or
	// rejects) anything past 72 bytes, so a longer one would silently weaken.
	MinPasswordLen = 8
	MaxPasswordLen = 72
	maxEmailLen    = 255
)

// NormalizeEmail returns the form a local user's sign-in name is stored in, or
// ErrInvalid if it is not a plain email address. Every path that creates a
// local user, the setup flow included, goes through it.
func NormalizeEmail(s string) (string, error) {
	s = NormalizeLogin(s)
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > maxEmailLen {
		return "", fmt.Errorf("%w: %q is not a valid email address", apperr.ErrInvalid, s)
	}
	return s, nil
}

// ValidatePassword returns ErrInvalid if a password is not an acceptable length.
func ValidatePassword(s string) error {
	if len(s) < MinPasswordLen || len(s) > MaxPasswordLen {
		return fmt.Errorf("%w: a password must be %d to %d characters", apperr.ErrInvalid, MinPasswordLen, MaxPasswordLen)
	}
	return nil
}

// dummyHash is a valid bcrypt hash of nothing anyone knows, built once.
var dummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("platrium-no-such-user"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
}()

// BurnVerify spends the time a real password check takes and discards the
// result. Sign-in calls it when there is no such user, so that "no such user"
// and "wrong password" take about as long and a timer cannot tell which emails
// have accounts.
func BurnVerify(rawPassword string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(rawPassword))
}
