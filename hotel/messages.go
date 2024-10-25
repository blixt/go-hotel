package hotel

import (
	"fmt"
	"reflect"
)

type Message interface {
	Type() string
}

type MessageRegistry map[string]reflect.Type

// Register adds one or more message types to the registry.
func (r MessageRegistry) Register(msgs ...Message) {
	for _, msg := range msgs {
		if _, ok := r[msg.Type()]; ok {
			panic(fmt.Sprintf("Message type %q was already registered", msg.Type()))
		}
		r[msg.Type()] = reflect.TypeOf(msg).Elem()
	}
}

func (r MessageRegistry) Create(msgType string) (Message, error) {
	if t, ok := r[msgType]; ok {
		return reflect.New(t).Interface().(Message), nil
	}
	return nil, fmt.Errorf("unknown message type: %q", msgType)
}
