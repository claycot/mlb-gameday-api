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

func AuditGames(ctx context.Context, gamesStore *data.GameCache, updates chan handlers.Update, logger *log.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	// update games every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		// if context is canceled, shut down the worker
		case <-ctx.Done():
			logger.Println("Shutting down AuditGames worker")
			return
		// on each tick, audit the games store
		case <-ticker.C:
			updated, removed, failed := gamesStore.Audit(ctx)

			// process updated games by pulling the new information
			if len(updated) > 0 {
				logger.Printf("Updated games: %v", updated)
				// create a wrapper for the games
				update := &data.Games{
					Metadata: data.Metadata{
						Timestamp: time.Now(),
					},
					Data: make([]*data.Game, len(updated)),
				}

				// retrieve and set information for each game
				var wgGetGames sync.WaitGroup
				for i, id := range updated {
					wgGetGames.Add(1)
					go func(writeIndex int, gameId uint32) {
						defer wgGetGames.Done()
						game, valid := gamesStore.GetOne(ctx, gameId)
						if valid {
							update.Data[writeIndex] = &game
						}
						// TODO: what about errors? uncertain 😇
					}(i, id)
				}
				wgGetGames.Wait()

				// marshal to json and return
				updateJson, err := update.ToJSON()
				if err != nil {
					logger.Printf("failed to marshal updates to json: %e\r\n", err)
				} else {
					updates <- handlers.Update{Event: "update", Data: string(updateJson)}
				}
			}
			// process removed games by outputting their IDs
			if len(removed) > 0 {
				logger.Printf("REMOVE:%v", removed)
				updates <- handlers.Update{Event: "remove", Data: fmt.Sprintf("%v", removed)}
			}
			// process failed games by outputting their IDs
			if len(failed) > 0 {
				logger.Printf("FAILED:%v", failed)
				updates <- handlers.Update{Event: "fail", Data: fmt.Sprintf("%v", failed)}
			}
		}
	}
}
