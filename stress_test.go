package main

import (
	"context"
	"encoding/json"
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
	maxClients   = 25
	messageCount = 5
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
	var joinWg sync.WaitGroup

	// Create a context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create an error channel to collect errors from goroutines
	errChan := make(chan error, maxClients)

	for i := 0; i < maxClients; i++ {
		wg.Add(1)
		joinWg.Add(1)
		go func(i int) {
			defer wg.Done()
			userID := fmt.Sprintf("User %d", i)
			conn, err := connectToRoom(roomID, userID)
			if err != nil {
				errChan <- fmt.Errorf("client %d failed to connect: %v", i, err)
				cancel()
				joinWg.Done()
				return
			}
			defer func() {
				conn.Close()
			}()

			// Start read loop
			var messagesWG sync.WaitGroup
			messagesWG.Add(1)
			go func() {
				defer messagesWG.Done()
				messagesCount := 0

				for {
					select {
					case <-ctx.Done():
						return
					default:
						// Set read deadline to prevent blocking forever
						conn.SetReadDeadline(time.Now().Add(5 * time.Second))
						_, data, err := conn.ReadMessage()
						if err != nil {
							if ctx.Err() != nil {
								// Context was cancelled, exit quietly
								return
							}
							errChan <- fmt.Errorf("client %d read error: %v", i, err)
							cancel()
							return
						}

						var msg Message
						if err := json.Unmarshal(data, &msg); err != nil {
							errChan <- fmt.Errorf("client %d unmarshal error: %v", i, err)
							cancel()
							return
						}

						if msg.From == "System" {
							continue
						}

						messagesCount++
						if messagesCount == (maxClients-1)*messageCount {
							return
						}
					}
				}
			}()

			time.Sleep(300 * time.Millisecond)

			joinWg.Done()
			joinWg.Wait()

			// Send messages
			for j := 0; j < messageCount; j++ {
				msg := fmt.Sprintf("Message %d from %s", j, userID)
				err := conn.WriteJSON(Message{Content: msg})
				if err != nil {
					t.Errorf("Failed to send message: %v", err)
					return
				}
			}

			messagesWG.Wait()
		}(i)
	}

	wg.Wait()

	// Check for any errors
	select {
	case err := <-errChan:
		t.Fatalf("Test failed: %v", err)
	default:
		// Test completed successfully
	}
}

func connectToRoom(roomID, userID string) (*websocket.Conn, error) {
	u, _ := url.Parse(wsServerAddr)
	u.Path += "/" + roomID
	qs := u.Query()
	qs.Set("name", userID)
	u.RawQuery = qs.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{})
	if err != nil {
		return nil, err
	}

	return conn, nil
}
