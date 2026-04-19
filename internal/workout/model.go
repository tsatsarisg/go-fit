package workout

import (
	"errors"
	"fmt"

	"github.com/tsatsarisg/go-fit/internal/user"
)

// WorkoutID is a named int64 wrapping the workouts.id column. Typed so the
// compiler refuses to accept (say) a UserID where a WorkoutID is expected —
// kills the id-mixup bug class D2 calls out. int64 matches BIGSERIAL.
type WorkoutID int64

type Workout struct {
	ID              WorkoutID      `json:"id"`
	UserID          user.UserID    `json:"user_id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	DurationMinutes int            `json:"duration_minutes"`
	CaloriesBurned  int            `json:"calories_burned"`
	Entries         []WorkoutEntry `json:"entries"`
}

type WorkoutEntry struct {
	ID              int      `json:"id"`
	ExerciseName    string   `json:"exercise_name"`
	Sets            int      `json:"sets"`
	Reps            *int     `json:"reps"`
	DurationSeconds *int     `json:"duration_seconds"`
	Weight          *float64 `json:"weight"`
	Notes           string   `json:"notes"`
	OrderIndex      int      `json:"order_index"`
}

// WorkoutPatch is the partial-update input. A nil pointer means "leave the
// existing value alone"; a non-nil Entries pointer (even to an empty slice)
// triggers a full replace of the entry collection.
type WorkoutPatch struct {
	Title           *string
	Description     *string
	DurationMinutes *int
	CaloriesBurned  *int
	Entries         *[]WorkoutEntry
}

// Validate enforces the workout aggregate's invariants so the service layer
// catches bad inputs before they reach the DB. Mirrors the workout_entries
// CHECK constraint (reps XOR duration) plus non-negative numerics.
func (w *Workout) Validate() error {
	if w.Title == "" {
		return errors.New("title must not be empty")
	}
	if w.DurationMinutes < 0 {
		return errors.New("duration_minutes must be non-negative")
	}
	if w.CaloriesBurned < 0 {
		return errors.New("calories_burned must be non-negative")
	}
	for i := range w.Entries {
		if err := w.Entries[i].Validate(i); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks a single entry. idx is threaded through so error messages
// point at the offending element.
func (e *WorkoutEntry) Validate(idx int) error {
	if e.ExerciseName == "" {
		return fmt.Errorf("entries[%d]: exercise_name must not be empty", idx)
	}
	if e.Sets < 0 {
		return fmt.Errorf("entries[%d]: sets must be non-negative", idx)
	}
	hasReps := e.Reps != nil
	hasDuration := e.DurationSeconds != nil
	if hasReps == hasDuration {
		return fmt.Errorf("entries[%d]: exactly one of reps or duration_seconds is required", idx)
	}
	if hasReps && *e.Reps < 0 {
		return fmt.Errorf("entries[%d]: reps must be non-negative", idx)
	}
	if hasDuration && *e.DurationSeconds < 0 {
		return fmt.Errorf("entries[%d]: duration_seconds must be non-negative", idx)
	}
	if e.Weight != nil && *e.Weight < 0 {
		return fmt.Errorf("entries[%d]: weight must be non-negative", idx)
	}
	return nil
}

// Validate enforces the same invariants Workout.Validate does, but only for
// fields the patch actually touches.
func (p *WorkoutPatch) Validate() error {
	if p.Title != nil && *p.Title == "" {
		return fmt.Errorf("title must not be empty")
	}
	if p.DurationMinutes != nil && *p.DurationMinutes < 0 {
		return fmt.Errorf("duration_minutes must be non-negative")
	}
	if p.CaloriesBurned != nil && *p.CaloriesBurned < 0 {
		return fmt.Errorf("calories_burned must be non-negative")
	}
	if p.Entries != nil {
		entries := *p.Entries
		for i := range entries {
			if err := entries[i].Validate(i); err != nil {
				return err
			}
		}
	}
	return nil
}
