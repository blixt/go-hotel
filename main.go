package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/blixt/go-hotel/hotel"
	"github.com/gorilla/websocket"
)

type RoomMetadata struct {
	Name string
}

type UserMetadata struct {
	Name string
}

type Message struct {
	From    string `json:"from"`
	Content string `json:"content"`
}

var roomManager = hotel.New(roomInit, roomHandler)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Implement proper origin checking in production
	},
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
	userName := r.URL.Query().Get("name")

	// Get or create the room
	room, err := roomManager.GetOrCreateRoom(roomID)
	if err != nil {
		log.Println("Room creation error:", err)
		conn.Close()
		return
	}

	// Create a new client
	client, err := room.NewClient(&UserMetadata{
		Name: userName,
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
				var msg Message
				err := conn.ReadJSON(&msg)
				if err != nil {
					log.Println("Read error:", err)
					return
				}
				msg.From = client.Metadata().Name
				room.HandleClientMessage(client, msg)
			}
		}
	}()

	// Handle outgoing messages to WebSocket
	go func() {
		defer conn.Close()
		for msg := range client.Receive() {
			err := conn.WriteJSON(msg)
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}()
}

func roomInit(id string) (*RoomMetadata, error) {
	// Initialization code (e.g., load data from DB)
	return &RoomMetadata{
		Name: "Test",
	}, nil
}

func roomHandler(ctx context.Context, room *hotel.Room[RoomMetadata, UserMetadata, Message]) {
	log.Printf("Room %s started", room.ID())

	for {
		select {
		case event := <-room.Events():
			switch event.Type {
			case hotel.EventJoin:
				userName := event.Client.Metadata().Name
				log.Printf("%s joined room %s", userName, room.ID())
				// Send a welcome message with the user's name
				welcomeMsg := Message{
					From:    "System",
					Content: fmt.Sprintf("%s has joined the room.", userName),
				}
				room.BroadcastExcept(event.Client, welcomeMsg)
			case hotel.EventLeave:
				userName := event.Client.Metadata().Name
				log.Printf("%s left room %s", userName, room.ID())
				// Notify others with the user's name
				leaveMsg := Message{
					From:    "System",
					Content: fmt.Sprintf("%s has left the room.", userName),
				}
				room.BroadcastExcept(event.Client, leaveMsg)
			case hotel.EventMessage:
				// Broadcast the message to all users except the sender
				log.Printf("%s is broadcasting message to room %s: %s", event.Client.Metadata().Name, room.ID(), event.Message.Content)
				room.BroadcastExcept(event.Client, event.Message)
			}
		case <-ctx.Done():
			// Handler context canceled, perform cleanup
			log.Printf("Handler for room %s is exiting", room.ID())
			return
		}
	}
}
