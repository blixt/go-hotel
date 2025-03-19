# go-hotel

A type-safe framework for building real-time, room-based applications in Go.

## Overview

`go-hotel` provides a simple architecture for real-time applications:

- **Hotel**: Manages room creation and lifecycle
- **Room**: Contains clients and processes events
- **Client**: Maintains connection with automatic buffering
- **Event**: Join, leave, or custom message from a client

## Installation

```bash
go get github.com/blixt/go-hotel
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "github.com/blixt/go-hotel/hotel"
)

// 1. Define your metadata and message types
type RoomData struct{ Name string }
type UserData struct{ ID, Username string }

// 2. Define a message type with required Type() method
type ChatMessage struct{ Content string `json:"content"` }

func (m ChatMessage) Type() string { return "chat" }

func main() {
    // 3. Create a hotel
    h := hotel.New(
        // 4. Define room initialization function
        func(ctx context.Context, id string) (*RoomData, error) {
            return &RoomData{Name: id}, nil
        },
        // 5. Define room event handler
        func(ctx context.Context, room *hotel.Room[RoomData, UserData, hotel.Message]) {
            for {
                select {
                case <-ctx.Done():
                    return
                case event := <-room.Events():
                    // 6. Handle different event types
                    switch event.Type {
                    case hotel.EventJoin:
                        log.Printf("User joined: %s", event.Client.Metadata().Username)
                        
                    case hotel.EventLeave:
                        log.Printf("User left: %s", event.Client.Metadata().Username)
                        
                    case hotel.EventCustom:
                        // 7. Type switch for custom messages
                        switch msg := event.Data.(type) {
                        case *ChatMessage:
                            sender := event.Client.Metadata().Username
                            log.Printf("Chat: %s: %s", sender, msg.Content)
                            // 8. Broadcast message to other clients
                            room.BroadcastExcept(event.Client, msg)
                        }
                    }
                }
            }
        },
    )

    // 9. Create a room and add a client
    room, _ := h.GetOrCreateRoom("room1")
    client, _ := room.NewClient(&UserData{ID: "user1", Username: "Alice"})
    
    // 10. Send a message
    room.HandleClientData(client, &ChatMessage{Content: "Hello!"})
}
```

## Core Concepts

### Hotel

The Hotel manages all your rooms:

```go
// Create a hotel with room init and handler functions
hotel := hotel.New(roomInitFunc, roomHandlerFunc)

// Get or create a room
room, err := hotel.GetOrCreateRoom("room-id")
```

### Room

Rooms manage clients and process events:

```go
// Get room details
id := room.ID()
metadata := room.Metadata()

// Client operations
clients := room.Clients()
client := room.FindClient(func(meta *UserData) bool {
    return meta.Username == "Bob"
})

// Communication
chatMsg := &ChatMessage{Content: "Hello everyone"}
room.Broadcast(chatMsg)                     // Send to all
room.BroadcastExcept(client, chatMsg)       // Send to all except one
err := room.SendToClient(client, chatMsg)   // Send to specific client

// Lifecycle
room.RemoveClient(client)                   // Remove a client
room.Close()                                // Close the room
```

Rooms are automatically cleaned up when empty after 2 minutes.

### Client

Clients represent connected users:

```go
// Create a client
client, err := room.NewClient(&UserData{ID: "user2", Username: "Bob"})

// Client properties
metadata := client.Metadata()
ctx := client.Context()  // Cancelled when client disconnects

// Reading messages sent to this client
for msg := range client.Receive() {
    // Process incoming message
}

// Manually close connection
client.Close()
```

Clients have a buffer capacity of 256 messages and will disconnect if full.

### Messages

Messages must implement the Message interface:

```go
type Message interface {
    Type() string
}
```

Example message types:

```go
type ChatMessage struct {
    Content string `json:"content"`
}

func (m ChatMessage) Type() string {
    return "chat"
}

type MoveMessage struct {
    X int `json:"x"`
    Y int `json:"y"`
}

func (m MoveMessage) Type() string {
    return "move"
}
```

The library also provides a `MessageRegistry` for dynamically creating messages by their type string:

```go
// Create a registry
registry := hotel.MessageRegistry[hotel.Message]{}

// Register your message types
registry.Register(&ChatMessage{}, &MoveMessage{})

// Create a message from a type string
msg, err := registry.Create("chat")
if err == nil {
    // Cast to the concrete type
    chatMsg := msg.(*ChatMessage)
    chatMsg.Content = "Hello!"
}
```

This is useful when parsing messages from external formats where the message type comes as a string.

## WebSocket Example

```go
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 1. Setup connection
    conn, _ := upgrader.Upgrade(w, r, nil)
    roomID := r.URL.Query().Get("room")
    userID := r.URL.Query().Get("id")
    username := r.URL.Query().Get("user")
    
    // 2. Create/join room
    room, _ := hotelManager.GetOrCreateRoom(roomID)
    client, _ := room.NewClient(&UserData{ID: userID, Username: username})
    defer room.RemoveClient(client)
    
    // 3. Handle incoming messages (WebSocket → Room)
    go func() {
        for {
            _, rawData, err := conn.ReadMessage()
            if err != nil {
                return // Connection closed
            }
            
            // 4. Parse message from your protocol
            msg, _ := parseMessage(rawData)
            room.HandleClientData(client, msg)
        }
    }()
    
    // 5. Handle outgoing messages (Room → WebSocket)
    for msg := range client.Receive() {
        // 6. Format message for your protocol
        data, _ := formatMessage(msg)
        conn.WriteMessage(websocket.TextMessage, data)
    }
}
```

## Protocol Implementation

The library doesn't dictate how you serialize messages. Here's a simple example using a "type JSON" format:

```go
// 1. Create message registry for type-safe message handling
var registry = hotel.MessageRegistry[hotel.Message]{}

// 2. Register message types during initialization
func init() {
    registry.Register(&ChatMessage{}, &MoveMessage{})
}

// 3. Parse an incoming "type JSON" formatted message
func parseMessage(data []byte) (hotel.Message, error) {
    parts := strings.SplitN(string(data), " ", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid format")
    }
    
    // 4. Create message of the correct type
    msg, err := registry.Create(parts[0])
    if err != nil {
        return nil, err
    }
    
    // 5. Populate with JSON data
    if err := json.Unmarshal([]byte(parts[1]), msg); err != nil {
        return nil, err
    }
    
    return msg, nil
}

// 6. Format a message as "type JSON"
func formatMessage(msg hotel.Message) ([]byte, error) {
    data, err := json.Marshal(msg)
    if err != nil {
        return nil, err
    }
    return []byte(fmt.Sprintf("%s %s", msg.Type(), string(data))), nil
}
```

## Error Handling

Always check errors when creating rooms, clients, and sending messages:

```go
room, err := hotel.GetOrCreateRoom("room-id")
if err != nil {
    // Room initialization failed
    return
}

client, err := room.NewClient(&UserData{ID: "user1", Username: "Alice"})
if err != nil {
    // Room is closed or invalid client data
    return
}

chatMsg := &ChatMessage{Content: "Hello!"}
err = room.SendToClient(client, chatMsg)
if err != nil {
    // Client is disconnected or buffer is full
}
```

## Thread Safety

The library is designed to be thread-safe - multiple goroutines can safely interact with the same hotel, room, or client.

## License

MIT License © 2025 Blixt
