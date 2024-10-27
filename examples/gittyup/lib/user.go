package lib

import "github.com/blixt/go-hotel/hotel"

type UserMetadata struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (um *UserMetadata) Envelop(msg hotel.Message) Envelope {
	return Envelope{um, msg}
}

type Envelope struct {
	Sender  *UserMetadata
	Message hotel.Message
}
