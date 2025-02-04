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
	// initialize logger
	logger := log.New(os.Stdout, "mlb-gameday-api", log.LstdFlags)

	// load config from .env file, or defaults if no file is provided
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Error loading configuration: ", err)
	}

	// create context to be used throughout requests
	ctx, cancel := context.WithCancel(context.Background())

	// capture termination signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, os.Kill)
	go func() {
		<-sigChan
		logger.Println("Received terminate signal")
		cancel()
	}()

	// waitGroup to manage goroutine daemons
	var wg sync.WaitGroup

	// initialize and start the server
	srv, err := server.New(ctx, &wg, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to start server: ", err)
	}

	// run the server in a separate goroutine
	go func() {
		if err := srv.Run(ctx); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: ", err)
		}
	}()

	// wait for all workers to complete
	wg.Wait()
	logger.Println("All workers completed, exiting application.")
}
