package hotel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type RoomInitFunc[RoomMetadata any] func(id string) (metadata *RoomMetadata, err error)

type RoomHandlerFunc[RoomMetadata, ClientMetadata, DataType any] func(ctx context.Context, room *Room[RoomMetadata, ClientMetadata, DataType])

type Room[RoomMetadata, ClientMetadata, DataType any] struct {
	initGroup errgroup.Group

	id           string
	metadata     *RoomMetadata
	clients      map[*Client[ClientMetadata, DataType]]struct{}
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	eventsCh     chan Event[ClientMetadata, DataType]
	closeTimer   *time.Timer
	closeTimerMu sync.Mutex
}

// TODO: This should be configurable on either a per-room or global basis.
const DefaultAutoCloseDelay = 2 * time.Minute

func newRoom[RoomMetadata, ClientMetadata, DataType any](id string, init RoomInitFunc[RoomMetadata], handler RoomHandlerFunc[RoomMetadata, ClientMetadata, DataType]) *Room[RoomMetadata, ClientMetadata, DataType] {
	ctx, cancel := context.WithCancel(context.Background())
	eventsCh := make(chan Event[ClientMetadata, DataType], 1024)
	room := &Room[RoomMetadata, ClientMetadata, DataType]{
		id:       id,
		clients:  make(map[*Client[ClientMetadata, DataType]]struct{}),
		ctx:      ctx,
		cancel:   cancel,
		eventsCh: eventsCh,
	}
	room.initGroup.Go(func() error {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("Room %s init panicked: %v\n%s", room.id, err, buf)
				room.Close()
			}
		}()

		metadata, err := init(id)
		if err != nil {
			return err
		}
		room.metadata = metadata

		go func() {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					log.Printf("Room %s handler panicked: %v\n%s", room.id, err, buf)
				}
				room.Close()
			}()
			handler(ctx, room)
		}()
		return nil
	})
	return room
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) ID() string {
	return r.id
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Events() <-chan Event[ClientMetadata, DataType] {
	return r.eventsCh
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Metadata() *RoomMetadata {
	return r.metadata
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) NewClient(metadata *ClientMetadata) (*Client[ClientMetadata, DataType], error) {
	r.mu.Lock()
	select {
	case <-r.ctx.Done():
		r.mu.Unlock()
		return nil, errors.New("cannot add client: room is closed")
	default:
		// Cancel any pending close timer
		r.cancelCloseTimer()

		client := newClient[ClientMetadata, DataType](metadata)
		newClients := make(map[*Client[ClientMetadata, DataType]]struct{}, len(r.clients)+1)
		for c := range r.clients {
			newClients[c] = struct{}{}
		}
		newClients[client] = struct{}{}
		r.clients = newClients
		r.mu.Unlock()
		r.Emit(Event[ClientMetadata, DataType]{
			Type:   EventJoin,
			Client: client,
		})
		return client, nil
	}
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) RemoveClient(client *Client[ClientMetadata, DataType]) error {
	r.mu.Lock()
	if _, exists := r.clients[client]; !exists {
		r.mu.Unlock()
		return fmt.Errorf("client not found")
	}
	newClients := make(map[*Client[ClientMetadata, DataType]]struct{}, len(r.clients)-1)
	for c := range r.clients {
		if c != client {
			newClients[c] = struct{}{}
		}
	}
	r.clients = newClients
	isEmpty := len(newClients) == 0
	r.mu.Unlock()

	r.Emit(Event[ClientMetadata, DataType]{
		Type:   EventLeave,
		Client: client,
	})
	client.Close()

	// Schedule room closure if empty
	if isEmpty {
		r.scheduleClose()
	}
	return nil
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Emit(event Event[ClientMetadata, DataType]) {
	select {
	case r.eventsCh <- event:
	default:
		log.Printf("Warning: Room %s events channel is full. Cannot send %s. Closing room.", r.id, event.Type)
		r.Close()
	}
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) HandleClientData(client *Client[ClientMetadata, DataType], data DataType) error {
	r.mu.RLock()
	_, exists := r.clients[client]
	r.mu.RUnlock()
	if !exists {
		return fmt.Errorf("client not found")
	}
	r.Emit(Event[ClientMetadata, DataType]{
		Type:   EventCustom,
		Client: client,
		Data:   data,
	})
	return nil
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) SendToClient(client *Client[ClientMetadata, DataType], data DataType) error {
	r.mu.RLock()
	_, exists := r.clients[client]
	r.mu.RUnlock()
	if !exists {
		return fmt.Errorf("client not found")
	}
	if err := client.send(data); err != nil {
		r.RemoveClient(client)
		return fmt.Errorf("failed to send data: %w", err)
	}
	return nil
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Broadcast(data DataType) {
	r.mu.RLock()
	clients := r.clients
	r.mu.RUnlock()
	for client := range clients {
		if err := client.send(data); err != nil {
			r.RemoveClient(client)
			log.Printf("Failed to send data to client %p: %v", client, err)
		}
	}
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) BroadcastExcept(except *Client[ClientMetadata, DataType], data DataType) {
	r.mu.RLock()
	clients := r.clients
	r.mu.RUnlock()
	for client := range clients {
		if client != except {
			if err := client.send(data); err != nil {
				r.RemoveClient(client)
				log.Printf("Failed to send data to client %p: %v", client, err)
			}
		}
	}
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Close() {
	r.cancelCloseTimer()
	r.cancel()
	r.mu.Lock()
	for client := range r.clients {
		client.Close()
	}
	r.clients = nil
	r.mu.Unlock()
	// TODO: Figure out if/when we should close the events channel. Close() is
	// public and so are methods writing to the channel, so it's very difficult
	// to prove that writes and close happen on the same goroutine.
	// close(r.eventsCh)
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) FindClient(predicate func(*ClientMetadata) bool) *Client[ClientMetadata, DataType] {
	r.mu.RLock()
	clients := r.clients
	r.mu.RUnlock()
	for client := range clients {
		if predicate(client.metadata) {
			return client
		}
	}
	return nil
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) Clients() []*Client[ClientMetadata, DataType] {
	r.mu.RLock()
	clients := r.clients
	r.mu.RUnlock()
	clientsSlice := make([]*Client[ClientMetadata, DataType], 0, len(r.clients))
	for client := range clients {
		clientsSlice = append(clientsSlice, client)
	}
	return clientsSlice
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) scheduleClose() {
	r.closeTimerMu.Lock()
	defer r.closeTimerMu.Unlock()

	if r.closeTimer != nil {
		r.closeTimer.Stop()
	}
	r.closeTimer = time.AfterFunc(DefaultAutoCloseDelay, func() {
		r.mu.RLock()
		isEmpty := len(r.clients) == 0
		r.mu.RUnlock()

		if isEmpty {
			r.Close()
		}
	})
}

func (r *Room[RoomMetadata, ClientMetadata, DataType]) cancelCloseTimer() {
	r.closeTimerMu.Lock()
	defer r.closeTimerMu.Unlock()

	if r.closeTimer != nil {
		r.closeTimer.Stop()
		r.closeTimer = nil
	}
}
