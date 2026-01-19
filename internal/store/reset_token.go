package store

import (
	"context"
	"database/sql"
	"time"
)

type PostgresResetTokenStore struct {
	db *sql.DB
}

func NewPostgresResetTokenStore (db *sql.DB) *PostgresResetTokenStore {
	return &PostgresResetTokenStore{
		db: db,
	}
}

type ResetToken struct {
	Id int64	`json:"id"`
	UserId int64 `json:"user_id"`
	TokenHash []byte `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	IsUsed bool `json:"is_used"`
}

type ResetTokenStore interface {
	CreateResetToken(ctx context.Context, token *ResetToken) error
	UseResetToken(ctx context.Context, tokenhash []byte) (int64, error)
}

func (pg *PostgresResetTokenStore) CreateResetToken(ctx context.Context, token *ResetToken) error {
	query := `INSERT INTO reset_tokens(user_id, token_hash, expires_at, is_used)
	VALUES($1, $2, $3, FALSE)
	RETURNING id, is_used`

	err := pg.db.QueryRowContext(ctx, query, token.UserId, token.TokenHash, token.ExpiresAt).Scan(&token.Id, &token.IsUsed)
	if err != nil {
		return err
	}

	return nil
}

func (pg *PostgresResetTokenStore) UseResetToken(ctx context.Context, tokenhash []byte) (int64, error) {
	query := `
	UPDATE reset_tokens
	SET is_used = TRUE
	WHERE token_hash = $1
		AND is_used = FALSE
		AND expires_at > NOW()
		RETURNING user_id`

	var userId int64 

	err := pg.db.QueryRowContext(ctx, query, tokenhash).Scan(&userId)
	if err != nil {
		return 0, err
	}

	return userId, nil
}