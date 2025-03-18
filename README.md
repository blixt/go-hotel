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

## License

MIT License © 2025 Blixt

See [LICENSE](LICENSE) file for details.
