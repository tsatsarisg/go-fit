package httpx

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
)

// StoreErrorMapping lets callers express domain-specific sentinel → HTTP
// mappings that WriteStoreError will consult before the built-in postgres
// sentinels. NotFoundErr and ForbiddenErr are optional — leave them nil if
// the caller doesn't own sentinels with those meanings.
//
// ResourceName is used to build the 404 body ("<ResourceName> not found") so
// we no longer hardcode "Workout" for every caller.
type StoreErrorMapping struct {
	ResourceName string
	NotFoundErr  error
	ForbiddenErr error
}

// WriteStoreError maps a store-layer error to an appropriate HTTP response.
//
// Order of checks:
//  1. domain NotFoundErr (if provided) → 404 with "<ResourceName> not found"
//  2. domain ForbiddenErr (if provided) → 403
//  3. postgres.ErrDuplicate → 409
//  4. postgres.ErrConstraintViolation → 400
//  5. sql.ErrNoRows → 404 (legacy fallback)
//  6. anything else → 500 with fallbackMsg, logged
//
// Response bodies are intentionally generic to avoid leaking which column /
// constraint was violated (e.g. enumeration resistance on duplicate email vs.
// username during registration).
// WriteStoreError takes the request context so the fallback 500 log line is
// emitted via ErrorContext — that route the request_id attribution wrapper.
func WriteStoreError(ctx context.Context, w http.ResponseWriter, logger *slog.Logger, err error, m StoreErrorMapping, fallbackMsg string) {
	resource := m.ResourceName
	if resource == "" {
		resource = "resource"
	}

	switch {
	case m.NotFoundErr != nil && errors.Is(err, m.NotFoundErr):
		WriteJson(w, http.StatusNotFound, Envelope{"error": resource + " not found"})
	case m.ForbiddenErr != nil && errors.Is(err, m.ForbiddenErr):
		WriteJson(w, http.StatusForbidden, Envelope{"error": "Forbidden"})
	case errors.Is(err, postgres.ErrDuplicate):
		WriteJson(w, http.StatusConflict, Envelope{"error": "resource already exists"})
	case errors.Is(err, postgres.ErrConstraintViolation):
		WriteJson(w, http.StatusBadRequest, Envelope{"error": "invalid request"})
	case errors.Is(err, sql.ErrNoRows):
		WriteJson(w, http.StatusNotFound, Envelope{"error": resource + " not found"})
	default:
		if logger != nil {
			logger.ErrorContext(ctx, "store call failed", slog.Any("err", err))
		}
		WriteJson(w, http.StatusInternalServerError, Envelope{"error": fallbackMsg})
	}
}
