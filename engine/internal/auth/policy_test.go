package auth_test

import (
	"errors"
	"testing"

	"platrium/internal/apperr"
	"platrium/internal/auth"
)

func TestValidateJITRole(t *testing.T) {
	if err := auth.ValidateJITRole("MEMBER"); err != nil {
		t.Errorf("MEMBER: %v", err)
	}
	for _, role := range []string{"ADMIN", "SUPER_ADMIN", "", "nope"} {
		if err := auth.ValidateJITRole(role); !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", role, err)
		}
	}
}

func TestPolicyNormalizeDomains(t *testing.T) {
	p, err := auth.ProvisioningPolicy{DefaultRole: "MEMBER", AllowedEmailDomains: []string{" Acme.COM ", "acme.com", "corp.acme.com"}}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.AllowedEmailDomains) != 2 || p.AllowedEmailDomains[0] != "acme.com" || p.AllowedEmailDomains[1] != "corp.acme.com" {
		t.Errorf("domains = %v", p.AllowedEmailDomains)
	}
	for _, bad := range []string{"", "acme", "a@acme.com", "acme.com/x"} {
		if _, err := (auth.ProvisioningPolicy{DefaultRole: "MEMBER", AllowedEmailDomains: []string{bad}}).Normalize(); err == nil {
			t.Errorf("domain %q accepted", bad)
		}
	}
}

func TestPolicyAllowsEmail(t *testing.T) {
	open := auth.ProvisioningPolicy{}
	if !open.AllowsEmail("anyone@anywhere.org") {
		t.Error("an unrestricted policy refused an email")
	}
	p := auth.ProvisioningPolicy{AllowedEmailDomains: []string{"acme.com"}}
	for email, want := range map[string]bool{
		"a@acme.com": true, "a@ACME.com": true, "a@evil.com": false,
		"a@acme.com.evil.com": false, "a@sub.acme.com": false, "no-at-sign": false,
	} {
		if got := p.AllowsEmail(email); got != want {
			t.Errorf("AllowsEmail(%q) = %v, want %v", email, got, want)
		}
	}
}
