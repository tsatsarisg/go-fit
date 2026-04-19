package store

import (
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
)

// ErrWorkoutNotFound is returned when a workout lookup finds no matching row.
var (
	ErrWorkoutNotFound     = errors.New("workout not found")
	ErrDuplicate           = errors.New("duplicate resource")   // 23505 unique violation
	ErrConstraintViolation = errors.New("constraint violation") // 23514 CHECK, etc.
	ErrForbidden           = errors.New("forbidden")
)

// classify inspects err for a *pgconn.PgError and returns the matching sentinel
// wrapped with %w so callers can use errors.Is. Returns the original err if it
// isn't a pg error or isn't a code we classify.
func classify(err error) error {
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
