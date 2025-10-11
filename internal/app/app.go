package app

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tsatsarig/go-project/internal/api"
	"github.com/tsatsarig/go-project/internal/store"
)

type Application struct {
	Logger         *log.Logger
	WorkoutHandler *api.WorkoutHandler
	DB             *sql.DB
}

func NewApplication() (*Application, error) {
	pgDB, err := store.Open()
	if err != nil {
		return nil, err
	}
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// stores

	// handlers
	workoutHandler := api.NewWorkoutHandler()

	app := &Application{
		Logger:         logger,
		WorkoutHandler: workoutHandler,
		DB:             pgDB,
	}

	return app, nil
}

func (a *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(a.Logger.Writer(), "Health check endpoint hit\n")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
