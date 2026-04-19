package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MaxRequestBodyBytes bounds the size of JSON request payloads accepted by
// handlers. Requests larger than this are rejected with 413.
const MaxRequestBodyBytes int64 = 1 << 20 // 1 MiB

// DecodeJSONBody reads a JSON body into dst with safe defaults:
//   - caps the body at MaxRequestBodyBytes via http.MaxBytesReader
//   - rejects unknown fields
//   - rejects trailing data after the top-level object
//
// It returns a DecodeError whose Status is:
//   - 413 Request Entity Too Large when the body exceeds the limit
//   - 400 Bad Request for any other decode failure
//
// Callers should pass the result to WriteDecodeError to produce a generic
// JSON response, or handle the status themselves. Returns nil on success.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) *DecodeError {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return &DecodeError{
				Status:  http.StatusRequestEntityTooLarge,
				Message: fmt.Sprintf("request body must not exceed %d bytes", MaxRequestBodyBytes),
				Err:     err,
			}
		}
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "invalid request payload",
			Err:     err,
		}
	}

	// Disallow anything after the top-level value: protects against smuggled
	// trailing JSON which DisallowUnknownFields alone won't catch.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &DecodeError{
			Status:  http.StatusBadRequest,
			Message: "request body must contain a single JSON object",
			Err:     err,
		}
	}

	return nil
}

// DecodeError is returned by DecodeJSONBody on any body-reading failure.
type DecodeError struct {
	Status  int
	Message string
	Err     error
}

func (e *DecodeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *DecodeError) Unwrap() error { return e.Err }

// WriteDecodeError writes a JSON response matching the error's Status.
func WriteDecodeError(w http.ResponseWriter, err *DecodeError) {
	WriteJson(w, err.Status, Envelope{"error": err.Message})
}
