package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/data"
	"github.com/claycot/mlb-gameday-api/handlers"
)

// run the audit games function on the games store and send updates as SSE events
func AuditGames(ctx context.Context, gamesStore *data.GameCache, updates chan handlers.Update, logger *log.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	// update games every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
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
				remove := &data.GameIDs{
					Metadata: data.Metadata{
						Timestamp: time.Now(),
					},
					Data: make([]*uint32, len(removed)),
				}
				for i := range removed {
					remove.Data[i] = &removed[i]
				}
				// marshal to json and return
				updateJson, err := remove.ToJSON()
				if err != nil {
					logger.Printf("failed to marshal updates to json: %e\r\n", err)
				} else {
					updates <- handlers.Update{Event: "remove", Data: string(updateJson)}
				}
			}
			// process failed games by outputting their IDs
			if len(failed) > 0 {
				logger.Printf("FAILED:%v", failed)
				fail := &data.GameIDs{
					Metadata: data.Metadata{
						Timestamp: time.Now(),
					},
					Data: make([]*uint32, len(failed)),
				}
				for i := range failed {
					fail.Data[i] = &failed[i]
				}
				// marshal to json and return
				updateJson, err := fail.ToJSON()
				if err != nil {
					logger.Printf("failed to marshal updates to json: %e\r\n", err)
				} else {
					updates <- handlers.Update{Event: "fail", Data: string(updateJson)}
				}
			}
		}
	}
}
