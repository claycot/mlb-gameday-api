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

func (g *Games) GetUpdates(rw http.ResponseWriter, r *http.Request, broadcaster *Broadcaster) {
	g.l.Println("Handle GET updates")

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")

	// make a channel to send SSE updates to the user
	userChannel := make(chan *Update, 16)
	chanId, err := broadcaster.Register(userChannel)

	if err != nil {
		http.Error(rw, fmt.Sprintf("Unable to create channel: %s", err), http.StatusInternalServerError)
		return
	}
	defer broadcaster.Deregister(chanId)

	// flush messages to the updates channel
	flusher, ok := rw.(http.Flusher)
	if !ok {
		http.Error(rw, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// send updates to the channel
	g.l.Println("Starting event stream")
	for {
		select {
		case update := <-userChannel:
			g.l.Printf("Sending update: %s", update)
			fmt.Fprintf(rw, "event: %s\ndata: %s\n\n", update.Event, update.Data)
			flusher.Flush()
		case <-r.Context().Done():
			g.l.Printf("Connection closed! Reason: %v", r.Context().Err())
			return
		}
	}
}
