package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/blixt/go-hotel/hotel"
	"github.com/gorilla/websocket"
)

// RoomMetadata contains metadata about a chat room
type RoomMetadata struct {
	Name string
}

// UserMetadata contains metadata about a connected user
type UserMetadata struct {
	Name string
}

// Message types that implement the hotel.Message interface
type JoinMessage struct {
	Name string `json:"name"`
}

func (m JoinMessage) Type() string {
	return "join"
}

type LeaveMessage struct {
	Name string `json:"name"`
}

func (m LeaveMessage) Type() string {
	return "leave"
}

type ChatMessage struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (m ChatMessage) Type() string {
	return "chat"
}

// Global room manager instance
var roomManager = hotel.New(roomInit, roomHandler)

// WebSocket connection upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Implement proper origin checking in production
	},
}

// Message registry for type handling
var messageRegistry = hotel.MessageRegistry{}

// Initialize message types
func init() {
	messageRegistry.Register(
		&JoinMessage{},
		&LeaveMessage{},
		&ChatMessage{},
	)
}

func main() {
	http.HandleFunc("/ws/", serveWs)
	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP request to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// Get room ID from path
	pathSegments := strings.Split(r.URL.Path, "/")
	roomID := pathSegments[len(pathSegments)-1]
	name := r.URL.Query().Get("name")

	// Get or create the room
	room, err := roomManager.GetOrCreateRoom(roomID)
	if err != nil {
		log.Println("Room creation error:", err)
		conn.Close()
		return
	}

	// Create a new client
	client, err := room.NewClient(&UserMetadata{
		Name: name,
	})
	if err != nil {
		log.Println("Client creation error:", err)
		conn.Close()
		return
	}

	// Handle incoming messages from WebSocket
	go func() {
		defer func() {
			room.RemoveClient(client)
			conn.Close()
		}()

		for {
			select {
			case <-client.Context().Done():
				return
			default:
				// Read the raw message
				_, rawMsg, err := conn.ReadMessage()
				if err != nil {
					log.Println("Read error:", err)
					return
				}

				// Split the message into type and payload
				parts := strings.SplitN(string(rawMsg), " ", 2)
				if len(parts) != 2 {
					log.Printf("Invalid message format: %s", string(rawMsg))
					continue
				}

				msgType := parts[0]
				payload := parts[1]

				// Create new message instance of the correct type
				msg, err := messageRegistry.Create(msgType)
				if err != nil {
					log.Printf("Message creation error: %v", err)
					continue
				}

				// Parse the JSON payload
				if err := json.Unmarshal([]byte(payload), msg); err != nil {
					log.Printf("Message unmarshal error: %v", err)
					continue
				}

				room.HandleClientMessage(client, msg)
			}
		}
	}()

	// Handle outgoing messages to WebSocket
	go func() {
		defer conn.Close()
		for msg := range client.Receive() {
			// Marshal the message to JSON
			payload, err := json.Marshal(msg)
			if err != nil {
				log.Printf("Message marshal error: %v", err)
				continue
			}

			// Format as "type payload"
			outMsg := fmt.Sprintf("%s %s", msg.Type(), string(payload))

			err = conn.WriteMessage(websocket.TextMessage, []byte(outMsg))
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}()
}

// roomInit initializes a new room with the given ID
func roomInit(id string) (*RoomMetadata, error) {
	// Initialization code (e.g., load data from DB)
	return &RoomMetadata{
		Name: "Test",
	}, nil
}

// roomHandler handles all room events and message broadcasting
func roomHandler(ctx context.Context, room *hotel.Room[RoomMetadata, UserMetadata, hotel.Message]) {
	log.Printf("Room %s started", room.ID())

	for {
		select {
		case event := <-room.Events():
			switch event.Type {
			case hotel.EventJoin:
				name := event.Client.Metadata().Name
				log.Printf("%s joined room %s", name, room.ID())
				room.BroadcastExcept(event.Client, &JoinMessage{
					Name: name,
				})
			case hotel.EventLeave:
				name := event.Client.Metadata().Name
				log.Printf("%s left room %s", name, room.ID())
				room.BroadcastExcept(event.Client, &LeaveMessage{
					Name: name,
				})
			case hotel.EventMessage:
				if chatMsg, ok := event.Message.(*ChatMessage); ok {
					log.Printf("%s is broadcasting message to room %s: %s",
						event.Client.Metadata().Name, room.ID(), chatMsg.Content)
					room.BroadcastExcept(event.Client, event.Message)
				}
			}
		case <-ctx.Done():
			// Handler context canceled, perform cleanup
			log.Printf("Handler for room %s is exiting", room.ID())
			return
		}
	}
}
