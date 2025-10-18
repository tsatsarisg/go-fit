package store

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type password struct {
	plainText string
	hash      []byte
}

func (p *password) Set(plainText string) error {
	p.plainText = plainText
	hash, err := bcrypt.GenerateFromPassword([]byte(plainText), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	p.hash = hash
	return nil
}

func (p *password) Matches(plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plainText))

	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash password  `json:"-"`
	Bio          string    `json:"bio,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

type UserStore interface {
	CreateUser(user *User) error
	GetUserByUsername(username string) (*User, error)
	UpdateUser(user *User) error
}

func (store *PostgresUserStore) CreateUser(user *User) error {
	query := `INSERT INTO users (username, email, password_hash, bio)
			  VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	err := store.db.QueryRow(query, user.Username, user.Email, user.PasswordHash.hash, user.Bio).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (store *PostgresUserStore) GetUserByUsername(username string) (*User, error) {
	user := &User{
		PasswordHash: password{},
	}
	query := `SELECT id, username, email, password_hash, bio, created_at, updated_at FROM users WHERE username = $1`
	row := store.db.QueryRow(query, username)

	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash.hash, &user.Bio, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (store *PostgresUserStore) UpdateUser(user *User) error {
	query := `UPDATE users SET email = $1, username = $2, bio = $3, updated_at = NOW() WHERE username = $4 RETURNING updated_at`
	result, err := store.db.Exec(query, user.Email, user.Username, user.Bio)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
