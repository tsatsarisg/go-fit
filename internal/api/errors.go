package api

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/store"
	"github.com/tsatsarisg/go-fit/internal/utils"
)

// writeStoreError maps a store-layer error to an appropriate HTTP response.
// It handles the classified sentinels (ErrDuplicate, ErrConstraintViolation,
// ErrWorkoutNotFound) plus the legacy sql.ErrNoRows path. Any unrecognized
// error is logged and returned as a 500 with fallbackMsg.
//
// Response bodies are intentionally generic to avoid leaking which column /
// constraint was violated (e.g. enumeration resistance on duplicate email vs.
// username during registration).
func writeStoreError(w http.ResponseWriter, logger *log.Logger, err error, fallbackMsg string) {
	switch {
	case errors.Is(err, store.ErrDuplicate):
		utils.WriteJson(w, http.StatusConflict, utils.Envelope{"error": "resource already exists"})
	case errors.Is(err, store.ErrConstraintViolation):
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request"})
	case errors.Is(err, store.ErrWorkoutNotFound):
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
	case errors.Is(err, store.ErrForbidden):
		utils.WriteJson(w, http.StatusForbidden, utils.Envelope{"error": "Forbidden"})
	case errors.Is(err, sql.ErrNoRows):
		utils.WriteJson(w, http.StatusNotFound, utils.Envelope{"error": "Workout not found"})
	default:
		if logger != nil {
			logger.Printf("ERROR: store call: %v\n", err)
		}
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": fallbackMsg})
	}
}
