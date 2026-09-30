package oidc

// OIDCClient encapsulates the cryptographic and protocol logic needed to securely 
// interact with upstream identity providers (like Google Workspace, Okta, or Azure AD).
type OIDCClient struct {
	// ... HTTP Client, OAuth2 Config, etc.
}

// Exchange handles parsing the OAuth code from the callback URL, exchanging it 
// with the upstream IdP for an ID Token, verifying the JWT signature, and 
// normalizing the claims into a standard format.
func (c *OIDCClient) Exchange(code string) error {
	// 1. Exchange code for Token
	// 2. Verify JWT signature against JWKS
	// 3. Extract standard claims (sub, email, picture)
	return nil
}
