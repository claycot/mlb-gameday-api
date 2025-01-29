package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/claycot/mlb-gameday-api/internal/config"
	"github.com/claycot/mlb-gameday-api/internal/server"
)

func main() {
	// Initialize logger
	logger := log.New(os.Stdout, "mlb-gameday-api", log.LstdFlags)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Error loading configuration: ", err)
	}

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// WaitGroup to manage goroutines
	var wg sync.WaitGroup

	// Capture termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	go func() {
		<-sigChan
		logger.Println("Received terminate signal")
		cancel()
	}()

	// Initialize and start the server
	srv, err := server.New(ctx, &wg, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to start server: ", err)
	}

	// Run the server in a separate goroutine
	go func() {
		if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: ", err)
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	logger.Println("All workers completed, exiting application.")
}
