package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"mime"

	"github.com/blixt/go-hotel/examples/gittyup/lib"
	"github.com/blixt/go-hotel/hotel"
	"github.com/gorilla/websocket"
)

var roomManager = hotel.New(lib.RoomInit, lib.RoomHandler)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Implement proper origin checking in production
	},
}

var messageRegistry = hotel.MessageRegistry[hotel.Message]{}

func init() {
	messageRegistry.Register(
		&lib.JoinMessage{},
		&lib.LeaveMessage{},
		&lib.ChatMessage{},
		&lib.WelcomeMessage{},
	)
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	http.HandleFunc("GET /v1/repo/{repo...}", serveWs)
	http.HandleFunc("GET /v1/file/{repoHash}/{commit}/{path...}", serveFile)

	log.Println("Server started on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

var nextUserID atomic.Int32

func serveWs(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP request to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v, Request: %v", err, r)
		return
	}
	log.Printf("WebSocket connection established, Request: %s %s", r.Method, r.URL)

	roomID := r.PathValue("repo")
	name := r.URL.Query().Get("name")

	// Get or create the room
	room, err := roomManager.GetOrCreateRoom(roomID)
	if err != nil {
		log.Printf("Room creation error: %v, RoomID: %s", err, roomID)
		conn.Close()
		return
	}

	// Create a new client
	id := int(nextUserID.Add(1))
	client, err := room.NewClient(&lib.UserMetadata{ID: id, Name: name})
	if err != nil {
		log.Printf("Client creation error: %v, Name: %s", err, name)
		conn.Close()
		return
	}

	// Handle incoming messages from WebSocket
	go func() {
		defer func() {
			room.RemoveClient(client)
			conn.Close()
			log.Printf("Client %s disconnected", name)
		}()

		for {
			select {
			case <-client.Context().Done():
				return
			default:
				// Read the raw message
				_, rawMsg, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Read error: %v, Name: %s", err, name)
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

				log.Printf("Received message from %s: %#v", name, msg)
				room.HandleClientData(client, client.Metadata().Envelop(msg))
			}
		}
	}()

	// Handle outgoing messages to WebSocket
	go func() {
		defer conn.Close()
		for envelope := range client.Receive() {
			// Marshal the message to JSON
			payload, err := json.Marshal(envelope.Message)
			if err != nil {
				log.Printf("Message marshal error: %v", err)
				continue
			}

			// Format as "id type payload"
			outMsg := fmt.Sprintf("%d %s %s", envelope.Sender.ID, envelope.Message.Type(), string(payload))

			err = conn.WriteMessage(websocket.TextMessage, []byte(outMsg))
			if err != nil {
				log.Printf("Write error: %v, Name: %s", err, name)
				return
			}
			log.Printf("Sent message to %s: %s", name, outMsg)
		}
	}()
}

func serveFile(w http.ResponseWriter, r *http.Request) {
	repoHash := r.PathValue("repoHash")
	commit := r.PathValue("commit")
	filePath := r.PathValue("path")

	fullRepoPath := filepath.Join(lib.RepoBasePath, repoHash)

	// Check if the repository exists
	if _, err := os.Stat(fullRepoPath); os.IsNotExist(err) {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	// Use git show to get file contents at specific commit
	cmd := exec.Command("git", "-C", fullRepoPath, "show", fmt.Sprintf("%s:%s", commit, filePath))
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("Git show error: %v, stderr: %s", err, string(exitErr.Stderr))
		}
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Get content type using mime package
	contentType := mime.TypeByExtension(path.Ext(filePath))
	if contentType == "" {
		// Fallback to text/plain if no mime type is found
		contentType = "text/plain"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(output)
}
