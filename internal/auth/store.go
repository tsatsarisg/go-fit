package auth

import (
	"context"
	"database/sql"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

type Store interface {
	Insert(ctx context.Context, token *Token) error
	CreateNewToken(ctx context.Context, userID int, ttl time.Duration, scope string) (*Token, error)
	DeleteAllForUser(ctx context.Context, scope string, userID int) error
}

func (pts *PostgresStore) CreateNewToken(ctx context.Context, userID int, ttl time.Duration, scope string) (*Token, error) {
	token, err := GenerateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = pts.Insert(ctx, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (pts *PostgresStore) Insert(ctx context.Context, token *Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES ($1, $2, $3, $4)`

	_, err := pts.db.ExecContext(ctx, query, token.Hash, token.UserID, token.Expiry, token.Scope)
	return err
}

func (pts *PostgresStore) DeleteAllForUser(ctx context.Context, scope string, userID int) error {
	query := `
		DELETE FROM tokens
		WHERE scope = $1 AND user_id = $2`

	_, err := pts.db.ExecContext(ctx, query, scope, userID)
	return err
}
