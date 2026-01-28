package relay

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/reverseproxy/internal/protocol"
)

// Tunnel represents a single agent websocket connection on the relay side.
type Tunnel struct {
	id       string
	codec    *protocol.Codec
	conn     *websocket.Conn
	streams  map[uint32]chan *protocol.Frame
	streamMu sync.RWMutex
	done     chan struct{}
	closeOnce sync.Once
	pingInterval time.Duration
}

// NewTunnel wraps an agent websocket connection for multiplexed communication.
func NewTunnel(id string, conn *websocket.Conn, pingInterval time.Duration) *Tunnel {
	t := &Tunnel{
		id:           id,
		codec:        protocol.NewCodec(conn),
		conn:         conn,
		streams:      make(map[uint32]chan *protocol.Frame),
		done:         make(chan struct{}),
		pingInterval: pingInterval,
	}
	go t._read_loop()
	go t._ping_loop()
	return t
}

// SendRequest sends a frame and registers a response channel for the stream.
func (t *Tunnel) SendRequest(f *protocol.Frame) (chan *protocol.Frame, error) {
	ch := make(chan *protocol.Frame, 64)
	t.streamMu.Lock()
	t.streams[f.StreamID] = ch
	t.streamMu.Unlock()

	if err := t.codec.WriteFrame(f); err != nil {
		t._remove_stream(f.StreamID)
		return nil, fmt.Errorf("writing request frame: %w", err)
	}
	return ch, nil
}

// SendFrame sends a frame without registering a response channel.
func (t *Tunnel) SendFrame(f *protocol.Frame) error {
	return t.codec.WriteFrame(f)
}

// Close shuts down the tunnel.
func (t *Tunnel) Close() {
	t.closeOnce.Do(func() {
		close(t.done)
		t.codec.Close()
		t.streamMu.Lock()
		for id, ch := range t.streams {
			close(ch)
			delete(t.streams, id)
		}
		t.streamMu.Unlock()
		slog.Info("tunnel closed", "id", t.id)
	})
}

// Done returns a channel that is closed when the tunnel shuts down.
func (t *Tunnel) Done() <-chan struct{} {
	return t.done
}

// ID returns the tunnel identifier.
func (t *Tunnel) ID() string {
	return t.id
}

// _read_loop continuously reads frames and dispatches them to stream channels.
func (t *Tunnel) _read_loop() {
	defer t.Close()
	for {
		frame, err := t.codec.ReadFrame()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				slog.Error("tunnel read error", "id", t.id, "err", err)
				return
			}
		}

		switch frame.Type {
		case protocol.TypePong:
			// keepalive response, nothing to do
		case protocol.TypeHTTPResponse, protocol.TypeBodyChunk, protocol.TypeStreamClose:
			t.streamMu.RLock()
			ch, ok := t.streams[frame.StreamID]
			t.streamMu.RUnlock()
			if ok {
				select {
				case ch <- frame:
				case <-t.done:
					return
				}
				if frame.Type == protocol.TypeStreamClose {
					t._remove_stream(frame.StreamID)
				}
			}
		default:
			slog.Warn("unexpected frame type from agent", "type", frame.Type, "stream", frame.StreamID)
		}
	}
}

// _ping_loop sends periodic pings to keep the connection alive.
func (t *Tunnel) _ping_loop() {
	ticker := time.NewTicker(t.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			f := &protocol.Frame{Type: protocol.TypePing}
			if err := t.codec.WriteFrame(f); err != nil {
				slog.Error("tunnel ping failed", "id", t.id, "err", err)
				t.Close()
				return
			}
		case <-t.done:
			return
		}
	}
}

// _remove_stream removes a stream channel from the map and closes it.
func (t *Tunnel) _remove_stream(streamID uint32) {
	t.streamMu.Lock()
	if ch, ok := t.streams[streamID]; ok {
		close(ch)
		delete(t.streams, streamID)
	}
	t.streamMu.Unlock()
}
