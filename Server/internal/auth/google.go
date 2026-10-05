package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// sessionTokenLength is the character length of a randomToken: 32 random bytes
// encoded with unpadded base64url. Session lookups reject anything else before
// touching the database.
const sessionTokenLength = 43

type GoogleLogin struct {
	oauth         oauth2.Config
	verifier      *oidc.IDTokenVerifier
	repo          Repository
	allowedDomain string
	sessionTTL    time.Duration
}

// NewGoogleLogin performs OIDC discovery against Google, so a server that
// cannot reach Google's configuration fails visibly at startup.
func NewGoogleLogin(ctx context.Context, cfg *config.Config, repo Repository) (*GoogleLogin, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}

	return &GoogleLogin{
		oauth: oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleSecret,
			RedirectURL:  cfg.GoogleCallback,
			// Discovery supplies Google's authorize and token URLs. Without
			// this, AuthCodeURL returns a bare query string and Exchange has
			// no endpoint to post to.
			Endpoint: provider.Endpoint(),
			Scopes: []string{
				oidc.ScopeOpenID,
				oidc.ScopeEmail,
				oidc.ScopeProfile,
			},
		},
		verifier:      provider.Verifier(&oidc.Config{ClientID: cfg.GoogleClientID}),
		repo:          repo,
		allowedDomain: cfg.GoogleAllowedDomain,
		sessionTTL:    4 * time.Hour,
	}, nil
}

// SessionTTL lets the HTTP layer give the session cookie the same lifetime as
// the session row, so the browser stops sending a token the server has expired.
func (g *GoogleLogin) SessionTTL() time.Duration {
	return g.sessionTTL
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (g *GoogleLogin) Start(ctx context.Context) (authURL, browserSecret string, err error) {
	state, err := randomToken()
	if err != nil {
		return "", "", err
	}

	browserSecret, err = randomToken()
	if err != nil {
		return "", "", err
	}

	nonce, err := randomToken()
	if err != nil {
		return "", "", err
	}

	pkceVerifier := oauth2.GenerateVerifier()

	flow := LoginFlow{
		StateHash:   sha256.Sum256([]byte(state)),
		BrowserHash: sha256.Sum256([]byte(browserSecret)),
		Nonce:       nonce,
		Verifier:    pkceVerifier,
		ExpiresAt:   time.Now().UTC().Add(5 * time.Minute),
	}

	if err := g.repo.SaveFlow(ctx, flow); err != nil {
		return "", "", err
	}

	authURL = g.oauth.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(pkceVerifier),
		oauth2.SetAuthURLParam("nonce", nonce),
	)

	return authURL, browserSecret, nil
}

func (g *GoogleLogin) Complete(
	ctx context.Context,
	state, browserSecret, code string,
) (string, error) {
	if state == "" || browserSecret == "" || code == "" {
		return "", ErrInvalidFlow
	}

	flow, err := g.repo.ConsumeFlow(
		ctx,
		sha256.Sum256([]byte(state)),
		sha256.Sum256([]byte(browserSecret)),
	)
	if err != nil {
		return "", err
	}

	// Refusals below wrap ErrAccessDenied so that errors.Is still classifies
	// them as a refusal, while the message carries enough detail for the
	// server log. The HTTP layer never shows these details to the browser.
	oauthToken, err := g.oauth.Exchange(
		ctx,
		code,
		oauth2.VerifierOption(flow.Verifier),
	)
	if err != nil {
		return "", fmt.Errorf("%w: code exchange failed: %v", ErrAccessDenied, err)
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return "", fmt.Errorf("%w: response carried no id_token", ErrAccessDenied)
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", fmt.Errorf("%w: id_token verification failed: %v", ErrAccessDenied, err)
	}
	if idToken.Nonce != flow.Nonce {
		return "", fmt.Errorf("%w: id_token nonce did not match the login flow", ErrAccessDenied)
	}

	var claims struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"email_verified"`
		HostedDomain  string `json:"hd"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return "", ErrAccessDenied
	}
	if !claims.VerifiedEmail {
		return "", fmt.Errorf("%w: Google reports the email is unverified", ErrAccessDenied)
	}
	// Google sends "hd" only for Workspace accounts, so a personal account
	// arrives with an empty hosted domain and is refused here.
	if !strings.EqualFold(claims.HostedDomain, g.allowedDomain) {
		return "", fmt.Errorf(
			"%w: hosted domain %q is not the allowed domain %q",
			ErrAccessDenied, claims.HostedDomain, g.allowedDomain,
		)
	}

	user, err := g.repo.FindOrBindUser(
		ctx,
		idToken.Subject,
		claims.Email,
	)
	if err != nil {
		return "", err
	}

	sessionToken, err := randomToken()
	if err != nil {
		return "", err
	}
	sessionTokenHash := sha256.Sum256([]byte(sessionToken))

	if err := g.repo.CreateSession(
		ctx,
		user.ID,
		sessionTokenHash,
		time.Now().UTC().Add(g.sessionTTL),
	); err != nil {
		return "", err
	}

	return sessionToken, nil
}

func (g *GoogleLogin) CurrentUser(
	ctx context.Context,
	sessionToken string,
) (User, error) {
	if len(sessionToken) != sessionTokenLength {
		return User{}, ErrUnauthenticated
	}

	return g.repo.ResolveSession(
		ctx,
		sha256.Sum256([]byte(sessionToken)),
	)
}

func (g *GoogleLogin) SignOut(
	ctx context.Context,
	sessionToken string,
) error {
	if sessionToken == "" {
		return ErrUnauthenticated
	}

	err := g.repo.RevokeSession(
		ctx,
		sha256.Sum256([]byte(sessionToken)),
	)

	if errors.Is(err, ErrUnauthenticated) {
		return nil
	}

	return err
}
