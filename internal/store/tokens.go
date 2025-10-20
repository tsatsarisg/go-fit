package store

import (
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
	Insert(token *tokens.Token) error
	CreateNewToken(userID int, ttl time.Duration, scope string) (*tokens.Token, error)
	DeleteAllForUser(scope string, userID int) error
}

func (pts *PostgresTokensStore) CreateNewToken(userID int, ttl time.Duration, scope string) (*tokens.Token, error) {
	token, err := tokens.GenerateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = pts.Insert(token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (pts *PostgresTokensStore) Insert(token *tokens.Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES ($1, $2, $3, $4)`

	_, err := pts.db.Exec(query, token.Hash, token.UserID, token.Expiry, token.Scope)
	return err
}

func (pts *PostgresTokensStore) DeleteAllForUser(scope string, userID int) error {
	query := `
		DELETE FROM tokens
		WHERE scope = $1 AND user_id = $2`

	_, err := pts.db.Exec(query, scope, userID)
	return err
}
