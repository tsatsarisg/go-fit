package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// RequestLogger logs each HTTP request's method, path, status, and duration
// at Info level. Pairs with chi.middleware.RequestID upstream so the record
// carries request_id (via the wrapper in logger.go).
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusCapture{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)
			logger.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// statusCapture records the response status code so RequestLogger can log it.
// The zero default is 200 because WriteHeader is optional for 200 OK.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (s *statusCapture) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
