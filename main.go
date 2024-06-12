package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/claycot/mlb-gameday-api/handlers"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// specify the type of logging to use
	l := log.New(os.Stdout, "mlb-gameday-api", log.LstdFlags)

	// load env (Hostname, Port, API url, CORS whitelist)
	err := godotenv.Load()
	if err != nil {
		l.Fatal("Error loading .env file")
	}

	// fetch and convert port from .env
	port, err := strconv.Atoi(os.Getenv("PORT_"))
	if err != nil {
		l.Fatal("Invalid PORT specified in environment")
	}

	// configure cors to use .env whitelist
	c := cors.New(cors.Options{
		AllowedOrigins: strings.Split(os.Getenv("ALLOWED_ORIGINS"), ","),
		AllowedMethods: []string{"GET"},
	})

	// create a new handler with the specified logger
	gh := handlers.NewGames(l)

	// set up routes with handler
	router := initializeRoutesGames(gh)

	// protect routes with CORS
	l.Println("Applying CORS configuration")
	handler := c.Handler(router)

	// create and configure a server
	addr := fmt.Sprintf("%s:%d", os.Getenv("HOSTNAME_"), port)
	s := &http.Server{
		Addr:         addr,
		Handler:      handler,
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
		l.Printf("Running server on %s", addr)
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
