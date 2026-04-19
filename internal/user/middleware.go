package user

import (
	"context"
	"net/http"
	"strings"

	"github.com/tsatsarisg/go-fit/internal/httpx"
)

// ScopeAuth is the token scope used by the bearer-token authentication flow.
// It's defined in the user package (not auth) because middleware here needs
// to reference it when resolving a bearer token, and user cannot import auth
// without creating a cycle (auth imports user for login/token creation).
const ScopeAuth = "authentication"

type Middleware struct {
	UserStore Store
}

type contextKey string

const userContextKey = contextKey("user")

func SetUser(r *http.Request, user *User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func GetUser(r *http.Request) *User {
	user, ok := r.Context().Value(userContextKey).(*User)
	if !ok {
		return AnonymousUser
	}
	return user
}

func (mw *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r = SetUser(r, AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Invalid Authorization header format"})
			return
		}

		tokenString := headerParts[1]
		user, err := mw.UserStore.GetUserToken(r.Context(), ScopeAuth, tokenString)
		if err != nil {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Invalid token"})
			return
		}
		if user == nil {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "User not found"})
			return
		}
		r = SetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (mw *Middleware) RequireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user.IsAnonymous() {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "You must be authenticated to access this resource"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
