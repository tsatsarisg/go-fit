package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/tsatsarisg/go-fit/internal/app"
)

func SetupRoutes(app *app.Application) *chi.Mux {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(app.Middleware.Authenticate)

		r.Get("/workouts/{id}", app.Middleware.RequireAuthenticatedUser(app.WorkoutHandler.HandleGetWorkoutByID))
		r.Post("/workouts", app.Middleware.RequireAuthenticatedUser(app.WorkoutHandler.HandleCreateWorkout))
		r.Put("/workouts/{id}", app.Middleware.RequireAuthenticatedUser(app.WorkoutHandler.HandleUpdateWorkout))
		r.Delete("/workouts/{id}", app.Middleware.RequireAuthenticatedUser(app.WorkoutHandler.HandleDeleteWorkout))
	})
	r.Get("/health", app.HealthCheck)
	r.Post("/users", app.UserHandler.HandleRegisterUser)
	r.Post("/tokens/authentication", app.TokenHandler.HandleCreateToken)

	return r
}
