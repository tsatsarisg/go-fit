package httpx

import (
	"context"
	"io"
	"log/slog"

	"github.com/go-chi/chi/v5/middleware"
)

// NewLogger wraps an underlying slog.Handler so every record automatically
// carries the chi request_id when one is set on the context. Callers choose
// between text and JSON output via NewHandler / chi.middleware.RequestID must
// be installed upstream in the router for request_id attribution to fire.
func NewLogger(h slog.Handler) *slog.Logger {
	return slog.New(&requestIDHandler{Handler: h})
}

// NewHandler returns a JSON slog handler for production or text for dev.
// Level is Info in prod, Debug in dev. Splitting creation from the wrapper
// keeps request-id-injection orthogonal to format choice.
func NewHandler(w io.Writer, production bool) slog.Handler {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	if production {
		opts.Level = slog.LevelInfo
		return slog.NewJSONHandler(w, opts)
	}
	return slog.NewTextHandler(w, opts)
}

// requestIDHandler wraps any slog.Handler and, on every Handle call, reads
// the chi request id from ctx and attaches it as an attribute. That way every
// logger.ErrorContext / InfoContext gets the id "for free" — handlers don't
// have to remember to attach it by hand.
type requestIDHandler struct {
	slog.Handler
}

func (h *requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if rid := middleware.GetReqID(ctx); rid != "" {
		r.AddAttrs(slog.String("request_id", rid))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs / WithGroup rewrap so child loggers produced by logger.With(...)
// also carry the request-id injection. Without this the wrapper is lost the
// moment anyone calls .With on the logger.
func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	return &requestIDHandler{Handler: h.Handler.WithGroup(name)}
}
