package main

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsServerAddr = "ws://localhost:8080/ws"
	numRooms     = 5
	maxClients   = 50
	messageCount = 10
)

func TestWebSocketStress(t *testing.T) {
	// Start the server
	go main()
	time.Sleep(time.Second) // Wait for the server to start

	for i := 0; i < numRooms; i++ {
		roomID := fmt.Sprintf("room%d", i)
		t.Run(fmt.Sprintf("TestRoom%d", i), func(t *testing.T) {
			t.Parallel()
			testRoom(t, roomID)
		})
	}
}

func testRoom(t *testing.T, roomID string) {
	var wg sync.WaitGroup

	// Join clients
	for i := 0; i < maxClients; i++ {
		wg.Add(1)
		go func(i int) {
			userID := fmt.Sprintf("User %d", i)

			defer wg.Done()
			conn := connectToRoom(t, roomID, userID)
			defer conn.Close()

			// Send messages
			for j := 0; j < messageCount; j++ {
				msg := fmt.Sprintf("Message %d from %s", j, userID)
				err := conn.WriteJSON(Message{Content: msg})
				if err != nil {
					t.Errorf("Failed to send message: %v", err)
					return
				}
			}

			// Read messages (including join notifications)
			for j := 0; j < messageCount*2; j++ {
				_, _, err := conn.ReadMessage()
				if err != nil {
					t.Errorf("Failed to read message: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Empty the room
	t.Logf("Emptying room %s", roomID)
	time.Sleep(time.Second)

	// Recreate the room with half the users
	t.Logf("Recreating room %s with half the users", roomID)
	for i := 0; i < maxClients/2; i++ {
		wg.Add(1)
		go func(i int) {
			userID := fmt.Sprintf("User %d", i)

			defer wg.Done()
			conn := connectToRoom(t, roomID, userID)
			defer conn.Close()

			// Send and receive a few messages
			for j := 0; j < messageCount/2; j++ {
				msg := fmt.Sprintf("Recreated room message %d from %s", j, userID)
				err := conn.WriteJSON(Message{Content: msg})
				if err != nil {
					t.Errorf("Failed to send message in recreated room: %v", err)
					return
				}

				_, _, err = conn.ReadMessage()
				if err != nil {
					t.Errorf("Failed to read message in recreated room: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func connectToRoom(t *testing.T, roomID, userID string) *websocket.Conn {
	u, _ := url.Parse(wsServerAddr)
	u.Path += "/" + roomID
	qs := u.Query()
	qs.Set("name", userID)
	u.RawQuery = qs.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{})
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	return conn
}
