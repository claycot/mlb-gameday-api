package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/claycot/mlb-gameday-api/handlers"
)

func main() {
	// specify the type of logging to use
	l := log.New(os.Stdout, "mlb-gameday-api", log.LstdFlags)

	// create a new handler with the specified logger
	gh := handlers.NewGames(l)

	// set up routes with handler
	router := initializeRoutesGames(gh)

	// create and configure a server
	s := &http.Server{
		Addr:         "localhost:3001",
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// expose the server
	go func() {
		err := s.ListenAndServe()
		if err != nil {
			l.Fatal(err)
		}
	}()

	// create a channel to hold OS signals (interrupt, shutdown)
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, os.Kill)

	// wait for a signal on that channel
	sig := <-sigChan
	l.Println("Received terminate, graceful shutdown.", sig)

	// graceful exit waits until active connections finish
	tc, _ := context.WithTimeout(context.Background(), 30*time.Second)
	s.Shutdown(tc)
}

func initializeRoutesGames(gh *handlers.Games) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/games/info/initial", gh.GetGames)
	return mux
}
