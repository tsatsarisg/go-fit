package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/tsatsarisg/go-fit/internal/tokens"
)

type PostgresTokensStore struct {
	db *sql.DB
}

func NewPostgresTokensStore(db *sql.DB) *PostgresTokensStore {
	return &PostgresTokensStore{db: db}
}

type TokensStore interface {
	Insert(ctx context.Context, token *tokens.Token) error
	CreateNewToken(ctx context.Context, userID int, ttl time.Duration, scope string) (*tokens.Token, error)
	DeleteAllForUser(ctx context.Context, scope string, userID int) error
}

func (pts *PostgresTokensStore) CreateNewToken(ctx context.Context, userID int, ttl time.Duration, scope string) (*tokens.Token, error) {
	token, err := tokens.GenerateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = pts.Insert(ctx, token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (pts *PostgresTokensStore) Insert(ctx context.Context, token *tokens.Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES ($1, $2, $3, $4)`

	_, err := pts.db.ExecContext(ctx, query, token.Hash, token.UserID, token.Expiry, token.Scope)
	return err
}

func (pts *PostgresTokensStore) DeleteAllForUser(ctx context.Context, scope string, userID int) error {
	query := `
		DELETE FROM tokens
		WHERE scope = $1 AND user_id = $2`

	_, err := pts.db.ExecContext(ctx, query, scope, userID)
	return err
}
