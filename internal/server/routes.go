package server

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/claycot/mlb-gameday-api/data"
	"github.com/claycot/mlb-gameday-api/handlers"
	"github.com/claycot/mlb-gameday-api/internal/workers"
)

func Initialize(ctx context.Context, wg *sync.WaitGroup, logger *log.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	// Initialize game store and updates channel
	gamesStore := &data.GameCache{}
	updates := make(chan handlers.Update)
	broadcaster := handlers.NewBroadcaster()

	// use a broadcaster to send updates to all connected clients
	go func() {
		for {
			// Wait for messages
			msg, ok := <-updates
			if !ok {
				return // Channel closed, exit goroutine
			}

			countSent, err := broadcaster.Broadcast(&msg)

			if err != nil {
				logger.Printf("failed to send update: %e\r\n", err)
			}

			logger.Printf("sent update to %d of %d connected clients", countSent, broadcaster.Count)
		}
	}()

	// on context cancelation, wait for workers to finish and then close the channel
	go func() {
		<-ctx.Done()
		logger.Println("Context canceled, waiting for workers to finish...")

		wg.Wait()
		logger.Println("All workers done, closing updates channel")
		close(updates)
	}()

	// Start background workers
	wg.Add(2)
	go workers.AuditGames(ctx, gamesStore, updates, logger, wg)
	go workers.FindNewGames(ctx, gamesStore, updates, logger, wg)

	// Initialize handlers
	gh := handlers.NewGames(logger)

	// Define routes
	mux.HandleFunc("/api/games/initial", func(rw http.ResponseWriter, r *http.Request) {
		gh.GetInitial(rw, r, gamesStore)
	})
	mux.HandleFunc("/api/games/update", func(rw http.ResponseWriter, r *http.Request) {
		gh.GetUpdates(rw, r, updates, broadcaster)
	})

	return mux
}
