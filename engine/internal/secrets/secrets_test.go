package secrets

import (
	"errors"
	"strings"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	k := New("k1")
	sealed, err := k.Seal("s3cret", "idp_1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, sealPrefix) || strings.Contains(sealed, "s3cret") {
		t.Fatalf("unexpected sealed form %q", sealed)
	}
	got, err := k.Open(sealed, "idp_1")
	if err != nil || got != "s3cret" {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestSealIsRandomized(t *testing.T) {
	k := New("k1")
	a, _ := k.Seal("x", "c")
	b, _ := k.Seal("x", "c")
	if a == b {
		t.Fatal("two seals of the same value must differ")
	}
}

func TestOpenRejects(t *testing.T) {
	k := New("k1")
	sealed, _ := k.Seal("x", "idp_1")

	if _, err := k.Open(sealed, "idp_2"); err == nil {
		t.Error("opened under a different context")
	}
	if _, err := New("k2").Open(sealed, "idp_1"); err == nil {
		t.Error("opened under a different key")
	}
	if _, err := k.Open("plaintext", "idp_1"); !errors.Is(err, ErrNotSealed) {
		t.Errorf("plaintext: err = %v, want ErrNotSealed", err)
	}
	if _, err := k.Open(sealPrefix+"!!", "idp_1"); err == nil {
		t.Error("opened malformed value")
	}
}

func TestSealersOfDifferentPurposesDoNotInterchange(t *testing.T) {
	k := New("k1")
	sealed, err := k.Sealer(PurposeAuthFlow).Seal("flow", "state-1")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := k.Sealer(PurposeAuthFlow).Open(sealed, "state-1"); err != nil || got != "flow" {
		t.Fatalf("Open = %q, %v", got, err)
	}
	if _, err := k.Sealer(PurposeAtRest).Open(sealed, "state-1"); err == nil {
		t.Error("a value sealed for the sign-in flow opened as an at-rest secret")
	}
	if _, err := k.Open(sealed, "state-1"); err == nil {
		t.Error("Keyring.Open (at rest) accepted a sign-in flow value")
	}
}

func TestPurposesAreIndependent(t *testing.T) {
	k := New("k1")
	if k.SigningSecret() == "" || k.SigningSecret() != New("k1").SigningSecret() {
		t.Fatal("signing secret must be stable for the same master key")
	}
	if k.SigningSecret() == New("k2").SigningSecret() {
		t.Fatal("different master keys must give different signing secrets")
	}
	if string(k.derive(PurposeHTTPSigning)) == string(k.derive(PurposeAtRest)) {
		t.Fatal("purposes must not share a key")
	}
}

func TestFromEnvRequiresAKey(t *testing.T) {
	t.Setenv("PLATRIUM_SECRET_KEY", "")
	func() {
		defer func() {
			if recover() == nil {
				t.Error("FromEnv accepted an empty PLATRIUM_SECRET_KEY")
			}
		}()
		FromEnv()
	}()

	t.Setenv("PLATRIUM_SECRET_KEY", "a-real-key")
	if FromEnv().SigningSecret() != New("a-real-key").SigningSecret() {
		t.Error("FromEnv did not use PLATRIUM_SECRET_KEY")
	}
}
