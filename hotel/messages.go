package hotel

import (
	"fmt"
	"reflect"
)

// TODO: Add in Envelope and User concepts
// Envelope points at a User and a Message
// User is backed by one or more Client instances (authed and has id)

// Message is an interface for message types that can be registered and created dynamically.
// Each message must have a unique type identifier string.
type Message interface {
	Type() string
}

// MessageRegistry is a map of message type identifiers to their corresponding Go types.
// It provides a way to dynamically create message instances of registered types.
type MessageRegistry[M Message] map[string]reflect.Type

// Register adds one or more message types to the registry.
func (r MessageRegistry[M]) Register(msgs ...M) {
	for _, msg := range msgs {
		if _, ok := r[msg.Type()]; ok {
			panic(fmt.Sprintf("Message type %q was already registered", msg.Type()))
		}
		r[msg.Type()] = reflect.TypeOf(msg).Elem()
	}
}

func (r MessageRegistry[M]) Create(msgType string) (msg M, err error) {
	if t, ok := r[msgType]; ok {
		return reflect.New(t).Interface().(M), nil
	}
	err = fmt.Errorf("unknown message type: %q", msgType)
	return
}
