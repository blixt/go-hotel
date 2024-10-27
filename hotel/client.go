package hotel

import (
	"context"
	"errors"
	"sync"
)

type Client[ClientMetadata, DataType any] struct {
	metadata  *ClientMetadata
	bufferCh  chan DataType
	sendCh    chan DataType
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func newClient[ClientMetadata, DataType any](metadata *ClientMetadata) *Client[ClientMetadata, DataType] {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client[ClientMetadata, DataType]{
		metadata: metadata,
		bufferCh: make(chan DataType, 256),
		sendCh:   make(chan DataType),
		ctx:      ctx,
		cancel:   cancel,
	}
	// Forward event data sent to sendCh (from any goroutine) to a channel that
	// is synchronized to a single goroutine.
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(c.sendCh)
				return
			case data := <-c.bufferCh:
				// Forwarding to sendCh will always block until the user code
				// has read from the Receive() channel. If the buffer channel
				// fills up, then the send method will close the client, which
				// is why we also check the context here.
				select {
				case <-ctx.Done():
					close(c.sendCh)
					return
				case c.sendCh <- data:
					// All good, keep going.
				}
			}
		}
	}()
	return c
}

func (c *Client[ClientMetadata, DataType]) Context() context.Context {
	return c.ctx
}

func (c *Client[ClientMetadata, DataType]) Metadata() *ClientMetadata {
	return c.metadata
}

func (c *Client[ClientMetadata, DataType]) send(data DataType) error {
	select {
	case <-c.ctx.Done():
		return errors.New("client disconnected")
	case c.bufferCh <- data:
		return nil
	default:
		// Channel is full, disconnect the client
		c.Close()
		return errors.New("send channel full, client disconnected")
	}
}

func (c *Client[ClientMetadata, DataType]) Receive() <-chan DataType {
	// Return the channel that only the internal client goroutine writes to.
	return c.sendCh
}

func (c *Client[ClientMetadata, DataType]) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
	})
}
