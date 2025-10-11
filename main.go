package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/tsatsarig/go-project/internal/app"
)

func main() {
	app, err := app.NewApplication()
	if err != nil {
		panic(err)
	}

	app.Logger.Printf("Starting application...")

	http.HandleFunc("/health", HealthCheck)

	server := &http.Server{
		Addr:         ":8080",
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	err = server.ListenAndServe()
	if err != nil {
		app.Logger.Fatalf("Error starting server: %v", err)
	}
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Health check endpoint hit\n")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
