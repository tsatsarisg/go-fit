package app

import (
	"github.com/go-chi/chi/v5"
)

// Routes builds the HTTP router with all application routes wired up. It lives
// on *Application so callers (e.g. main) can set up the server with a single
// application.Routes() call after construction.
func (a *Application) Routes() *chi.Mux {
	r := chi.NewRouter()
	r.Use(a.Middleware.Authenticate)

	r.Get("/workouts/{id}", a.Middleware.RequireAuthenticatedUser(a.WorkoutHandler.HandleGetWorkoutByID))
	r.Post("/workouts", a.Middleware.RequireAuthenticatedUser(a.WorkoutHandler.HandleCreateWorkout))
	// PATCH — body is a partial-merge patch (nil fields = untouched), not
	// a full replacement, so PATCH is the correct verb per RFC 5789.
	r.Patch("/workouts/{id}", a.Middleware.RequireAuthenticatedUser(a.WorkoutHandler.HandleUpdateWorkout))
	r.Delete("/workouts/{id}", a.Middleware.RequireAuthenticatedUser(a.WorkoutHandler.HandleDeleteWorkout))

	r.Get("/health", a.HealthCheck)
	r.Post("/users", a.UserHandler.HandleRegisterUser)
	r.Post("/tokens/authentication", a.TokenHandler.HandleCreateToken)
	r.Post("/tokens/authentication/logout", a.Middleware.RequireAuthenticatedUser(a.TokenHandler.HandleLogout))

	return r
}
