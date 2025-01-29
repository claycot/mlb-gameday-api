package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/data"
	"github.com/claycot/mlb-gameday-api/handlers"
)

func FindNewGames(ctx context.Context, gamesStore *data.GameCache, updates chan handlers.Update, logger *log.Logger, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Println("Shutting down FindNewGames worker")
			return
		case <-ticker.C:
			// Fetch new games and update `gamesStore`
			var added []uint32
			gameIds, gameLinks, err := data.ListGamesByDate("04/26/2024")
			if err != nil {
				logger.Println("Added 0 games")
				continue
			}
			var wgInner sync.WaitGroup
			for i, id := range gameIds {
				_, valid := gamesStore.Get(id)
				if !valid {
					gamesStore.Fetch(gameLinks[i], &wgInner)
					added = append(added, id)
				}
			}
			wgInner.Wait()

			// notify channel if games were added
			if len(added) > 0 {
				logger.Printf("Added games: %v", added)
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
					updates <- handlers.Update{Event: "add", Data: string(addJson)}
				} else {
					fmt.Println(err)
				}
			} else {
				logger.Println("Added 0 games")
			}
		}
	}
}
