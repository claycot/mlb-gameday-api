package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/data"
	"github.com/claycot/mlb-gameday-api/handlers"
)

// fetch new games and update gamesStore
func FindNewGames(ctx context.Context, gamesStore *data.GameCache, updates chan handlers.Update, logger *log.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	// update games every 15 minutes
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// run immediately on creation
	logger.Println("[INFO] FindNewGames: running initial fetch")
	updateGames(ctx, gamesStore, updates, logger)

	for {
		select {
		// if context is canceled, shut down the worker
		case <-ctx.Done():
			logger.Println("[INFO] Shutting down FindNewGames worker")
			return
		// on each tick, fetch new games, add them to game store, and retrieve their info
		case <-ticker.C:
			logger.Println("[INFO] FindNewGames: finding new games")
			updateGames(ctx, gamesStore, updates, logger)
		}
	}
}

func updateGames(ctx context.Context, gamesStore *data.GameCache, updates chan handlers.Update, logger *log.Logger) {
	var added []uint32
	// fetch a list of all games on today's date and their links
	gameIds, gameLinks, err := data.ListGamesByDate(ctx, logger, "")
	if err != nil {
		logger.Printf("[ERROR] Added 0 games: %v\r\n", err)
		return
	}

	// add new games to the cache
	for i, id := range gameIds {
		// if !discovered, game already existed or cache is full (full cache throws err)
		discovered, err := gamesStore.Discover(id, gameLinks[i])

		// cache may be full
		// TODO: handle this error more smarter
		if err != nil {
			return
		}

		// if the game is new, queue it for fetching
		if discovered {
			added = append(added, id)
		}
	}

	// if games were added, update their information and notify channel
	if len(added) > 0 {
		logger.Printf("[INFO] Added games: %v", added)
		add := &data.Games{
			Metadata: data.Metadata{
				Timestamp: time.Now(),
			},
			Data: make([]*data.Game, len(added)),
		}

		// fetch information on new games
		var wgGameInfo sync.WaitGroup
		for i, id := range added {
			wgGameInfo.Add(1)
			go func(writeIndex int, gameId uint32) {
				defer wgGameInfo.Done()
				game, valid := gamesStore.GetOne(ctx, gameId)
				if valid {
					add.Data[writeIndex] = &game
				} else {
					logger.Printf("[ERROR] Failed to get information on game %d", gameId)
				}
			}(i, id)
		}
		wgGameInfo.Wait()

		// marshal into json and send
		addJson, err := add.ToJSON()
		if err == nil {
			updates <- handlers.Update{Event: "add", Data: string(addJson)}
		} else {
			logger.Printf("[ERROR] Failed to marshal add to json: %v\r\n", err)
		}
	} else {
		logger.Println("[INFO] Added 0 games")
	}
}
