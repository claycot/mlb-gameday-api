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
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Println("Shutting down AuditGames worker")
			return
		case <-ticker.C:
			var wgInner sync.WaitGroup
			updated, removed, failed := gamesStore.Audit(&wgInner)
			wgInner.Wait()

			if len(updated) > 0 {
				logger.Printf("Updated games: %v", updated)
				update := &data.Games{
					Metadata: data.Metadata{
						Timestamp: time.Now(),
					},
					Data: make([]*data.Game, len(updated)),
				}
				for i, id := range updated {
					game, valid := gamesStore.Get(id)
					if valid {
						update.Data[i] = &game
					}
				}
				updateJson, err := update.ToJSON()
				if err == nil {
					updates <- handlers.Update{Event: "update", Data: string(updateJson)}
				} else {
					fmt.Println(err)
				}
			}
			if len(removed) > 0 {
				logger.Printf("REMOVE:%v", removed)
				updates <- handlers.Update{Event: "remove", Data: fmt.Sprintf("%v", removed)}
			}
			if len(failed) > 0 {
				logger.Printf("FAILED:%v", failed)
				updates <- handlers.Update{Event: "fail", Data: fmt.Sprintf("%v", failed)}
			}
		}
	}
}
