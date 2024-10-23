package hotel

import "fmt"

type EventType int

func (et EventType) String() string {
	switch et {
	case EventJoin:
		return "EventJoin"
	case EventLeave:
		return "EventLeave"
	case EventMessage:
		return "EventMessage"
	}
	return fmt.Sprintf("<!EventType %d>", et)
}

const (
	EventJoin EventType = iota
	EventLeave
	EventMessage
)

type Event[ClientMetadata any, MessageType any] struct {
	Type    EventType
	Client  *Client[ClientMetadata, MessageType]
	Message MessageType
}
