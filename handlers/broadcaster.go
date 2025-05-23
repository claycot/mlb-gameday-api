package handlers

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

type Broadcaster struct {
	clients sync.Map
	Count   int32
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{}
}

// register a client's channel to the broadcaster and return their uuid
func (b *Broadcaster) Register(channel chan *Update, logger *log.Logger) (uuid.UUID, error) {
	var id uuid.UUID
	var err error
	exists := true

	// avoid uuid collisions even though they'll never happen
	for exists {
		id, err = uuid.NewRandom()

		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to generate UUID: %w", err)
		}
		_, exists = b.clients.Load(id)
	}

	// store the channel in the map
	b.clients.Store(id, channel)
	atomic.AddInt32(&b.Count, 1)

	logger.Printf("[INFO] Registered client with ID %v. Now serving %d clients\r\n", id, b.Count)

	return id, nil
}

// deregister a client's channel from the broadcaster and delete all references
func (b *Broadcaster) Deregister(clientId uuid.UUID, logger *log.Logger) (bool, error) {
	// attempt to load the client's channel from the broadcaster
	channelRaw, exists := b.clients.Load(clientId)

	// if it doesn't exist, return an err
	if !exists {
		return false, fmt.Errorf("client with given ID did not exist: %s", clientId)
	}

	// otherwise, close the channel
	channel := channelRaw.(chan *Update)
	close(channel)

	// delete the client from the broadcaster and decrement the counter
	b.clients.Delete(clientId)
	atomic.AddInt32(&b.Count, -1)

	logger.Printf("[INFO] Deregistered client with ID %v. Now serving %d clients\r\n", clientId, b.Count)

	return true, nil
}

// broadcast an update to all clients
func (b *Broadcaster) Broadcast(message *Update, logger *log.Logger) (int, error) {
	i := 0
	b.clients.Range(func(key, value interface{}) bool {
		channel, ok := value.(chan *Update)
		if !ok {
			logger.Printf("[ERROR] Client %s has an invalid channel type", key)
			return true
		}

		select {
		case channel <- message:
			i++
		default:
			logger.Printf("[ERROR] Dropping message for client %s: channel is full", key)
		}

		return true
	})

	// if i == 0 {
	// 	return 0, fmt.Errorf("no active clients to broadcast to")
	// }

	return i, nil
}
