package lib

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/blixt/go-hotel/hotel"
)

type RoomMetadata struct {
	CloneURL      string
	RepoHash      string
	CurrentCommit string
	Files         []string
}

// HashRoomID generates a SHA-256 hash of the room ID and encodes it in URL-safe base64.
func HashRoomID(roomID string) string {
	hash := sha256.Sum256([]byte(roomID))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// Initialize room
// Runs once when the room is loaded into memory

func RoomInit(roomID string) (*RoomMetadata, error) {
	// Hash the room ID for directory naming.
	hashedRoomID := HashRoomID(roomID)

	// Prepare the repository path
	repoPath := filepath.Join(RepoBasePath, hashedRoomID)
	cloneURL := fmt.Sprintf("https://%s.git", roomID)

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Clone the repository if it doesn't exist.
		if err := CloneRepo(cloneURL, repoPath); err != nil {
			log.Println("Clone error:", err)
			return nil, err
		}
	} else {
		// Pull the latest changes if the repository exists.
		// if err := PullRepo(repoPath); err != nil {
		// 	log.Println("Pull error:", err)
		// 	return nil, err
		// }
	}

	// Get the current commit hash.
	currentCommit, err := GetCurrentCommit(repoPath)
	if err != nil {
		log.Println("Error getting current commit:", err)
		return nil, err
	}

	// Get the list of files
	files, err := GetRepoFiles(repoPath)
	if err != nil {
		log.Println("Error getting repo files:", err)
		return nil, err
	}

	// Return the room metadata with additional information.
	m := &RoomMetadata{
		CloneURL:      cloneURL,
		RepoHash:      hashedRoomID,
		CurrentCommit: currentCommit,
		Files:         files,
	}
	return m, nil
}

// Room event loop
// Runs for as long as the room is active

func RoomHandler(ctx context.Context, room *hotel.Room[RoomMetadata, UserMetadata, Envelope]) {
	// We can safely work on this object directly because nothing else will touch it.
	metadata := room.Metadata()

	defer func() {
		log.Printf("Handler for room %s is exiting", room.ID())
		// TODO: Clean up here.
	}()
	log.Printf("Room %s started", room.ID())

	for {
		select {
		case event := <-room.Events():
			clientMetadata := event.Client.Metadata()
			switch event.Type {
			case hotel.EventJoin:
				// A client joined the room.
				log.Printf("%s joined room %s", clientMetadata.Name, room.ID())
				users := []*UserMetadata{}
				for _, client := range room.Clients() {
					users = append(users, client.Metadata())
				}
				// Send welcome message to the new client.
				room.SendToClient(event.Client, clientMetadata.Envelop(WelcomeMessage{
					Users:         users,
					RepoHash:      metadata.RepoHash,
					CurrentCommit: metadata.CurrentCommit,
					Files:         metadata.Files,
				}))
				// Tell existing clients about the new client.
				room.BroadcastExcept(event.Client, clientMetadata.Envelop(JoinMessage{User: clientMetadata}))
			case hotel.EventLeave:
				// A client left the room.
				log.Printf("%s left room %s", clientMetadata.Name, room.ID())
				// Notify others with the user's name.
				room.BroadcastExcept(event.Client, clientMetadata.Envelop(LeaveMessage{}))
			case hotel.EventCustom:
				// Incoming message from a client.
				switch msg := event.Data.Message.(type) {
				case *ChatMessage:
					log.Printf("<%s> in %s: %s", clientMetadata.Name, room.ID(), msg.Content)
					room.BroadcastExcept(event.Client, event.Data)
				default:
					log.Printf("Unhandled message type: %T", msg)
				}
			}
		case <-ctx.Done():
			// Handler context canceled, perform cleanup.
			return
		}
	}
}
