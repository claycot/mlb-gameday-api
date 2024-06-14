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

func NewGames(l *log.Logger) *Games {
	return &Games{l}
}

func (g *Games) GetGames(rw http.ResponseWriter, r *http.Request) {
	g.l.Println("Handle GET games")

	gameList, err := data.GetCachedGames("")
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
