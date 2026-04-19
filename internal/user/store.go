package user

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// Store is the set of user-related persistence operations used by handlers
// and middleware. GetUserToken lives here — not in the auth package — because
// middleware needs to resolve a bearer token to a user without creating an
// import cycle (auth already imports user for login).
type Store interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	GetUserToken(ctx context.Context, scope, plainTextPassword string) (*User, error)
}

func (store *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	query := `INSERT INTO users (username, email, password_hash, bio)
			  VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	err := store.db.QueryRowContext(ctx, query, user.Username, user.Email, user.PasswordHash.hash, user.Bio).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return postgres.ClassifyError(err)
	}
	return nil
}

func (store *PostgresStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{
		PasswordHash: password{},
	}
	query := `SELECT id, username, email, password_hash, bio, created_at, updated_at FROM users WHERE username = $1`
	row := store.db.QueryRowContext(ctx, query, username)

	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash.hash, &user.Bio, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (store *PostgresStore) UpdateUser(ctx context.Context, user *User) error {
	query := `UPDATE users SET email = $1, username = $2, bio = $3, updated_at = NOW() WHERE id = $4 RETURNING updated_at`
	err := store.db.QueryRowContext(ctx, query, user.Email, user.Username, user.Bio, user.ID).Scan(&user.UpdatedAt)
	if err != nil {
		return postgres.ClassifyError(err)
	}
	return nil
}

func (store *PostgresStore) GetUserToken(ctx context.Context, scope, plainTextPassword string) (*User, error) {
	tokenHash := sha256.Sum256([]byte(plainTextPassword))
	query := `	SELECT u.id, u.username, u.email, u.password_hash, u.bio, u.created_at, u.updated_at
				FROM users u
				INNER JOIN tokens t ON u.id = t.user_id
				WHERE t.scope = $1 AND t.hash = $2 AND t.expiry > $3`

	user := &User{
		PasswordHash: password{},
	}
	err := store.db.QueryRowContext(ctx, query, scope, tokenHash[:], time.Now()).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash.hash, &user.Bio, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}
