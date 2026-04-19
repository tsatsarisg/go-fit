package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinels classify postgres-layer error conditions that domain stores expose
// upward via errors.Is. These live here (rather than in each domain) because
// they're infrastructure-level classifications — the pg SQLSTATE codes are the
// same whether a workout insert or a user insert violated a unique constraint.
var (
	ErrDuplicate           = errors.New("duplicate resource")   // 23505 unique violation
	ErrConstraintViolation = errors.New("constraint violation") // 23514 CHECK, etc.
)

// ClassifyError inspects err for a *pgconn.PgError and returns the matching
// sentinel wrapped with %w so callers can use errors.Is. Returns the original
// err if it isn't a pg error or isn't a code we classify.
func ClassifyError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return fmt.Errorf("%w: %s", ErrDuplicate, pgErr.ConstraintName)
	case "23514":
		return fmt.Errorf("%w: %s", ErrConstraintViolation, pgErr.ConstraintName)
	default:
		return err
	}
}
