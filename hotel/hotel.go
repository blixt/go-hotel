package hotel

import (
	"errors"
	"sync"
)

type Hotel[RoomMetadata any, ClientMetadata any, MessageType any] struct {
	mu      sync.RWMutex
	rooms   map[string]*Room[RoomMetadata, ClientMetadata, MessageType]
	init    RoomInitFunc[RoomMetadata]
	handler RoomHandlerFunc[RoomMetadata, ClientMetadata, MessageType]
}

func New[RoomMetadata any, ClientMetadata any, MessageType any](init RoomInitFunc[RoomMetadata], handler RoomHandlerFunc[RoomMetadata, ClientMetadata, MessageType]) *Hotel[RoomMetadata, ClientMetadata, MessageType] {
	return &Hotel[RoomMetadata, ClientMetadata, MessageType]{
		rooms:   make(map[string]*Room[RoomMetadata, ClientMetadata, MessageType]),
		init:    init,
		handler: handler,
	}
}

func (h *Hotel[RoomMetadata, ClientMetadata, MessageType]) GetOrCreateRoom(id string) (*Room[RoomMetadata, ClientMetadata, MessageType], error) {
	if id == "" {
		return nil, errors.New("invalid room id: cannot be empty")
	}

	// If a room exists we only need a read lock to retrieve it.
	h.mu.RLock()
	room, exists := h.rooms[id]
	h.mu.RUnlock()

	if !exists {
		// A room might've been created in the short duration between RUnlock()
		// and this code so now we need a write lock where we only create the
		// room if it still doesn't exist.
		h.mu.Lock()
		room, exists = h.rooms[id]
		if !exists {
			room = newRoom(id, h.init, h.handler)
			h.rooms[id] = room
		}
		h.mu.Unlock()
	}

	// Wait for room init to run (or it might've already run in which case this
	// will immediately return nil).
	err := room.initGroup.Wait()

	if !exists {
		// This was the call that created the room, so do additional book
		// keeping once its init has finished and we know if it errored.
		if err != nil {
			h.mu.Lock()
			delete(h.rooms, id)
			h.mu.Unlock()
		} else {
			go func() {
				<-room.ctx.Done()
				h.mu.Lock()
				delete(h.rooms, room.id)
				h.mu.Unlock()
			}()
		}
	}

	if err != nil {
		return nil, err
	}

	return room, nil
}
