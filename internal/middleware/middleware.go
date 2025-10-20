package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/tsatsarisg/go-fit/internal/store"
	"github.com/tsatsarisg/go-fit/internal/tokens"
	"github.com/tsatsarisg/go-fit/internal/utils"
)

type UserMiddleware struct {
	UserStore store.UserStore
}

type contextKey string

const userContextKey = contextKey("user")

func SetUser(r *http.Request, user *store.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func GetUser(r *http.Request) *store.User {
	user, ok := r.Context().Value(userContextKey).(*store.User)
	if !ok {
		panic("user not found in context")
	}
	return user
}

func (mw *UserMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r = SetUser(r, store.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Invalid Authorization header format"})
			return
		}

		tokenString := headerParts[1]
		user, err := mw.UserStore.GetUserToken(tokens.ScopeAuth, tokenString)
		if err != nil {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "Invalid token"})
			return
		}
		if user == nil {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "User not found"})
			return
		}
		r = SetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (mw *UserMiddleware) RequireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user.IsAnonymous() {
			utils.WriteJson(w, http.StatusUnauthorized, utils.Envelope{"error": "You must be authenticated to access this resource"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
