package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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