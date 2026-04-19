package user

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/tsatsarisg/go-fit/internal/httpx"
)

type registerUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Bio      string `json:"bio,omitempty"`
}

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) HandleRegisterUser(w http.ResponseWriter, r *http.Request) {
	var req registerUserRequest
	if derr := httpx.DecodeJSONBody(w, r, &req); derr != nil {
		h.logger.WarnContext(r.Context(), "decode register user", slog.Any("err", derr))
		httpx.WriteDecodeError(w, derr)
		return
	}

	u, err := h.service.Register(r.Context(), RegisterCommand{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
		Bio:      req.Bio,
	})
	if err != nil {
		if errors.Is(err, ErrValidation) {
			httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": err.Error()})
			return
		}
		httpx.WriteStoreError(r.Context(), w, h.logger, err, httpx.StoreErrorMapping{ResourceName: "User"}, "internal server error")
		return
	}

	httpx.WriteJson(w, http.StatusCreated, httpx.Envelope{"user": u})
}
