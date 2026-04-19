package workout

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/httpx"
	"github.com/tsatsarisg/go-fit/internal/user"
)

type Handler struct {
	workoutStore Store
	logger       *log.Logger
}

func NewHandler(workoutStore Store, logger *log.Logger) *Handler {
	return &Handler{workoutStore: workoutStore, logger: logger}
}

// errorMapping captures the workout-specific sentinels so handlers can hand
// them off to httpx.WriteStoreError without re-stating the mapping per call.
var errorMapping = httpx.StoreErrorMapping{
	ResourceName: "Workout",
	NotFoundErr:  ErrNotFound,
	ForbiddenErr: ErrForbidden,
}

func (wh *Handler) HandleGetWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := httpx.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": err.Error()})
		return
	}

	workout, err := wh.workoutStore.GetWorkoutByID(r.Context(), int(workoutID))
	if err != nil {
		wh.logger.Printf("ERROR: GetWorkoutByID: %v\n", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "Failed to retrieve workout"})
		return
	}
	if workout == nil {
		httpx.WriteJson(w, http.StatusNotFound, httpx.Envelope{"error": "Workout not found"})
		return
	}

	httpx.WriteJson(w, http.StatusOK, httpx.Envelope{"workout": workout})

}

func (wh *Handler) HandleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	var wkt Workout
	if derr := httpx.DecodeJSONBody(w, r, &wkt); derr != nil {
		wh.logger.Printf("ERROR: HandleCreateWorkout decode: %v\n", derr)
		httpx.WriteDecodeError(w, derr)
		return
	}

	if verr := validateWorkoutCreate(&wkt); verr != nil {
		httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": verr.Error()})
		return
	}

	currentUser := user.GetUser(r)
	if currentUser.IsAnonymous() {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Unauthenticated"})
		return
	}

	wkt.UserID = currentUser.ID
	createWorkout, err := wh.workoutStore.CreateWorkout(r.Context(), &wkt)
	if err != nil {
		httpx.WriteStoreError(w, wh.logger, err, errorMapping, "Failed to create workout")
		return
	}
	httpx.WriteJson(w, http.StatusCreated, httpx.Envelope{"workout": createWorkout})

}

// validateWorkoutCreate enforces domain invariants up-front so callers get
// a 400 with a useful message instead of a DB-backed 500. Mirrors the DB
// CHECK on workout_entries (reps XOR duration) plus non-negative numerics.
func validateWorkoutCreate(wkt *Workout) error {
	if wkt.Title == "" {
		return errors.New("title must not be empty")
	}
	if wkt.DurationMinutes < 0 {
		return errors.New("duration_minutes must be non-negative")
	}
	if wkt.CaloriesBurned < 0 {
		return errors.New("calories_burned must be non-negative")
	}
	for i := range wkt.Entries {
		if err := validateWorkoutEntry(i, &wkt.Entries[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkoutEntry(idx int, e *WorkoutEntry) error {
	if e.ExerciseName == "" {
		return fmt.Errorf("entries[%d]: exercise_name must not be empty", idx)
	}
	if e.Sets < 0 {
		return fmt.Errorf("entries[%d]: sets must be non-negative", idx)
	}
	// Exactly one of reps / duration_seconds must be set. This matches the
	// workout_entries CHECK constraint so we fail-fast with 400 instead of
	// bouncing through Postgres for a 23514.
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

func (wh *Handler) HandleUpdateWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := httpx.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": err.Error()})
		return
	}

	currentUser := user.GetUser(r)
	if currentUser.IsAnonymous() {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Unauthenticated"})
		return
	}

	var updatedWorkoutRequest struct {
		Title           *string         `json:"title"`
		Description     *string         `json:"description"`
		DurationMinutes *int            `json:"duration_minutes"`
		CaloriesBurned  *int            `json:"calories_burned"`
		Entries         *[]WorkoutEntry `json:"entries"`
	}

	if derr := httpx.DecodeJSONBody(w, r, &updatedWorkoutRequest); derr != nil {
		wh.logger.Printf("ERROR: HandleUpdateWorkout decode: %v\n", derr)
		httpx.WriteDecodeError(w, derr)
		return
	}

	patch := WorkoutPatch{
		Title:           updatedWorkoutRequest.Title,
		Description:     updatedWorkoutRequest.Description,
		DurationMinutes: updatedWorkoutRequest.DurationMinutes,
		CaloriesBurned:  updatedWorkoutRequest.CaloriesBurned,
		Entries:         updatedWorkoutRequest.Entries,
	}

	updatedWorkout, err := wh.workoutStore.UpdateWorkout(r.Context(), int(workoutID), currentUser.ID, patch)
	if err != nil {
		httpx.WriteStoreError(w, wh.logger, err, errorMapping, "Failed to update workout")
		return
	}

	httpx.WriteJson(w, http.StatusOK, httpx.Envelope{"workout": updatedWorkout})
}

func (wh *Handler) HandleDeleteWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := httpx.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": err.Error()})
		return
	}

	currentUser := user.GetUser(r)
	if currentUser.IsAnonymous() {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Unauthenticated"})
		return
	}

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(r.Context(), int(workoutID))
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			httpx.WriteJson(w, http.StatusNotFound, httpx.Envelope{"error": "Workout not found"})
			return
		}
		wh.logger.Printf("ERROR: GetWorkoutOwner: %v\n", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "Failed to retrieve workout owner"})
		return
	}

	if workoutOwner != currentUser.ID {
		wh.logger.Printf("ERROR: HandleDeleteWorkout: unauthorized user\n")
		httpx.WriteJson(w, http.StatusForbidden, httpx.Envelope{"error": "Forbidden"})
		return
	}

	err = wh.workoutStore.DeleteWorkout(r.Context(), int(workoutID))
	if err != nil {
		httpx.WriteStoreError(w, wh.logger, err, errorMapping, "Failed to delete workout")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
