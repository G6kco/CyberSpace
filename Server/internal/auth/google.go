package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type GoogleLogin struct {
	oauth oauth2.Config
	verifier *oidc.IDTokenVerifier
	repo Repository
	allowedDomains string
	sessionTTL time.Duration
}

func NewLogin(ctx context.Context, cfg *config.Config, repo Repository) (*GoogleLogin, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}
	
	return &GoogleLogin{
		oauth: oauth2.Config{
			ClientID: cfg.GoogleClientID,
			ClientSecret: cfg.GoogleSecret,
			RedirectURL: cfg.GoogleCallback,
			Scopes: []string{
				oidc.ScopeOpenID,
				oidc.ScopeEmail,
				oidc.ScopeProfile,
			},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.GoogleClientID}),
		repo: repo,
		allowedDomains: cfg.GoogleAllowedDomain,
		sessionTTL: 4 * time.Hour,
	}, nil
}

func randomToken() (string, error){
	value := make([]byte, 32)
	if _ , err := rand.Read(value); err != nil {
		return "", err
	}
	
	return base64.RawURLEncoding.EncodeToString(value), nil
}