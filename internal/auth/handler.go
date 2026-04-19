package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

type createTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if derr := httpx.DecodeJSONBody(w, r, &req); derr != nil {
		h.logger.WarnContext(r.Context(), "decode create token", slog.Any("err", derr))
		httpx.WriteDecodeError(w, derr)
		return
	}

	token, err := h.service.Login(r.Context(), LoginCommand{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "invalid credentials"})
			return
		}
		h.logger.ErrorContext(r.Context(), "login failed", slog.Any("err", err))
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}

	// 200 OK — token creation is RPC-style authentication, not a REST
	// resource creation (no addressable URI for the token).
	httpx.WriteJson(w, http.StatusOK, httpx.Envelope{"token": token.Plaintext, "expiry": token.Expiry})
}

// HandleLogout revokes every authentication-scoped token for the current
// principal. RequireAuthenticatedUser guards the route so an anonymous
// caller never reaches here, but we double-check defensively.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	p := GetPrincipal(r)
	if p.IsAnonymous() {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Unauthenticated"})
		return
	}

	if err := h.service.Logout(r.Context(), p.ID); err != nil {
		h.logger.ErrorContext(r.Context(), "logout failed", slog.Any("err", err))
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
