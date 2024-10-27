package hotel

import "fmt"

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
	EventJoin EventType = iota
	EventLeave
	EventCustom
)

type Event[ClientMetadata, DataType any] struct {
	Type   EventType
	Client *Client[ClientMetadata, DataType]
	Data   DataType
}
