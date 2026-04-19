package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tsatsarisg/go-fit/internal/auth"
	"github.com/tsatsarisg/go-fit/internal/config"
	"github.com/tsatsarisg/go-fit/internal/httpx"
	"github.com/tsatsarisg/go-fit/internal/platform/postgres"
	"github.com/tsatsarisg/go-fit/internal/user"
	"github.com/tsatsarisg/go-fit/internal/workout"
	"github.com/tsatsarisg/go-fit/migrations"
)

type Application struct {
	Logger         *log.Logger
	Config         *config.Config
	WorkoutHandler *workout.Handler
	UserHandler    *user.Handler
	TokenHandler   *auth.Handler
	Middleware     *user.Middleware
	DB             *sql.DB
}

// NewApplication wires up the application's dependencies. The provided ctx is
// used to bound potentially slow startup operations such as the initial DB
// ping. Migration errors are returned rather than panicked so callers can
// react and shut down cleanly.
func NewApplication(ctx context.Context, cfg *config.Config) (*Application, error) {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	// Pretty-print JSON in development for readability; compact in
	// production to minimize payload size and CPU.
	httpx.SetPrettyJSON(!cfg.IsProduction())

	openCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pgDB, err := postgres.Open(openCtx, cfg.DatabaseURL, logger)
	if err != nil {
		return nil, err
	}

	if err := postgres.MigrateFS(pgDB, migrations.FS, "."); err != nil {
		_ = pgDB.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	logger.Println("Database migrated successfully")

	// stores
	workoutStore := workout.NewPostgresStore(pgDB)
	userStore := user.NewPostgresStore(pgDB)
	tokenStore := auth.NewPostgresStore(pgDB)

	// handlers
	workoutHandler := workout.NewHandler(workoutStore, logger)
	userHandler := user.NewHandler(userStore, logger)
	tokenHandler := auth.NewHandler(tokenStore, userStore, logger)

	// middleware
	userMiddleware := &user.Middleware{
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
