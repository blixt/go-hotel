package hotel

import "fmt"

// EventType defines the type of event that occurred in a room.
type EventType int

func (et EventType) String() string {
	switch et {
	case EventJoin:
		return "EventJoin"
	case EventLeave:
		return "EventLeave"
	case EventCustom:
		return "EventCustom"
	}
	return fmt.Sprintf("<!EventType %d>", et)
}

const (
	// EventJoin indicates a client has joined a room.
	EventJoin EventType = iota
	// EventLeave indicates a client has left a room.
	EventLeave
	// EventCustom indicates a custom message or event from a client.
	EventCustom
)

// Event represents an occurrence in a room, such as a client joining, leaving,
// or sending a message. It contains the event type, the client that triggered it,
// and optional data associated with the event.
type Event[ClientMetadata, DataType any] struct {
	Type   EventType
	Client *Client[ClientMetadata, DataType]
	Data   DataType
}
