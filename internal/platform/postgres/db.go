package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/pressly/goose/v3"
)

// Open opens a database connection using the supplied DSN, applies pool
// limits, and verifies connectivity with a PingContext before returning.
func Open(ctx context.Context, dsn string, logger *log.Logger) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		// Best-effort close so we don't leak the pool on a failed ping.
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if logger != nil {
		logger.Println("Database connection opened successfully")
	}

	return db, nil
}

func MigrateFS(db *sql.DB, migrationFs fs.FS, dir string) error {
	goose.SetBaseFS(migrationFs)
	defer func() {
		goose.SetBaseFS(nil)
	}()

	return Migrate(db, dir)
}

func Migrate(db *sql.DB, dir string) error {
	// "postgres" is the canonical goose dialect string; "pgx" is accepted
	// as an alias but the public Dialect constant is DialectPostgres.
	err := goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	err = goose.Up(db, dir)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
