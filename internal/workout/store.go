package workout

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
)

// Domain-level sentinels. ErrNotFound corresponds to "workout id doesn't exist",
// ErrForbidden to "row exists but belongs to a different user" — callers use
// errors.Is to map to the appropriate HTTP status.
var (
	ErrNotFound  = errors.New("workout not found")
	ErrForbidden = errors.New("forbidden")
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

type WorkoutPatch struct {
	Title           *string
	Description     *string
	DurationMinutes *int
	CaloriesBurned  *int
	// Entries is nil when the caller wants to leave entries untouched.
	// A non-nil pointer (even to an empty slice) triggers a full replace.
	Entries *[]WorkoutEntry
}

type Store interface {
	CreateWorkout(ctx context.Context, workout *Workout) (*Workout, error)
	GetWorkoutByID(ctx context.Context, id int) (*Workout, error)
	UpdateWorkout(ctx context.Context, id int, userID int, patch WorkoutPatch) (*Workout, error)
	DeleteWorkout(ctx context.Context, id int) error
	GetWorkoutOwner(ctx context.Context, id int) (int, error)
}

func (pg *PostgresStore) CreateWorkout(ctx context.Context, workout *Workout) (*Workout, error) {
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `INSERT INTO workouts (user_id, title, description, duration_minutes, calories_burned)
			  VALUES ($1, $2, $3, $4, $5)
			  RETURNING id`

	// Insert the workout
	err = tx.QueryRowContext(ctx, query, workout.UserID, workout.Title, workout.Description, workout.DurationMinutes, workout.CaloriesBurned).Scan(&workout.ID)
	if err != nil {
		return nil, postgres.ClassifyError(err)
	}

	// Insert the workout entries
	for i := range workout.Entries {
		query := `
			INSERT INTO workout_entries (workout_id, exercise_name, sets, reps, duration_seconds, weight, notes, order_index)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`

		entry := &workout.Entries[i]
		err = tx.QueryRowContext(ctx, query, workout.ID, entry.ExerciseName, entry.Sets, entry.Reps, entry.DurationSeconds, entry.Weight, entry.Notes, entry.OrderIndex).Scan(&entry.ID)
		if err != nil {
			return nil, postgres.ClassifyError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return workout, nil
}

func (pg *PostgresStore) GetWorkoutByID(ctx context.Context, id int) (*Workout, error) {
	query := `SELECT id, user_id, title, description, duration_minutes, calories_burned
			  FROM workouts
			  WHERE id = $1`

	workout := &Workout{}
	err := pg.db.QueryRowContext(ctx, query, id).Scan(&workout.ID, &workout.UserID, &workout.Title, &workout.Description, &workout.DurationMinutes, &workout.CaloriesBurned)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	entriesQuery := `SELECT id, exercise_name, sets, reps, duration_seconds, weight, notes, order_index
					 FROM workout_entries
					 WHERE workout_id = $1
					 ORDER BY order_index`

	rows, err := pg.db.QueryContext(ctx, entriesQuery, workout.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		entry := WorkoutEntry{}
		err := rows.Scan(&entry.ID, &entry.ExerciseName, &entry.Sets, &entry.Reps, &entry.DurationSeconds, &entry.Weight, &entry.Notes, &entry.OrderIndex)
		if err != nil {
			return nil, err
		}
		workout.Entries = append(workout.Entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return workout, nil
}

func (pg *PostgresStore) UpdateWorkout(ctx context.Context, id int, userID int, patch WorkoutPatch) (*Workout, error) {
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Single round-trip: enforce ownership in SQL and preserve unspecified
	// fields via COALESCE. Explicit casts keep pgx happy about parameter types
	// when the typed pointer is nil. updated_at always bumps on a successful
	// update so the row's audit timestamp stays accurate.
	updateQuery := `UPDATE workouts
					SET title = COALESCE($1::text, title),
					    description = COALESCE($2::text, description),
					    duration_minutes = COALESCE($3::int, duration_minutes),
					    calories_burned = COALESCE($4::int, calories_burned),
					    updated_at = NOW()
					WHERE id = $5 AND user_id = $6
					RETURNING id, user_id, title, description, duration_minutes, calories_burned`

	workout := &Workout{}
	err = tx.QueryRowContext(
		ctx,
		updateQuery,
		patch.Title,
		patch.Description,
		patch.DurationMinutes,
		patch.CaloriesBurned,
		id,
		userID,
	).Scan(
		&workout.ID,
		&workout.UserID,
		&workout.Title,
		&workout.Description,
		&workout.DurationMinutes,
		&workout.CaloriesBurned,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Disambiguate 404 vs 403 inside the same tx.
		var ownerID int
		probeErr := tx.QueryRowContext(ctx, `SELECT user_id FROM workouts WHERE id = $1`, id).Scan(&ownerID)
		if errors.Is(probeErr, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if probeErr != nil {
			return nil, probeErr
		}
		if ownerID != userID {
			return nil, ErrForbidden
		}
		// Row exists with matching user but UPDATE touched 0 rows — shouldn't
		// happen; treat defensively as not found.
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, postgres.ClassifyError(err)
	}

	// Entries: nil => leave existing rows untouched; non-nil => full replace.
	if patch.Entries != nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM workout_entries WHERE workout_id = $1`, workout.ID)
		if err != nil {
			return nil, err
		}

		newEntries := *patch.Entries
		for i := range newEntries {
			insertQuery := `
				INSERT INTO workout_entries (workout_id, exercise_name, sets, reps, duration_seconds, weight, notes, order_index)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING id
			`

			entry := &newEntries[i]
			err = tx.QueryRowContext(ctx, insertQuery, workout.ID, entry.ExerciseName, entry.Sets, entry.Reps, entry.DurationSeconds, entry.Weight, entry.Notes, entry.OrderIndex).Scan(&entry.ID)
			if err != nil {
				return nil, postgres.ClassifyError(err)
			}
		}
	}

	// Re-read entries inside the tx so the handler gets a full response.
	entriesQuery := `SELECT id, exercise_name, sets, reps, duration_seconds, weight, notes, order_index
					 FROM workout_entries
					 WHERE workout_id = $1
					 ORDER BY order_index`

	rows, err := tx.QueryContext(ctx, entriesQuery, workout.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		entry := WorkoutEntry{}
		if err := rows.Scan(&entry.ID, &entry.ExerciseName, &entry.Sets, &entry.Reps, &entry.DurationSeconds, &entry.Weight, &entry.Notes, &entry.OrderIndex); err != nil {
			return nil, err
		}
		workout.Entries = append(workout.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return workout, nil
}

func (pg *PostgresStore) DeleteWorkout(ctx context.Context, id int) error {
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM workout_entries WHERE workout_id = $1`, id)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM workouts WHERE id = $1`, id)
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

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (pg *PostgresStore) GetWorkoutOwner(ctx context.Context, id int) (int, error) {
	query := `SELECT user_id FROM workouts WHERE id = $1`

	var userID int
	err := pg.db.QueryRowContext(ctx, query, id).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	return userID, nil
}
