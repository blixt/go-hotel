package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blixt/go-hotel/hotel"
	"github.com/gorilla/websocket"
)

type RoomMetadata struct {
	CloneURL      string
	CurrentCommit string
}

type UserMetadata struct {
	Name string
}

type JoinMessage struct {
	UserName string `json:"userName"`
}

func (m JoinMessage) Type() string {
	return "join"
}

type LeaveMessage struct {
	UserName string `json:"userName"`
}

func (m LeaveMessage) Type() string {
	return "leave"
}

type ChatMessage struct {
	From    string `json:"from"`
	Content string `json:"content"`
}

func (m ChatMessage) Type() string {
	return "chat"
}

type WelcomeMessage struct {
	Users []string `json:"users"`
}

func (m WelcomeMessage) Type() string {
	return "welcome"
}

type GitStatusMessage struct {
	CurrentCommit string `json:"currentCommit"`
}

func (m GitStatusMessage) Type() string {
	return "git_status"
}

var roomManager = hotel.New(roomInit, roomHandler)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Implement proper origin checking in production
	},
}

const repoBasePath = "./repos"

// hashRoomID generates a SHA-256 hash of the room ID and encodes it in URL-safe base64.
func hashRoomID(roomID string) string {
	hash := sha256.Sum256([]byte(roomID))
	return base64.URLEncoding.EncodeToString(hash[:])
}

var messageRegistry = hotel.MessageRegistry{}

func init() {
	messageRegistry.Register(
		&JoinMessage{},
		&LeaveMessage{},
		&ChatMessage{},
		&WelcomeMessage{},
		&GitStatusMessage{},
	)
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	http.HandleFunc("GET /v1/repo/{repo...}", serveWs)

	log.Println("Server started on http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP request to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v, Request: %v", err, r)
		return
	}
	log.Printf("WebSocket connection established, Request: %s %s", r.Method, r.URL)

	roomID := r.PathValue("repo")
	userName := r.URL.Query().Get("name")

	// Get or create the room
	room, err := roomManager.GetOrCreateRoom(roomID)
	if err != nil {
		log.Printf("Room creation error: %v, RoomID: %s", err, roomID)
		conn.Close()
		return
	}

	// Create a new client
	client, err := room.NewClient(&UserMetadata{
		Name: userName,
	})
	if err != nil {
		log.Printf("Client creation error: %v, UserName: %s", err, userName)
		conn.Close()
		return
	}

	// Handle incoming messages from WebSocket
	go func() {
		defer func() {
			room.RemoveClient(client)
			conn.Close()
			log.Printf("Client %s disconnected", userName)
		}()

		for {
			select {
			case <-client.Context().Done():
				return
			default:
				// Read the raw message
				_, rawMsg, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Read error: %v, UserName: %s", err, userName)
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

				log.Printf("Received message from %s: %v", userName, msg)
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
				log.Printf("Write error: %v, UserName: %s", err, userName)
				return
			}
			log.Printf("Sent message to %s: %s", userName, outMsg)
		}
	}()
}

// getCurrentCommit retrieves the current commit hash of the repository at the given path
func getCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func cloneRepo(remote, path string) error {
	cmd := exec.Command("git", "clone", remote, path)
	// Capture the combined output (stdout and stderr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Clone error: %v, Output: %s", err, string(output))
		return err
	}
	return nil
}

func pullRepo(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "pull")
	return cmd.Run()
}

// Initialize room
// Runs once when the room is loaded into memory

func roomInit(roomID string) (*RoomMetadata, error) {
	// Hash the room ID for directory naming
	hashedRoomID := hashRoomID(roomID)

	// Prepare the repository path
	repoPath := filepath.Join(repoBasePath, hashedRoomID)
	cloneURL := fmt.Sprintf("https://%s.git", roomID)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone the repository if it doesn't exist
		if err := cloneRepo(cloneURL, repoPath); err != nil {
			log.Println("Clone error:", err)
			return nil, err
		}
	} else {
		// Pull the latest changes if the repository exists
		if err := pullRepo(repoPath); err != nil {
			log.Println("Pull error:", err)
			return nil, err
		}
	}

	// Get the current commit hash
	currentCommit, err := getCurrentCommit(repoPath)
	if err != nil {
		log.Println("Error getting current commit:", err)
		return nil, err
	}

	// Return the room metadata with additional information
	m := &RoomMetadata{
		CloneURL:      cloneURL,
		CurrentCommit: currentCommit,
	}
	return m, nil
}

// Room event loop
// Runs for as long as the room is active

func roomHandler(ctx context.Context, room *hotel.Room[RoomMetadata, UserMetadata, hotel.Message]) {
	defer func() {
		log.Printf("Handler for room %s is exiting", room.ID())
		// TODO: Clean up here.
	}()
	log.Printf("Room %s started", room.ID())

	for {
		select {
		case event := <-room.Events():
			switch event.Type {
			case hotel.EventJoin:
				userName := event.Client.Metadata().Name
				log.Printf("%s joined room %s", userName, room.ID())

				// Get list of other users in the room
				otherUsers := []string{}
				for _, client := range room.Clients() {
					if client != event.Client {
						otherUsers = append(otherUsers, client.Metadata().Name)
					}
				}

				// Send welcome message
				room.SendToClient(event.Client, WelcomeMessage{
					Users: otherUsers,
				})

				// Send git status
				room.SendToClient(event.Client, GitStatusMessage{
					CurrentCommit: room.Metadata().CurrentCommit,
				})

				room.BroadcastExcept(event.Client, JoinMessage{UserName: userName})
			case hotel.EventLeave:
				userName := event.Client.Metadata().Name
				log.Printf("%s left room %s", userName, room.ID())
				// Notify others with the user's name
				room.BroadcastExcept(event.Client, LeaveMessage{UserName: userName})
			case hotel.EventMessage:
				if chatMsg, ok := event.Message.(*ChatMessage); ok {
					log.Printf("%s is broadcasting message to room %s: %s",
						event.Client.Metadata().Name, room.ID(), chatMsg.Content)
					room.BroadcastExcept(event.Client, event.Message)
				}
			}
		case <-ctx.Done():
			// Handler context canceled, perform cleanup
			return
		}
	}
}
