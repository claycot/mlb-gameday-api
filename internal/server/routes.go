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

	// Start background workers
	go workers.AuditGames(ctx, gamesStore, updates, logger, wg)
	go workers.FindNewGames(ctx, gamesStore, updates, logger, wg)

	// Initialize handlers
	gh := handlers.NewGames(logger)

	// Define routes
	mux.HandleFunc("/api/games/initial", func(rw http.ResponseWriter, r *http.Request) {
		gh.GetInitial(rw, r, gamesStore)
	})
	mux.HandleFunc("/api/games/update", func(rw http.ResponseWriter, r *http.Request) {
		gh.GetUpdates(rw, r, updates)
	})

	return mux
}
