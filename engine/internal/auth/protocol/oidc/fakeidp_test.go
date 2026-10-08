package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeIdP is a minimal but real OpenID provider: discovery, signing keys, and a
// token endpoint that enforces the client secret and PKCE, issuing RSA-signed
// ID tokens. It lets the flow run end to end without a network.
type fakeIdP struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	clientID, clientSecret string

	mu    sync.Mutex
	codes map[string]issuedCode

	// idClaims builds the ID token claims for an issued code. Tests override
	// individual fields through tweak.
	tweak func(claims map[string]any)
	// userinfo, when set, is served at /userinfo.
	userinfo map[string]any
	// reportedSuffix is appended to the issuer the provider reports about
	// itself ("/" mimics Auth0) while it is still reached without it.
	reportedSuffix string
}

type issuedCode struct {
	challenge, nonce, redirectURI string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{t: t, key: key, clientID: "platrium-client", clientSecret: "client-secret", codes: map[string]issuedCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("/jwks", f.jwks)
	mux.HandleFunc("/token", f.token)
	mux.HandleFunc("/userinfo", f.userinfoHandler)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) issuer() string { return f.srv.URL }

// reportedIssuer is how the provider names itself in discovery and in tokens.
func (f *fakeIdP) reportedIssuer() string { return f.srv.URL + f.reportedSuffix }

func (f *fakeIdP) discovery(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                f.reportedIssuer(),
		"authorization_endpoint":                f.issuer() + "/authorize",
		"token_endpoint":                        f.issuer() + "/token",
		"jwks_uri":                              f.issuer() + "/jwks",
		"userinfo_endpoint":                     f.issuer() + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeIdP) jwks(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

func (f *fakeIdP) userinfoHandler(w http.ResponseWriter, r *http.Request) {
	if f.userinfo == nil {
		http.NotFound(w, r)
		return
	}
	json.NewEncoder(w).Encode(f.userinfo)
}

// authorize plays the part of the user approving the request: it takes the
// URL Platrium sent the browser to and returns the code and state the provider
// would redirect back with.
func (f *fakeIdP) authorize(authURL string) (code, state string) {
	f.t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || q.Get("client_id") != f.clientID || q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge") == "" {
		f.t.Fatalf("bad authorization request: %s", authURL)
	}
	code = "code-" + q.Get("state")[:8]
	f.mu.Lock()
	f.codes[code] = issuedCode{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirectURI: q.Get("redirect_uri")}
	f.mu.Unlock()
	return code, q.Get("state")
}

func (f *fakeIdP) token(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": msg})
	}
	r.ParseForm()
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	} else if u, err := url.QueryUnescape(id); err == nil {
		id = u
	}
	if id != f.clientID || secret != f.clientSecret {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
		return
	}
	f.mu.Lock()
	c, ok := f.codes[r.PostForm.Get("code")]
	f.mu.Unlock()
	if !ok {
		fail("unknown code")
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != c.challenge {
		fail("PKCE verification failed")
		return
	}
	if r.PostForm.Get("redirect_uri") != c.redirectURI {
		fail("redirect_uri mismatch")
		return
	}

	f.mu.Lock()
	delete(f.codes, r.PostForm.Get("code")) // single use, once the request is valid
	f.mu.Unlock()

	claims := map[string]any{
		"iss": f.reportedIssuer(), "sub": "user-1", "aud": f.clientID, "nonce": c.nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"email": "alice@acme.com", "email_verified": true, "name": "Alice",
		"picture": "https://img.example/alice.png",
	}
	if f.tweak != nil {
		f.tweak(claims)
	}
	json.NewEncoder(w).Encode(map[string]any{
		"access_token": "at", "token_type": "Bearer", "expires_in": 3600,
		"id_token": f.sign(claims),
	})
}

func (f *fakeIdP) sign(claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		f.t.Fatal(err)
	}
	return raw
}
