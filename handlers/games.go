package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/claycot/mlb-gameday-api/data"
)

type Games struct {
	l *log.Logger
}

type Update struct {
	Event string
	Data  string
}

func NewGames(l *log.Logger) *Games {
	return &Games{l}
}

func (g *Games) GetInitial(rw http.ResponseWriter, r *http.Request, store *data.GameCache) {
	g.l.Println("Handle GET initial")

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

func (g *Games) GetUpdates(rw http.ResponseWriter, r *http.Request, updates chan Update) {
	g.l.Println("Handle GET updates")

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	g.l.Println("Starting event stream")
	for {
		select {
		case update := <-updates:
			g.l.Printf("Sending update: %s", update)
			fmt.Fprintf(rw, "event: %s\ndata: %s\n\n", update.Event, update.Data)
			flusher.Flush()
		case <-r.Context().Done():
			g.l.Printf("Connection closed! Reason: %v", r.Context().Err())
			return
		}
	}
}
