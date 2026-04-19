package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/tsatsarisg/go-fit/internal/httpx"
)

// ScopeAuth is the token scope used by the bearer-token authentication flow.
// Owned by the auth bounded context (moved here from the user package to
// break the user→auth→user cycle that blocked token-resolution from living
// in auth).
const ScopeAuth = "authentication"

type Middleware struct {
	Store Store
}

func NewMiddleware(store Store) *Middleware {
	return &Middleware{Store: store}
}

type contextKey string

const principalContextKey = contextKey("auth.principal")

// SetPrincipal returns a copy of r whose context carries p. Kept exported for
// tests that want to inject a principal without going through Authenticate.
func SetPrincipal(r *http.Request, p *Principal) *http.Request {
	ctx := context.WithValue(r.Context(), principalContextKey, p)
	return r.WithContext(ctx)
}

// GetPrincipal returns the Principal stored on the request context. It always
// returns a non-nil *Principal, falling back to AnonymousPrincipal when the
// request was never passed through Authenticate — callers just call
// IsAnonymous rather than nil-checking.
func GetPrincipal(r *http.Request) *Principal {
	p, ok := r.Context().Value(principalContextKey).(*Principal)
	if !ok || p == nil {
		return AnonymousPrincipal
	}
	return p
}

// Authenticate resolves the bearer token (if present) to a Principal and
// stashes it on the request. Missing / empty header ⇒ AnonymousPrincipal so
// public routes still work. Malformed header ⇒ 401 immediately.
func (mw *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Authorization")

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r = SetPrincipal(r, AnonymousPrincipal)
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Invalid Authorization header format"})
			return
		}

		principal, err := mw.Store.ResolvePrincipal(r.Context(), ScopeAuth, parts[1])
		if err != nil {
			// Infrastructure failure — don't leak details to the client.
			httpx.WriteJson(w, http.StatusInternalServerError, httpx.Envelope{"error": "internal error"})
			return
		}
		if principal == nil {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "Invalid token"})
			return
		}

		r = SetPrincipal(r, principal)
		next.ServeHTTP(w, r)
	})
}

// RequireAuthenticatedUser is a route-level guard for endpoints that demand a
// real principal. Public routes can skip it and read GetPrincipal directly.
func (mw *Middleware) RequireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetPrincipal(r).IsAnonymous() {
			httpx.WriteJson(w, http.StatusUnauthorized, httpx.Envelope{"error": "You must be authenticated to access this resource"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
