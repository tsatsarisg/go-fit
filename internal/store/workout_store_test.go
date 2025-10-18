package store

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("pgx", "host=localhost port=5433 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	err = Migrate(db, "../../migrations/")
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	_, err = db.Exec(`TRUNCATE TABLE workout_entries CASCADE;`)
	if err != nil {
		t.Fatalf("failed to truncate workout_entries table: %v", err)
	}

	return db
}

func TestCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	store := NewPostgresWorkoutStore(db)

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
			createdWorkout, err := store.CreateWorkout(tt.workout)
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

			retrievedWorkout, err := store.GetWorkoutByID(createdWorkout.ID)
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
