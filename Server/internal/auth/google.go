package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type GoogleLogin struct {
	oauth          oauth2.Config
	verifier       *oidc.IDTokenVerifier
	repo           Repository
	allowedDomains string
	sessionTTL     time.Duration
}

func NewLogin(ctx context.Context, cfg *config.Config, repo Repository) (*GoogleLogin, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}

	return &GoogleLogin{
		oauth: oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleSecret,
			RedirectURL:  cfg.GoogleCallback,
			Scopes: []string{
				oidc.ScopeOpenID,
				oidc.ScopeEmail,
				oidc.ScopeProfile,
			},
		},
		verifier:       provider.Verifier(&oidc.Config{ClientID: cfg.GoogleClientID}),
		repo:           repo,
		allowedDomains: cfg.GoogleAllowedDomain,
		sessionTTL:     4 * time.Hour,
	}, nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (g *GoogleLogin) Start(ctx context.Context) (authURL, bowserSecret string, err error) {
	state, err := randomToken()
	if err != nil {
		return "", "", err
	}

	browserSecret, err := randomToken()
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

	oauthToken, err := g.oauth.Exchange(
		ctx,
		code,
		oauth2.VerifierOption(flow.Verifier),
	)
	if err != nil {
		return "", ErrAccessDenied
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return "", ErrAccessDenied
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil || idToken.Nonce != flow.Nonce {
		return "", ErrAccessDenied
	}

	var claims struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"email_verified"`
		HostedDomain  string `json:"hd"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return "", ErrAccessDenied
	}
	if !claims.VerifiedEmail || strings.EqualFold(claims.HostedDomain, g.allowedDomains) {
		return "", ErrAccessDenied
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
	if len(sessionToken) != 43 {
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
