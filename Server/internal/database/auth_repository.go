package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/G6kco/CyberSpace/internal/auth"
)

type AuthRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) SaveFlow(
	ctx context.Context,
	flow auth.LoginFlow,
) error {
	_, err := r.db.ExecContext(ctx,
		`insert into oauth_login_flows 
		(state_hash,browser_hash,nonce,pkce_verifier,expires_at,created_at)
		values (?,?,?,?,?,UTC_TIMESTAMP(6));`,
		flow.StateHash[:],
		flow.BrowserHash[:],
		flow.Nonce,
		flow.Verifier,
		flow.ExpiresAt,
	)
	return err
}

func (r *AuthRepository) ConsumeFlow(
	ctx context.Context,
	stateHash, browserHash [32]byte, 
) (auth.LoginFlow, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return auth.LoginFlow{}, err
	}
	defer tx.Rollback()
	
	var flow auth.LoginFlow
	
	err = tx.QueryRowContext(
		ctx,
		`SELECT nonce, pkce_verifier
        FROM oauth_login_flows
        WHERE state_hash = ?
          AND browser_hash = ?
          AND consumed_at IS NULL
          AND expires_at > UTC_TIMESTAMP(6)
        FOR UPDATE`,
		stateHash[:],
		browserHash[:],
	).Scan(
		&flow.Nonce,
		&flow.Verifier,
	)
	
	if errors.Is(err, sql.ErrNoRows){
		return auth.LoginFlow{}, auth.ErrInvalidFlow
	}
	if err != nil{
		return auth.LoginFlow{}, err
	}
	
	_, err = tx.ExecContext(
		ctx,
		`UPDATE oauth_login_flows
        SET consumed_at = UTC_TIMESTAMP(6)
        WHERE state_hash = ?`,
		stateHash[:],
	)
	if err  != nil {
		return auth.LoginFlow{}, err
	}
	if err := tx.Commit(); err != nil{
		return auth.LoginFlow{}, err
	}
	
	return flow, nil
}