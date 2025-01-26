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
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/data"
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
		AllowedHeaders: []string{"Content-Type"},
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
		Addr:    addr,
		Handler: handler,
		// ReadTimeout:  5 * time.Second,
		// WriteTimeout: 10 * time.Second,
		// IdleTimeout:  120 * time.Second,
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
	l.Println("Received terminate. Shutting down.", sig)

	// graceful exit waits until active connections finish
	tc, _ := context.WithTimeout(context.Background(), 30*time.Second)
	s.Shutdown(tc)
}

func initializeRoutesGames(gh *handlers.Games) *http.ServeMux {
	mux := http.NewServeMux()

	// create a store for game information, so if 100000 users are online (standard load)
	// we only get 1 set of requests to the MLB API
	var (
		gamesStore *data.GameCache = &data.GameCache{}
		updates                    = make(chan string)
	)

	// goroutines to audit current games and find new ones
	go auditGames(gamesStore, updates)
	go findNewGames(gamesStore, updates)

	// pull signals from channel so if no users are online they don't get a stale message on connect
	go func() {
		for update := range updates {
			fmt.Println(update)
		}
	}()

	// initial game info
	mux.HandleFunc("GET /api/games/initial", func(rw http.ResponseWriter, r *http.Request) {
		gh.GetInitial(rw, r, gamesStore)
	})

	// game updates
	mux.HandleFunc("/api/games/update", func(rw http.ResponseWriter, r *http.Request) {
		req := r.WithContext(context.Background())

		gh.GetUpdates(rw, req, updates)
	})

	return mux
}

// update live games and prune old ones
func auditGames(gamesStore *data.GameCache, updates chan string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var wg sync.WaitGroup
		updated, removed, failed := gamesStore.Audit(&wg)
		wg.Wait()
		// process updated games
		if len(updated) > 0 {
			update := &data.Games{
				Metadata: data.Metadata{
					Timestamp: time.Now(),
				},
				Data: make([]*data.Game, len(updated)),
			}
			log.Printf("Updated games: %v", updated)
			for i, id := range updated {
				game, valid := gamesStore.Get(id)
				if valid {
					update.Data[i] = &game
				}
			}
			updateJson, err := update.ToJSON()
			if err == nil {
				updates <- fmt.Sprintf("Updated games: %s,", updateJson)
			} else {
				fmt.Println(err)
			}
		}
		// process removed games
		if len(removed) > 0 {
			log.Printf("Removed games: %v", removed)
		}
		// process failed games
		if len(failed) > 0 {
			log.Printf("Failed games: %v", failed)
		}
	}
}

// find new games (usually on changed date)
func findNewGames(gamesStore *data.GameCache, updates chan string) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var added []uint32
		gameIds, gameLinks, err := data.ListGamesByDate("")
		if err != nil {
			log.Println("Added 0 games")
			updates <- fmt.Sprint("Added no games")
			continue
		}
		var wg sync.WaitGroup
		for i, id := range gameIds {
			_, valid := gamesStore.Get(id)
			if !valid {
				gamesStore.Fetch(gameLinks[i], &wg)
				added = append(added, id)
			}
		}
		wg.Wait()

		// notify channel if games were added
		if len(added) > 0 {
			log.Printf("Added games: %v", added)
			add := &data.Games{
				Metadata: data.Metadata{
					Timestamp: time.Now(),
				},
				Data: make([]*data.Game, len(added)),
			}
			for i, id := range added {
				game, valid := gamesStore.Get(id)
				if valid {
					add.Data[i] = &game
				}
			}
			addJson, err := add.ToJSON()
			if err == nil {
				updates <- fmt.Sprintf("Added games: %s,", addJson)
			} else {
				fmt.Println(err)
			}
		} else {
			log.Println("Added 0 games")
			updates <- fmt.Sprint("Added no games")
		}
	}
}
