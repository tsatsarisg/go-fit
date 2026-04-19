package user

import (
	"errors"
	"log"
	"net/http"
	"net/mail"

	"github.com/tsatsarisg/go-fit/internal/httpx"
)

type registerUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Bio      string `json:"bio,omitempty"`
}

type Handler struct {
	userStore Store
	logger    *log.Logger
}

func NewHandler(userStore Store, logger *log.Logger) *Handler {
	return &Handler{
		userStore: userStore,
		logger:    logger,
	}
}

func (h *Handler) validateRegisterRequest(req *registerUserRequest) error {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return errors.New("missing required fields")
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return errors.New("invalid email format")
	}

	if len(req.Password) < 12 {
		return errors.New("password must be at least 12 characters long")
	}

	return nil
}

func (h *Handler) HandleRegisterUser(w http.ResponseWriter, r *http.Request) {
	var req registerUserRequest
	if derr := httpx.DecodeJSONBody(w, r, &req); derr != nil {
		h.logger.Println("Error decoding register user request:", derr)
		httpx.WriteDecodeError(w, derr)
		return
	}

	if err := h.validateRegisterRequest(&req); err != nil {
		httpx.WriteJson(w, http.StatusBadRequest, httpx.Envelope{"error": err.Error()})
		return
	}

	user := &User{
		Username: req.Username,
		Email:    req.Email,
		Bio:      req.Bio,
	}

	err := user.PasswordHash.Set(req.Password)
	if err != nil {
		h.logger.Println("Error hashing password:", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal server error"})
		return
	}

	err = h.userStore.CreateUser(r.Context(), user)
	if err != nil {
		httpx.WriteStoreError(w, h.logger, err, httpx.StoreErrorMapping{ResourceName: "User"}, "internal server error")
		return
	}

	httpx.WriteJson(w, http.StatusCreated, httpx.Envelope{"user": user})

}
