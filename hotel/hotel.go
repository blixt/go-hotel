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

	h.mu.RLock()
	room, exists := h.rooms[id]
	h.mu.RUnlock()

	if !exists {
		// TODO: The newRoom function will wait for initialization, meaning this
		// lock is held for way too long. We need to create a placeholder room
		// and then swap it out once initialization is done.
		h.mu.Lock()
		defer h.mu.Unlock()

		room, exists = h.rooms[id]
		if !exists {
			var err error
			room, err = newRoom(id, h.init, h.handler)
			if err != nil {
				return nil, err
			}
			h.rooms[id] = room
			go h.monitorRoom(room)
		}
	}

	return room, nil
}

func (h *Hotel[RoomMetadata, ClientMetadata, MessageType]) monitorRoom(room *Room[RoomMetadata, ClientMetadata, MessageType]) {
	<-room.ctx.Done()
	h.mu.Lock()
	delete(h.rooms, room.id)
	h.mu.Unlock()
}
