package workout

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
	"github.com/tsatsarisg/go-fit/internal/user"
)

// PostgresStore is the concrete adapter satisfying the Store interface
// defined in service.go. Named "Postgres..." because other adapters (in-memory
// fakes, future read replicas) will live alongside it in this package.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
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

	err = tx.QueryRowContext(ctx, query, workout.UserID, workout.Title, workout.Description, workout.DurationMinutes, workout.CaloriesBurned).Scan(&workout.ID)
	if err != nil {
		return nil, postgres.ClassifyError(err)
	}

	for i := range workout.Entries {
		entryQuery := `
			INSERT INTO workout_entries (workout_id, exercise_name, sets, reps, duration_seconds, weight, notes, order_index)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`

		entry := &workout.Entries[i]
		err = tx.QueryRowContext(ctx, entryQuery, workout.ID, entry.ExerciseName, entry.Sets, entry.Reps, entry.DurationSeconds, entry.Weight, entry.Notes, entry.OrderIndex).Scan(&entry.ID)
		if err != nil {
			return nil, postgres.ClassifyError(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return workout, nil
}

func (pg *PostgresStore) GetWorkoutByID(ctx context.Context, id WorkoutID) (*Workout, error) {
	query := `SELECT id, user_id, title, description, duration_minutes, calories_burned
			  FROM workouts
			  WHERE id = $1`

	workout := &Workout{}
	err := pg.db.QueryRowContext(ctx, query, id).Scan(&workout.ID, &workout.UserID, &workout.Title, &workout.Description, &workout.DurationMinutes, &workout.CaloriesBurned)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
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
		if err := rows.Scan(&entry.ID, &entry.ExerciseName, &entry.Sets, &entry.Reps, &entry.DurationSeconds, &entry.Weight, &entry.Notes, &entry.OrderIndex); err != nil {
			return nil, err
		}
		workout.Entries = append(workout.Entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return workout, nil
}

func (pg *PostgresStore) UpdateWorkout(ctx context.Context, id WorkoutID, userID user.UserID, patch WorkoutPatch) (*Workout, error) {
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
		var ownerID user.UserID
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
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, postgres.ClassifyError(err)
	}

	if patch.Entries != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM workout_entries WHERE workout_id = $1`, workout.ID); err != nil {
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
			if err := tx.QueryRowContext(ctx, insertQuery, workout.ID, entry.ExerciseName, entry.Sets, entry.Reps, entry.DurationSeconds, entry.Weight, entry.Notes, entry.OrderIndex).Scan(&entry.ID); err != nil {
				return nil, postgres.ClassifyError(err)
			}
		}
	}

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

// DeleteWorkout removes a workout and its entries in a single tx, enforcing
// ownership in the WHERE clause. Uses DELETE ... RETURNING to learn whether
// a row was actually removed without a second round trip, and disambiguates
// 404 vs 403 via a probe inside the same tx.
func (pg *PostgresStore) DeleteWorkout(ctx context.Context, id WorkoutID, userID user.UserID) error {
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM workout_entries WHERE workout_id = $1`, id); err != nil {
		return err
	}

	var deletedID WorkoutID
	err = tx.QueryRowContext(
		ctx,
		`DELETE FROM workouts WHERE id = $1 AND user_id = $2 RETURNING id`,
		id, userID,
	).Scan(&deletedID)
	if errors.Is(err, sql.ErrNoRows) {
		var ownerID user.UserID
		probeErr := tx.QueryRowContext(ctx, `SELECT user_id FROM workouts WHERE id = $1`, id).Scan(&ownerID)
		if errors.Is(probeErr, sql.ErrNoRows) {
			return ErrNotFound
		}
		if probeErr != nil {
			return probeErr
		}
		return ErrForbidden
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}
