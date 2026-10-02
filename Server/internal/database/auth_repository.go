package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

func (r *AuthRepository) FindOrBindUser(
	ctx context.Context,
	subject, email string,
) (auth.User, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return auth.User{}, err
	}
	defer tx.Rollback()
	
	var user auth.User
	var status string
	var savedSubject sql.NullString
	
	scan := func(row *sql.Row) error {
		return row.Scan(
			&user.ID,
			&user.PublicID,
			&user.Name,
			&user.Email,
			&user.Role,
			&status,
			&savedSubject,
		)
	}
	
	err = scan(tx.QueryRowContext(
		ctx,
		` SELECT id, public_id, email, display_name, role,
               status, google_subject
        FROM users
        WHERE google_subject = ?
        FOR UPDATE`,
		subject,
	))
	
	if errors.Is(err, sql.ErrNoRows) {
		err = scan(tx.QueryRowContext(
			ctx,
			`SELECT id, public_id, email, display_name, role,
                   status, google_subject
            FROM users
            WHERE email = ?
            FOR UPDATE;`,
			email,
		))
	}
	
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, err
	}
	
	if status != "active" {
		return auth.User{}, auth.ErrAccessDenied
	}
	
	if savedSubject.Valid && savedSubject.String != subject {
		return auth.User{}, auth.ErrAccessDenied
	}
	
	if !savedSubject.Valid {
        _, err = tx.ExecContext(ctx, `
            UPDATE users
            SET google_subject = ?, last_login_at = UTC_TIMESTAMP(6)
            WHERE id = ? AND google_subject IS NULL
        `, subject, user.ID)
    } else {
        _, err = tx.ExecContext(ctx, `
            UPDATE users
            SET last_login_at = UTC_TIMESTAMP(6)
            WHERE id = ?
        `, user.ID)
    }
    if err != nil {
        return auth.User{}, err
    }
    if err := tx.Commit(); err != nil {
        return auth.User{}, err
    }
    return user, nil
}

func (r *AuthRepository) CreateSession(
	ctx context.Context,
	userID uint64,
	tokenhash [32]byte,
	expiresAt time.Time,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO auth_sessions
        (user_id, token_hash, last_seen_at,
        expires_at, created_at)
        VALUES (?, ?, UTC_TIMESTAMP(6), ?, UTC_TIMESTAMP(6))`,
		userID,
		tokenhash[:],
		expiresAt,
	)
	return err
}