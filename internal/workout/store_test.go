package workout

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"

	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
	"github.com/tsatsarisg/go-fit/internal/user"
)

const defaultTestDSN = "host=localhost port=5433 user=postgres password=postgres dbname=postgres sslmode=disable"

// setupTestDB wipes the workouts-related tables and seeds a single user so
// the FK on workouts.user_id can be satisfied. Returns the seeded user's id.
func setupTestDB(t *testing.T) (*sql.DB, user.UserID) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	err = postgres.Migrate(db, "../../migrations/")
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	ctx := context.Background()
	// Wipe workouts + entries + test user so each run starts clean.
	if _, err := db.ExecContext(ctx, `TRUNCATE TABLE workout_entries, workouts, users RESTART IDENTITY CASCADE;`); err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	var userID user.UserID
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (username, email, password_hash, bio)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		"test_user", "test@example.com", "unused-hash", "",
	).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to seed test user: %v", err)
	}

	return db, userID
}

func TestCreate(t *testing.T) {
	db, userID := setupTestDB(t)
	defer db.Close()

	store := NewPostgresStore(db)

	tests := []struct {
		name    string
		workout *Workout
		wantErr bool
	}{
		{name: "valid workout",
			workout: &Workout{
				Title:           "Morning Routine",
				Description:     "A quick morning workout",
				DurationMinutes: 30,
				CaloriesBurned:  250,
				Entries: []WorkoutEntry{
					{
						ExerciseName: "Push Ups",
						Sets:         3,
						Reps:         ptrInt(15),
						Weight:       ptrFloat64(1.5),
						Notes:        "Felt good",
						OrderIndex:   0,
					},
					{
						ExerciseName: "Plank",
						Sets:         3,
						Reps:         ptrInt(10),
						Weight:       ptrFloat64(1.0),
						Notes:        "Challenging",
						OrderIndex:   1,
					},
				},
			},
			wantErr: false,
		}, {
			name: "workout with invalid entries",
			workout: &Workout{
				Title:           "Evening Routine",
				Description:     "A quick evening workout",
				DurationMinutes: 20,
				CaloriesBurned:  200,
				Entries: []WorkoutEntry{
					{
						ExerciseName: "Squats",
						Sets:         3,
						Reps:         nil,
						Weight:       ptrFloat64(2.0),
						Notes:        "Tiring",
						OrderIndex:   0,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tt.workout.UserID = userID
			createdWorkout, err := store.CreateWorkout(ctx, tt.workout)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotZero(t, createdWorkout.ID)
			assert.Equal(t, tt.workout.Title, createdWorkout.Title)
			assert.Equal(t, tt.workout.Description, createdWorkout.Description)
			assert.Equal(t, tt.workout.DurationMinutes, createdWorkout.DurationMinutes)
			assert.Equal(t, tt.workout.CaloriesBurned, createdWorkout.CaloriesBurned)
			assert.Len(t, createdWorkout.Entries, len(tt.workout.Entries))

			retrievedWorkout, err := store.GetWorkoutByID(ctx, createdWorkout.ID)
			assert.NoError(t, err)
			assert.Equal(t, createdWorkout, retrievedWorkout)
			assert.Equal(t, len(tt.workout.Entries), len(retrievedWorkout.Entries))

			for i, entry := range createdWorkout.Entries {
				expectedEntry := tt.workout.Entries[i]
				assert.NotZero(t, entry.ID)
				assert.Equal(t, expectedEntry.ExerciseName, entry.ExerciseName)
				assert.Equal(t, expectedEntry.Sets, entry.Sets)
				assert.Equal(t, expectedEntry.Reps, entry.Reps)
				assert.Equal(t, expectedEntry.DurationSeconds, entry.DurationSeconds)
				assert.Equal(t, expectedEntry.Weight, entry.Weight)
				assert.Equal(t, expectedEntry.Notes, entry.Notes)
				assert.Equal(t, expectedEntry.OrderIndex, entry.OrderIndex)
			}

		})
	}
}

func ptrInt(i int) *int {
	return &i
}
func ptrFloat64(f float64) *float64 {
	return &f
}
