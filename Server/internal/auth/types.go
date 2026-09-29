package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidFlow     error = errors.New("Invalid or expired login flow")
	ErrAccessDenied    error = errors.New("account is not allowed")
	ErrUnauthenticated error = errors.New("session is not valid")
)

type LoginFlow struct {
	StateHash   [32]byte
	BrowserHash [32]byte
	Nonce       string
	Verifier    string
	ExpiresAt   time.Time
}

type User struct {
	ID       uint64 `json:"-"`
	PublicID string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type Repository interface {
	SaveFlow(ctx context.Context, flow LoginFlow) error
	ConsumeFlow(ctx context.Context, stateHash, browserHash [32]byte) (LoginFlow, error)
	FindOrBindUser(ctx context.Context, googleSubject, varifiedEmail string) (User, error)
	CreateSession(ctx context.Context, userID uint64, tokenHash [32]byte, expiresAt time.Time) error
	ResolveSession(ctx context.Context, tokenHash [32]byte) (User, error)
	RevokeSession(ctx context.Context, tokenHash [32]byte) error
}
