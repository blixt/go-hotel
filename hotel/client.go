package hotel

import (
	"context"
	"errors"
	"sync"
)

type Client[ClientMetadata any, MessageType any] struct {
	metadata  *ClientMetadata
	bufferCh  chan MessageType
	sendCh    chan MessageType
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func newClient[ClientMetadata any, MessageType any](metadata *ClientMetadata) *Client[ClientMetadata, MessageType] {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client[ClientMetadata, MessageType]{
		metadata: metadata,
		bufferCh: make(chan MessageType, 256),
		sendCh:   make(chan MessageType),
		ctx:      ctx,
		cancel:   cancel,
	}
	// Forward messages sent to sendCh (from any goroutine) to a channel that is
	// synchronized to a single goroutine.
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(c.sendCh)
				return
			case msg := <-c.bufferCh:
				// Forwarding to sendCh will always block until the user code
				// has read from the Receive() channel. If the buffer channel
				// fills up, then the send method will close the client, which
				// is why we also check the context here.
				select {
				case <-ctx.Done():
					close(c.sendCh)
					return
				case c.sendCh <- msg:
					// All good, keep going.
				}
			}
		}
	}()
	return c
}

func (c *Client[ClientMetadata, MessageType]) Context() context.Context {
	return c.ctx
}

func (c *Client[ClientMetadata, MessageType]) Metadata() *ClientMetadata {
	return c.metadata
}

func (c *Client[ClientMetadata, MessageType]) send(message MessageType) error {
	select {
	case <-c.ctx.Done():
		return errors.New("client disconnected")
	case c.bufferCh <- message:
		return nil
	default:
		// Channel is full, disconnect the client
		c.Close()
		return errors.New("send channel full, client disconnected")
	}
}

func (c *Client[ClientMetadata, MessageType]) Receive() <-chan MessageType {
	// Return the channel that only the internal client goroutine writes to.
	return c.sendCh
}

func (c *Client[ClientMetadata, MessageType]) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
	})
}
