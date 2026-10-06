// Package authtest provides test doubles for the Google sign-in flow. It is
// imported only by tests and is never linked into the server binary.
package authtest

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	ClientID     = "cyberspace-test.apps.googleusercontent.com"
	ClientSecret = "test-client-secret"
	RedirectURL  = "http://localhost:8080/api/v1/auth/google/callback"
	Issuer       = "https://accounts.google.com"
	AuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL     = "https://oauth2.googleapis.com/token"
	jwksURL      = "https://www.googleapis.com/oauth2/v3/certs"
	keyID        = "authtest-key"
)

// Account is the Google identity a fake authorization is issued for.
type Account struct {
	Subject       string
	Email         string
	EmailVerified bool
	HostedDomain  string

	// Mutate, when set, edits the ID token claims before they are signed.
	Mutate func(claims map[string]any)
	// OmitIDToken makes the token response carry no id_token.
	OmitIDToken bool
	// SignWithUnknownKey signs the ID token with a key absent from the JWKS.
	SignWithUnknownKey bool
}

type grant struct {
	account   Account
	challenge string
	nonce     string
	redirect  string
}

// Google is an in-process stand-in for Google's OIDC discovery, JWKS, and
// token endpoints. Its HTTP client is injected through the context, which is
// how oidc and oauth2 choose their client, so production code runs unchanged.
type Google struct {
	t          testing.TB
	key        *rsa.PrivateKey
	unknownKey *rsa.PrivateKey
	client     *http.Client

	mu     sync.Mutex
	grants map[string]grant
	issued int
}

func NewGoogle(t testing.TB) *Google {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	unknownKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate unknown key: %v", err)
	}

	g := &Google{
		t:          t,
		key:        key,
		unknownKey: unknownKey,
		grants:     map[string]grant{},
	}
	g.client = &http.Client{Transport: roundTripper(g.serve)}
	return g
}

// Context returns ctx carrying the fake's HTTP client. Use it both for
// auth.NewGoogleLogin (discovery and key fetching) and for Complete (code
// exchange).
func (g *Google) Context(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, g.client)
}

// Config returns the configuration auth.NewGoogleLogin reads.
func Config(allowedDomain string) *config.Config {
	return &config.Config{
		GoogleClientID:      ClientID,
		GoogleSecret:        ClientSecret,
		GoogleCallback:      RedirectURL,
		GoogleAllowedDomain: allowedDomain,
	}
}

// Authorize plays the browser's visit to Google: it validates the
// authorization URL produced by Start and returns the state and code Google
// would send to the callback.
func (g *Google) Authorize(authURL string, account Account) (state, code string) {
	g.t.Helper()

	parsed, err := url.Parse(authURL)
	if err != nil {
		g.t.Fatalf("parse authorization URL: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != AuthURL {
		g.t.Fatalf("authorization URL endpoint = %q, want %q", got, AuthURL)
	}

	query := parsed.Query()
	require := func(name, want string) {
		if got := query.Get(name); got != want {
			g.t.Fatalf("authorization URL %s = %q, want %q", name, got, want)
		}
	}
	require("client_id", ClientID)
	require("redirect_uri", RedirectURL)
	require("response_type", "code")
	require("code_challenge_method", "S256")
	for _, name := range []string{"state", "nonce", "code_challenge"} {
		if query.Get(name) == "" {
			g.t.Fatalf("authorization URL has no %s", name)
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	code = fmt.Sprintf("code-%d", g.issued)
	g.grants[code] = grant{
		account:   account,
		challenge: query.Get("code_challenge"),
		nonce:     query.Get("nonce"),
		redirect:  query.Get("redirect_uri"),
	}
	return query.Get("state"), code
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (g *Google) serve(r *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	switch r.URL.Scheme + "://" + r.URL.Host + r.URL.Path {
	case Issuer + "/.well-known/openid-configuration":
		writeJSON(recorder, http.StatusOK, map[string]any{
			"issuer":                                Issuer,
			"authorization_endpoint":                AuthURL,
			"token_endpoint":                        tokenURL,
			"jwks_uri":                              jwksURL,
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case jwksURL:
		writeJSON(recorder, http.StatusOK, map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": keyID,
				"n":   encode(g.key.N.Bytes()),
				"e":   encode(big.NewInt(int64(g.key.E)).Bytes()),
			}},
		})
	case tokenURL:
		g.token(recorder, r)
	default:
		// Anything else would be a real network call, which tests must not make.
		return nil, fmt.Errorf("authtest: unexpected request to %s", r.URL)
	}
	return recorder.Result(), nil
}

func (g *Google) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.Method != http.MethodPost {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	clientID, secret, ok := r.BasicAuth()
	if !ok {
		clientID, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if clientID != ClientID || secret != ClientSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}

	code := r.PostForm.Get("code")
	g.mu.Lock()
	issued, found := g.grants[code]
	verifierHash := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	valid := found &&
		r.PostForm.Get("grant_type") == "authorization_code" &&
		r.PostForm.Get("redirect_uri") == issued.redirect &&
		encode(verifierHash[:]) == issued.challenge
	if valid {
		// Google codes are single use.
		delete(g.grants, code)
	}
	g.mu.Unlock()

	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}

	response := map[string]any{
		"access_token": "access-" + code,
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
	if !issued.account.OmitIDToken {
		response["id_token"] = g.idToken(issued)
	}
	writeJSON(w, http.StatusOK, response)
}

func (g *Google) idToken(issued grant) string {
	now := time.Now()
	claims := map[string]any{
		"iss":            Issuer,
		"aud":            ClientID,
		"sub":            issued.account.Subject,
		"email":          issued.account.Email,
		"email_verified": issued.account.EmailVerified,
		"nonce":          issued.nonce,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
	// Google includes "hd" only for Workspace accounts.
	if issued.account.HostedDomain != "" {
		claims["hd"] = issued.account.HostedDomain
	}
	if issued.account.Mutate != nil {
		issued.account.Mutate(claims)
	}

	key := g.key
	if issued.account.SignWithUnknownKey {
		key = g.unknownKey
	}
	return g.sign(key, claims)
}

func (g *Google) sign(key *rsa.PrivateKey, claims map[string]any) string {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	if err != nil {
		g.t.Fatalf("encode token header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		g.t.Fatalf("encode token claims: %v", err)
	}

	signingInput := encode(header) + "." + encode(payload)
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		g.t.Fatalf("sign token: %v", err)
	}
	return signingInput + "." + encode(signature)
}

func encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// Unreachable returns ctx with an HTTP client that fails every request, as
// when Google cannot be reached.
func Unreachable(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, &http.Client{Transport: roundTripper(
		func(r *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("authtest: network unavailable for %s", r.URL.Host)
		},
	)})
}

// HasScope reports whether a space-separated scope list contains scope.
func HasScope(scopes, scope string) bool {
	return slices.Contains(strings.Fields(scopes), scope)
}
