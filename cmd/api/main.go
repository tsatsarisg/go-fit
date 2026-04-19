package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsatsarisg/go-fit/internal/app"
	"github.com/tsatsarisg/go-fit/internal/config"
)

func main() {
	if err := run(); err != nil {
		// Use log.Print (not log.Fatal) so any deferred work in run() has
		// already executed by the time we exit non-zero here.
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	var portFlag int
	flag.IntVar(&portFlag, "port", 0, "Port to run the server on (overrides PORT env)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if portFlag != 0 {
		cfg.Port = portFlag
	}

	// Root context cancels on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.NewApplication(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init application: %w", err)
	}
	defer func() {
		application.Logger.Println("Closing database connection...")
		if cerr := application.DB.Close(); cerr != nil {
			application.Logger.Printf("error closing database: %v", cerr)
		}
	}()

	application.Logger.Printf("Starting application on :%d (env=%s)...", cfg.Port, cfg.Env)

	r := application.Routes()
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if serr := server.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			serverErrCh <- serr
			return
		}
		serverErrCh <- nil
	}()

	select {
	case serr := <-serverErrCh:
		if serr != nil {
			return fmt.Errorf("server error: %w", serr)
		}
		return nil

	case <-ctx.Done():
		application.Logger.Println("Shutdown signal received, stopping server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if serr := server.Shutdown(shutdownCtx); serr != nil {
			application.Logger.Printf("graceful shutdown failed: %v", serr)
			// Force close so we don't block exit.
			if cerr := server.Close(); cerr != nil {
				application.Logger.Printf("forced server close failed: %v", cerr)
			}
			return fmt.Errorf("server shutdown: %w", serr)
		}

		application.Logger.Println("Server stopped cleanly")
		return nil
	}
}
