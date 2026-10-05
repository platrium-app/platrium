package token_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"platrium/internal/auth/token"
	"platrium/internal/infra/kvstore"
)

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" // RFC 7636 example

func newCodes(t *testing.T) *token.CodeStore {
	t.Helper()
	kv, err := kvstore.NewInMemoryStore()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kv.Close() })
	return token.NewCodeStore(kv)
}

func TestCodeRoundTripIsSingleUse(t *testing.T) {
	ctx := context.Background()
	codes := newCodes(t)
	code, err := codes.Create(ctx, token.Grant{TenantID: "t", UserID: "u", Name: "iPhone", Platform: "IOS", Challenge: token.Challenge(verifier)})
	if err != nil {
		t.Fatal(err)
	}
	g, err := codes.Redeem(ctx, code, verifier)
	if err != nil || g.UserID != "u" || g.Platform != "IOS" || g.Name != "iPhone" {
		t.Fatalf("Redeem = %+v, %v", g, err)
	}
	if _, err := codes.Redeem(ctx, code, verifier); !errors.Is(err, token.ErrInvalidCode) {
		t.Fatalf("code reused: %v", err)
	}
}

func TestWrongVerifierBurnsCode(t *testing.T) {
	ctx := context.Background()
	codes := newCodes(t)
	code, _ := codes.Create(ctx, token.Grant{TenantID: "t", UserID: "u", Name: "x", Challenge: token.Challenge(verifier)})

	if _, err := codes.Redeem(ctx, code, strings.Repeat("a", 43)); !errors.Is(err, token.ErrInvalidCode) {
		t.Fatalf("wrong verifier accepted: %v", err)
	}
	if _, err := codes.Redeem(ctx, code, verifier); !errors.Is(err, token.ErrInvalidCode) {
		t.Fatalf("code survived a failed attempt: %v", err)
	}
}

func TestUnknownCode(t *testing.T) {
	if _, err := newCodes(t).Redeem(context.Background(), "nope", verifier); !errors.Is(err, token.ErrInvalidCode) {
		t.Fatalf("err = %v", err)
	}
}

func TestPKCE(t *testing.T) {
	if got := token.Challenge(verifier); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("challenge = %s (RFC 7636 vector)", got)
	}
	if token.VerifyPKCE("", verifier) || token.VerifyPKCE(token.Challenge(verifier), "short") {
		t.Fatal("accepted degenerate input")
	}
}
