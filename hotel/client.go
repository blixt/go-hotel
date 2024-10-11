package hotel

import (
	"context"
	"errors"
	"sync"
)

type Client[ClientMetadata any, MessageType any] struct {
	metadata  *ClientMetadata
	sendCh    chan MessageType
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func newClient[ClientMetadata any, MessageType any](metadata *ClientMetadata) *Client[ClientMetadata, MessageType] {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client[ClientMetadata, MessageType]{
		metadata: metadata,
		sendCh:   make(chan MessageType, 256),
		ctx:      ctx,
		cancel:   cancel,
	}
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
	case c.sendCh <- message:
		return nil
	default:
		// Channel is full, disconnect the client
		c.Close()
		return errors.New("send channel full, client disconnected")
	}
}

func (c *Client[ClientMetadata, MessageType]) Receive() <-chan MessageType {
	return c.sendCh
}

func (c *Client[ClientMetadata, MessageType]) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		// FIXME: Writes and closes of sendCh have to happen on same goroutine/under lock.
		close(c.sendCh)
	})
}
