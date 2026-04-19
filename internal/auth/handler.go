package auth

import (
	"log"
	"net/http"
	"time"

	"github.com/tsatsarisg/go-fit/internal/httpx"
	"github.com/tsatsarisg/go-fit/internal/user"
)

type Handler struct {
	tokenStore Store
	userStore  user.Store
	logger     *log.Logger
}

type createTokenRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHandler(tokenStore Store, userStore user.Store, logger *log.Logger) *Handler {
	return &Handler{
		tokenStore: tokenStore,
		userStore:  userStore,
		logger:     logger,
	}
}

func (h *Handler) HandleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	if derr := httpx.DecodeJSONBody(w, r, &req); derr != nil {
		h.logger.Println("Error decoding create token request:", derr)
		httpx.WriteDecodeError(w, derr)
		return
	}

	u, err := h.userStore.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		h.logger.Println("ERROR: GetUserByUsername failed:", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}

	ok, err := user.VerifyPassword(u, req.Password)
	if err != nil {
		h.logger.Println("ERROR: VerifyPassword failed:", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}
	if !ok {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "invalid credentials"})
		return
	}

	token, err := h.tokenStore.CreateNewToken(r.Context(), u.ID, 24*time.Hour, user.ScopeAuth)
	if err != nil {
		h.logger.Println("ERROR: CreateNewToken failed:", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}

	// 200 OK — token creation is RPC-style authentication, not a REST
	// resource creation (no addressable URI for the token).
	httpx.WriteJson(w, http.StatusOK, httpx.Envelope{"token": token.Plaintext, "expiry": token.Expiry})

}

// HandleLogout revokes every authentication-scoped token for the current
// user. Requires an authenticated request; RequireAuthenticatedUser guards
// the route so an anonymous caller never reaches here, but we double-check
// defensively.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	currentUser := user.GetUser(r)
	if currentUser.IsAnonymous() {
		httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Unauthenticated"})
		return
	}

	if err := h.tokenStore.DeleteAllForUser(r.Context(), user.ScopeAuth, currentUser.ID); err != nil {
		h.logger.Println("ERROR: DeleteAllForUser failed:", err)
		httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
