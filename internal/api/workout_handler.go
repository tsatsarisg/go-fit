package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/middleware"
	"github.com/tsatsarisg/go-fit/internal/store"
	"github.com/tsatsarisg/go-fit/internal/utils"
)

type WorkoutHandler struct {
	workoutStore store.WorkoutStore
	logger       *log.Logger
}

func NewWorkoutHandler(workoutStore store.WorkoutStore, logger *log.Logger) *WorkoutHandler {
	return &WorkoutHandler{workoutStore: workoutStore, logger: logger}
}

func (wh *WorkoutHandler) HandleGetWorkoutByID(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": err.Error()})
		return
	}

	workout, err := wh.workoutStore.GetWorkoutByID(int(workoutID))
	if err != nil {
		wh.logger.Printf("ERROR: GetWorkoutByID: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to retrieve workout"})
		return
	}
	if workout == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"workout": workout})

}

func (wh *WorkoutHandler) HandleCreateWorkout(w http.ResponseWriter, r *http.Request) {
	var workout store.Workout
	err := json.NewDecoder(r.Body).Decode(&workout)
	if err != nil {
		wh.logger.Printf("ERROR: HandleCreateWorkout: %v\n", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Invalid request payload"})
		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		wh.logger.Printf("ERROR: HandleCreateWorkout: unauthenticated user\n")
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthenticated"})
		return
	}

	workout.UserID = currentUser.ID
	createWorkout, err := wh.workoutStore.CreateWorkout(&workout)
	if err != nil {
		wh.logger.Printf("ERROR: HandleCreateWorkout: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to create workout"})
		return
	}
	utils.WriteJson(w, http.StatusCreated, utils.Envelope{"workout": createWorkout})

}

func (wh *WorkoutHandler) HandleUpdateWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": err.Error()})
		return
	}

	existingWorkout, err := wh.workoutStore.GetWorkoutByID(int(workoutID))
	if err != nil {
		wh.logger.Printf("ERROR: GetWorkoutByID: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to retrieve workout"})
		return
	}
	if existingWorkout == nil {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
		return
	}

	var updatedWorkoutRequest struct {
		Title           *string              `json:"title"`
		Description     *string              `json:"description"`
		DurationMinutes *int                 `json:"duration_minutes"`
		CaloriesBurned  *int                 `json:"calories_burned"`
		Entries         []store.WorkoutEntry `json:"entries"`
	}

	err = json.NewDecoder(r.Body).Decode(&updatedWorkoutRequest)
	if err != nil {
		wh.logger.Printf("ERROR: HandleUpdateWorkout: %v\n", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "Invalid request payload"})
		return
	}
	if updatedWorkoutRequest.Title != nil {
		existingWorkout.Title = *updatedWorkoutRequest.Title
	}
	if updatedWorkoutRequest.Description != nil {
		existingWorkout.Description = *updatedWorkoutRequest.Description
	}
	if updatedWorkoutRequest.DurationMinutes != nil {
		existingWorkout.DurationMinutes = *updatedWorkoutRequest.DurationMinutes
	}
	if updatedWorkoutRequest.CaloriesBurned != nil {
		existingWorkout.CaloriesBurned = *updatedWorkoutRequest.CaloriesBurned
	}
	if updatedWorkoutRequest.Entries != nil {
		existingWorkout.Entries = updatedWorkoutRequest.Entries
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser || existingWorkout.UserID != currentUser.ID {
		wh.logger.Printf("ERROR: HandleUpdateWorkout: unauthorized user\n")
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthorized"})
		return
	}

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(int(workoutID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
			return
		}
		wh.logger.Printf("ERROR: GetWorkoutOwner: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to retrieve workout owner"})
		return
	}

	if workoutOwner != currentUser.ID {
		wh.logger.Printf("ERROR: HandleUpdateWorkout: unauthorized user\n")
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "Forbidden"})
		return
	}

	err = wh.workoutStore.UpdateWorkout(existingWorkout)
	if err != nil {
		wh.logger.Printf("ERROR: UpdateWorkout: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to update workout"})
		return
	}

	utils.WriteJson(w, http.StatusOK, utils.Envelope{"workout": existingWorkout})
}

func (wh *WorkoutHandler) HandleDeleteWorkout(w http.ResponseWriter, r *http.Request) {
	workoutID, err := utils.ReadIdParam(r, "id")
	if err != nil {
		wh.logger.Printf("ERROR: readIdParams: %v\n", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": err.Error()})
		return
	}

	currentUser := middleware.GetUser(r)
	if currentUser == nil || currentUser == store.AnonymousUser {
		wh.logger.Printf("ERROR: HandleUpdateWorkout: unauthorized user\n")
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Unauthorized"})
		return
	}

	workoutOwner, err := wh.workoutStore.GetWorkoutOwner(int(workoutID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
			return
		}
		wh.logger.Printf("ERROR: GetWorkoutOwner: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to retrieve workout owner"})
		return
	}

	if workoutOwner != currentUser.ID {
		wh.logger.Printf("ERROR: HandleDeleteWorkout: unauthorized user\n")
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "Forbidden"})
		return
	}

	err = wh.workoutStore.DeleteWorkout(int(workoutID))
	if err == sql.ErrNoRows {
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
		return
	}

	if err != nil {
		wh.logger.Printf("ERROR: DeleteWorkout: %v\n", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "Failed to delete workout"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
