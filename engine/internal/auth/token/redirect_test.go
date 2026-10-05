package token_test

import (
	"testing"

	"platrium/internal/auth/token"
)

func TestParseRedirect(t *testing.T) {
	allow := []string{"https://app.example.com/auth/"}
	ok := []string{
		"platrium://callback",
		"org.platrium.app:/cb",
		"http://127.0.0.1:53124/callback",
		"http://localhost:8080/cb",
		"http://[::1]:9000/cb",
		"https://app.example.com/auth/callback",
	}
	bad := []string{
		"", "/relative", "javascript:alert(1)", "data:text/html,x", "file:///etc/passwd",
		"http://evil.com/cb", "http://127.0.0.1.evil.com/cb", "https://evil.com/auth/",
		"https://app.example.com/other", "platrium://u:p@host/cb", "platrium://cb#frag",
	}
	for _, r := range ok {
		if _, err := token.ParseRedirect(r, allow); err != nil {
			t.Errorf("ParseRedirect(%q) rejected: %v", r, err)
		}
	}
	for _, r := range bad {
		if _, err := token.ParseRedirect(r, allow); err == nil {
			t.Errorf("ParseRedirect(%q) accepted", r)
		}
	}
}
