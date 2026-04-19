package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tsatsarisg/go-fit/internal/api"
	"github.com/tsatsarisg/go-fit/internal/config"
	"github.com/tsatsarisg/go-fit/internal/middleware"
	"github.com/tsatsarisg/go-fit/internal/store"
	"github.com/tsatsarisg/go-fit/migrations"
)

type Application struct {
	Logger         *log.Logger
	Config         *config.Config
	WorkoutHandler *api.WorkoutHandler
	UserHandler    *api.UserHandler
	TokenHandler   *api.TokenHandler
	Middleware     *middleware.UserMiddleware
	DB             *sql.DB
}

// NewApplication wires up the application's dependencies. The provided ctx is
// used to bound potentially slow startup operations such as the initial DB
// ping. Migration errors are returned rather than panicked so callers can
// react and shut down cleanly.
func NewApplication(ctx context.Context, cfg *config.Config) (*Application, error) {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	openCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pgDB, err := store.Open(openCtx, cfg.DatabaseURL, logger)
	if err != nil {
		return nil, err
	}

	if err := store.MigrateFS(pgDB, migrations.FS, "."); err != nil {
		_ = pgDB.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	logger.Println("Database migrated successfully")

	// stores
	workoutStore := store.NewPostgresWorkoutStore(pgDB)
	userStore := store.NewPostgresUserStore(pgDB)
	tokenStore := store.NewPostgresTokensStore(pgDB)

	// handlers
	workoutHandler := api.NewWorkoutHandler(workoutStore, logger)
	userHandler := api.NewUserHandler(userStore, logger)
	tokenHandler := api.NewTokenHandler(tokenStore, userStore, logger)

	// middleware
	userMiddleware := &middleware.UserMiddleware{
		UserStore: userStore,
	}

	app := &Application{
		Logger:         logger,
		Config:         cfg,
		WorkoutHandler: workoutHandler,
		UserHandler:    userHandler,
		TokenHandler:   tokenHandler,
		Middleware:     userMiddleware,
		DB:             pgDB,
	}

	return app, nil
}

func (a *Application) HealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(a.Logger.Writer(), "Health check endpoint hit\n")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
