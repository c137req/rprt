package protocol

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// Codec handles reading and writing frames over a websocket connection.
type Codec struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// NewCodec wraps a websocket connection with frame encoding/decoding.
func NewCodec(conn *websocket.Conn) *Codec {
	return &Codec{conn: conn}
}

// WriteFrame serialises and sends a frame over the websocket.
func (c *Codec) WriteFrame(f *Frame) error {
	data, err := MarshalFrame(f)
	if err != nil {
		return fmt.Errorf("marshalling frame: %w", err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, data)
}

// ReadFrame reads and deserialises a frame from the websocket.
func (c *Codec) ReadFrame() (*Frame, error) {
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("reading websocket message: %w", err)
	}
	if msgType != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", msgType)
	}
	return UnmarshalFrame(data)
}

// Close closes the underlying websocket connection.
func (c *Codec) Close() error {
	return c.conn.Close()
}
