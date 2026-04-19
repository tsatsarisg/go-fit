package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/tsatsarisg/go-fit/internal/store"
	"github.com/tsatsarisg/go-fit/internal/tokens"
	"github.com/tsatsarisg/go-fit/internal/utils"
)

type TokenHandler struct {
	tokenStore store.TokensStore
	userStore  store.UserStore
	logger     *log.Logger
}

type createTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewTokenHandler(tokenStore store.TokensStore, userStore store.UserStore, logger *log.Logger) *TokenHandler {
	return &TokenHandler{
		tokenStore: tokenStore,
		userStore:  userStore,
		logger:     logger,
	}
}

func (h *TokenHandler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		h.logger.Println("Error decoding create token request:", err)
		utils.WriteJson(w, http.StatusBadRequest, utils.Envelope{"error": "invalid request payload"})
		return
	}

	user, err := h.userStore.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		h.logger.Println("ERROR: GetUserByUsername failed:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal error"})
		return
	}

	ok, err := store.VerifyPassword(user, req.Password)
	if err != nil {
		h.logger.Println("ERROR: VerifyPassword failed:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal error"})
		return
	}
	if !ok {
		utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "invalid credentials"})
		return
	}

	token, err := h.tokenStore.CreateNewToken(r.Context(), user.ID, 24*time.Hour, tokens.ScopeAuth)
	if err != nil {
		h.logger.Println("ERROR: CreateNewToken failed:", err)
		utils.WriteJson(w, http.StatusInternalServerError, utils.Envelope{"error": "internal error"})
		return
	}

	utils.WriteJson(w, http.StatusCreated, utils.Envelope{"token": token.Plaintext, "expiry": token.Expiry})

}
