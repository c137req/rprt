package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/reverseproxy/internal/protocol"
	"github.com/reverseproxy/internal/relay"
)

// Tunnel manages the agent-side websocket connection to the relay.
type Tunnel struct {
	codec     *protocol.Codec
	conn      *websocket.Conn
	done      chan struct{}
	closeOnce sync.Once
	handler   *RequestHandler
	pingInterval time.Duration
}

// ConnectTunnel establishes a websocket connection to the relay,
// optionally routing through a proxy.
func ConnectTunnel(ctx context.Context, cfg *Config, dialer *ProxyDialer) (*Tunnel, error) {
	wsDialer := websocket.Dialer{}
	if dialer != nil {
		wsDialer.NetDialContext = dialer.DialContext
	}

	token := relay.GenerateToken(cfg.Auth.SharedSecret)
	url := cfg.Relay.URL + "?token=" + token

	slog.Info("connecting to relay", "url", cfg.Relay.URL)
	conn, _, err := wsDialer.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dialling relay: %w", err)
	}

	slog.Info("connected to relay")
	return &Tunnel{
		codec:        protocol.NewCodec(conn),
		conn:         conn,
		done:         make(chan struct{}),
		handler:      NewRequestHandler(cfg.Backend.TargetURL),
		pingInterval: cfg.Tunnel.PingInterval,
	}, nil
}

// Run starts processing frames from the relay. blocks until the tunnel closes.
func (t *Tunnel) Run() error {
	go t._ping_loop()
	return t._read_loop()
}

// Close shuts down the tunnel connection.
func (t *Tunnel) Close() {
	t.closeOnce.Do(func() {
		close(t.done)
		t.codec.Close()
		slog.Info("agent tunnel closed")
	})
}

// Done returns a channel that closes when the tunnel shuts down.
func (t *Tunnel) Done() <-chan struct{} {
	return t.done
}

// _read_loop reads frames from the relay and processes them.
func (t *Tunnel) _read_loop() error {
	defer t.Close()
	// collect partial request data per stream
	streams := make(map[uint32][]byte)

	for {
		frame, err := t.codec.ReadFrame()
		if err != nil {
			select {
			case <-t.done:
				return nil
			default:
				return fmt.Errorf("reading frame: %w", err)
			}
		}

		switch frame.Type {
		case protocol.TypePing:
			if err := t.codec.WriteFrame(&protocol.Frame{Type: protocol.TypePong}); err != nil {
				return fmt.Errorf("sending pong: %w", err)
			}

		case protocol.TypeHTTPRequest:
			streams[frame.StreamID] = append(streams[frame.StreamID], frame.Payload...)

		case protocol.TypeBodyChunk:
			streams[frame.StreamID] = append(streams[frame.StreamID], frame.Payload...)

		case protocol.TypeStreamClose:
			data, ok := streams[frame.StreamID]
			if ok {
				delete(streams, frame.StreamID)
				go t._handle_request(frame.StreamID, data)
			}

		default:
			slog.Warn("unexpected frame type from relay", "type", frame.Type)
		}
	}
}

// _handle_request processes a complete request and sends the response back.
func (t *Tunnel) _handle_request(streamID uint32, requestData []byte) {
	responseData, err := t.handler.HandleRequest(requestData)
	if err != nil {
		slog.Error("failed to handle request", "stream", streamID, "err", err)
		responseData = _error_response(502, "backend error: "+err.Error())
	}

	frames := _response_frames(streamID, responseData)
	for _, f := range frames {
		if err := t.codec.WriteFrame(f); err != nil {
			slog.Error("failed to send response frame", "stream", streamID, "err", err)
			return
		}
	}

	// send stream close
	if err := t.codec.WriteFrame(&protocol.Frame{
		Type:     protocol.TypeStreamClose,
		StreamID: streamID,
	}); err != nil {
		slog.Error("failed to send stream close", "stream", streamID, "err", err)
	}
}

// _response_frames splits response data into appropriately sized frames.
func _response_frames(streamID uint32, data []byte) []*protocol.Frame {
	if len(data) <= protocol.MaxPayloadSize {
		return []*protocol.Frame{{
			Type:     protocol.TypeHTTPResponse,
			StreamID: streamID,
			Payload:  data,
		}}
	}

	var frames []*protocol.Frame
	for i := 0; i < len(data); i += protocol.MaxPayloadSize {
		end := i + protocol.MaxPayloadSize
		if end > len(data) {
			end = len(data)
		}
		msgType := protocol.TypeBodyChunk
		if i == 0 {
			msgType = protocol.TypeHTTPResponse
		}
		frames = append(frames, &protocol.Frame{
			Type:     msgType,
			StreamID: streamID,
			Payload:  data[i:end],
		})
	}
	return frames
}

// _ping_loop sends periodic pings to keep the websocket alive.
func (t *Tunnel) _ping_loop() {
	ticker := time.NewTicker(t.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := t.codec.WriteFrame(&protocol.Frame{Type: protocol.TypePing}); err != nil {
				slog.Error("agent ping failed", "err", err)
				t.Close()
				return
			}
		case <-t.done:
			return
		}
	}
}
