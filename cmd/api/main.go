package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tsatsarisg/go-fit/internal/app"
	"github.com/tsatsarisg/go-fit/internal/config"
)

func main() {
	if err := run(); err != nil {
		// log.Printf (not log.Fatal) so deferred Close() runs before exit.
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

	// Root context cancels on SIGINT/SIGTERM — Application.Run observes this
	// to trigger graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init application: %w", err)
	}
	defer func() {
		if cerr := application.Close(); cerr != nil {
			log.Printf("close application: %v", cerr)
		}
	}()

	return application.Run(ctx)
}
