package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/claycot/mlb-gameday-api/data"
)

type Games struct {
	logger *log.Logger
}

type Update struct {
	Event string
	Data  string
}

func NewGames(l *log.Logger) *Games {
	return &Games{l}
}

// handler for when a user first visits and the existing games should be ready on page load
func (g *Games) GetInitial(rw http.ResponseWriter, r *http.Request, store *data.GameCache) {
	g.logger.Println("[INFO] GET initial called")

	gameList, err := data.GetInitialGames(store)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to fetch games: %s", err), http.StatusBadGateway)
		return
	}

	games, err := gameList.ToJSON()
	if err != nil {
		http.Error(rw, "Unable to marshal JSON", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write(games)
}

// handler for SSE updates to the games on the site
func (g *Games) GetUpdates(rw http.ResponseWriter, r *http.Request, broadcaster *Broadcaster) {
	g.logger.Println("[INFO] GET updates called")

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	// make a channel to send SSE updates to the user
	userChannel := make(chan *Update, 16)
	chanId, err := broadcaster.Register(userChannel, g.logger)

	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to create channel: %s", err), http.StatusInternalServerError)
		return
	}
	defer broadcaster.Deregister(chanId, g.logger)

	// flush messages to the updates channel
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// keep alive timer
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// send updates to the channel
	// g.logger.Println("[INFO] Starting event stream")
	for {
		select {
		case update := <-userChannel:
			// g.logger.Printf("[INFO] Sending update: %s", update)
			fmt.Fprintf(rw, "event: %s\ndata: %s\n\n", update.Event, update.Data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(rw, "event: %s\ndata: %s\n\n", "keep-alive", " ")
			flusher.Flush()
		case <-r.Context().Done():
			g.logger.Printf("[INFO] Connection %v closed! Reason: %v", chanId, r.Context().Err())
			return
		}
	}
}
