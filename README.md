# go-hotel

A lightweight, flexible framework for building real-time room-based applications in Go. The package provides a type-safe way to manage virtual rooms where clients can connect, communicate, and exchange messages.

## Features

- **Type-safe with Go generics**: Define your own custom metadata and message types
- **Room-based architecture**: Clients join specific rooms for isolated communication
- **Event-driven design**: React to clients joining, leaving, and sending messages
- **Automatic room lifecycle management**: Rooms are automatically cleaned up when empty
- **Graceful handling of disconnections**: Automatically manages client connection lifecycles

## Installation

```bash
go get github.com/blixt/go-hotel
```

## Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/blixt/go-hotel/hotel"
    "github.com/gorilla/websocket"
)

// Define your metadata types
type RoomMetadata struct {
    CreatedAt time.Time
}

type ClientMetadata struct {
    Username string
}

// Define message types
type ChatMessage struct {
    Content string
}

func main() {
    // Create a new hotel
    h := hotel.New(
        // Room initialization function
        func(ctx context.Context, id string) (*RoomMetadata, error) {
            return &RoomMetadata{CreatedAt: time.Now()}, nil
        },
        // Room handler function
        func(ctx context.Context, room *hotel.Room[RoomMetadata, ClientMetadata, ChatMessage]) {
            // Handle room events
            for {
                select {
                case <-ctx.Done():
                    return
                case event := <-room.Events():
                    switch event.Type {
                    case hotel.EventJoin:
                        // Notify everyone about the new client
                        username := event.Client.Metadata().Username
                        log.Printf("User %s joined room %s", username, room.ID())
                        // Broadcast a welcome message
                        room.BroadcastExcept(event.Client, ChatMessage{
                            Content: fmt.Sprintf("User %s joined the chat", username),
                        })
                    case hotel.EventLeave:
                        username := event.Client.Metadata().Username
                        log.Printf("User %s left room %s", username, room.ID())
                        room.Broadcast(ChatMessage{
                            Content: fmt.Sprintf("User %s left the chat", username),
                        })
                    case hotel.EventCustom:
                        // Handle custom message
                        msg := event.Data
                        log.Printf("Message in room %s: %s", room.ID(), msg.Content)
                        // Broadcast the message to everyone
                        room.Broadcast(msg)
                    }
                }
            }
        },
    )

    // Set up your HTTP handlers for WebSocket connections...
}
```

## Key Concepts

### Hotel

The `Hotel` is a container that manages all rooms. It handles room creation, retrieval, and cleanup.

### Room

A `Room` represents a virtual space where clients can connect and interact. Each room has:

- A unique ID
- Custom metadata (defined by you)
- A collection of connected clients
- An event channel for room events

### Client

A `Client` represents a connection to a room. It handles:

- Bidirectional communication
- Message buffering
- Connection lifecycle management

### Events

The system is event-driven, with built-in events:

- `EventJoin`: When a client joins a room
- `EventLeave`: When a client leaves a room
- `EventCustom`: When a client sends data to the room

## Advanced Usage

### Finding Clients

```go
// Find a client by username
client := room.FindClient(func(metadata *ClientMetadata) bool {
    return metadata.Username == "john"
})
```

### Targeted Messages

```go
// Send a message to a specific client
room.SendToClient(client, ChatMessage{Content: "Private message"})
```

### Broadcasting

```go
// Send a message to all clients in the room
room.Broadcast(ChatMessage{Content: "Announcement to everyone"})

// Send a message to all clients except the sender
room.BroadcastExcept(sender, ChatMessage{Content: "Message from sender"})
```

## Advanced Behaviors

### Handling Long-Running Room Initialization

The room initialization function is executed asynchronously when a room is first created. This means you can perform time-consuming tasks without blocking:

```go
h := hotel.New(
    // Room init function that performs expensive operations
    func(ctx context.Context, id string) (*RoomMetadata, error) {
        // Load data from a database
        data, err := loadFromDatabase(id)
        if err != nil {
            return nil, fmt.Errorf("failed to load room data: %w", err)
        }
        
        // Process the data (potentially time-consuming)
        processedData, err := processData(data)
        if err != nil {
            return nil, fmt.Errorf("failed to process room data: %w", err)
        }
        
        return &RoomMetadata{
            CreatedAt: time.Now(),
            Data: processedData,
        }, nil
    },
    roomHandlerFunc,
)
```

When calling `GetOrCreateRoom()`, the hotel will wait for initialization to complete before returning the room:

```go
room, err := hotel.GetOrCreateRoom("room-id")
if err != nil {
    // Handle initialization error
    log.Printf("Failed to create room: %v", err)
    return
}
// Room is now fully initialized and ready to use
```

If the initialization fails, the room is automatically cleaned up and removed from the hotel.

### Automatic Room Cleanup

Rooms are automatically closed and removed from the hotel when they become empty (all clients have left or disconnected). By default, empty rooms are closed after 2 minutes of inactivity:

```go
// The default auto-close delay is 2 minutes
const DefaultAutoCloseDelay = 2 * time.Minute
```

This automatic cleanup helps manage resources by ensuring that unused rooms don't stay in memory indefinitely.

### Message Buffering and Handling Client Disconnections

The system includes built-in message buffering to handle temporary network delays:

- Each client has a buffer channel with capacity for 256 messages
- If this buffer fills up (e.g., if a client is slow or unresponsive), the client is automatically disconnected
- The system properly handles client disconnections by emitting leave events and removing clients from rooms

```go
// When sending messages, check for errors that might indicate disconnection
if err := room.SendToClient(client, message); err != nil {
    log.Printf("Failed to send message, client likely disconnected: %v", err)
    // No need to call RemoveClient - it's handled automatically when send fails
}
```

### Handling Room Context Cancellation

Each room has its own context that's canceled when the room is closed. You can use this to clean up resources or stop goroutines when a room is closed:

```go
func roomHandler(ctx context.Context, room *Room[RoomMetadata, ClientMetadata, DataType]) {
    // Start some background work
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            // Room is closed, clean up and return
            log.Printf("Room %s closed, stopping handler", room.ID())
            return
            
        case <-ticker.C:
            // Do periodic work
            room.Broadcast(Message{Content: "Server heartbeat"})
            
        case event := <-room.Events():
            // Handle events
            // ...
        }
    }
}
```

### Thread Safety

The hotel, rooms, and clients are designed to be thread-safe:

- Multiple goroutines can safely call methods on the same hotel, room, or client
- The implementation uses appropriate mutexes to ensure concurrent operations are safe
- Event handling is designed to be performed in a single goroutine per room for simplicity

## License

MIT License © 2025 Blixt

See [LICENSE](LICENSE) file for details.
