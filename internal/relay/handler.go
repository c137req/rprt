package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/reverseproxy/internal/protocol"
)

// TunnelledRequest is the serialised form of an http request sent through the tunnel.
type TunnelledRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body,omitempty"`
}

// TunnelledResponse is the serialised form of an http response received through the tunnel.
type TunnelledResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body,omitempty"`
}

// Handler forwards incoming http requests to connected agents via the tunnel.
type Handler struct {
	pool    *Pool
	timeout time.Duration
}

// NewHandler creates a new forwarding handler.
func NewHandler(pool *Pool, timeout time.Duration) *Handler {
	return &Handler{pool: pool, timeout: timeout}
}

// ServeHTTP handles incoming requests by forwarding them through the tunnel.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tunnel, err := h.pool.Get()
	if err != nil {
		slog.Warn("no agent available", "err", err)
		http.Error(w, "no backend agents connected", http.StatusBadGateway)
		return
	}

	req, err := _build_tunnelled_request(r)
	if err != nil {
		slog.Error("failed to build tunnelled request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	payload, err := json.Marshal(req)
	if err != nil {
		slog.Error("failed to marshal request", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	streamID := protocol.NextStreamID()
	frames := _chunk_payload(streamID, protocol.TypeHTTPRequest, payload)

	// send request frames and register stream
	var responseCh chan *protocol.Frame
	for i, f := range frames {
		if i == 0 {
			responseCh, err = tunnel.SendRequest(f)
			if err != nil {
				slog.Error("failed to send request", "err", err)
				http.Error(w, "tunnel error", http.StatusBadGateway)
				return
			}
		} else {
			if err := tunnel.SendFrame(f); err != nil {
				slog.Error("failed to send body chunk", "err", err)
				http.Error(w, "tunnel error", http.StatusBadGateway)
				return
			}
		}
	}

	// send stream close after all request data
	if err := tunnel.SendFrame(&protocol.Frame{
		Type:     protocol.TypeStreamClose,
		StreamID: streamID,
	}); err != nil {
		slog.Error("failed to send stream close", "err", err)
	}

	// wait for response with timeout
	_collect_response(w, responseCh, h.timeout)
}

// _build_tunnelled_request converts an http.Request into a TunnelledRequest.
func _build_tunnelled_request(r *http.Request) (*TunnelledRequest, error) {
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
		r.Body.Close()
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	url := r.URL.String()
	return &TunnelledRequest{
		Method:  r.Method,
		URL:     url,
		Headers: headers,
		Body:    body,
	}, nil
}

// _chunk_payload splits a payload into frames respecting the maximum payload size.
func _chunk_payload(streamID uint32, firstType uint8, payload []byte) []*protocol.Frame {
	if len(payload) <= protocol.MaxPayloadSize {
		return []*protocol.Frame{{
			Type:     firstType,
			StreamID: streamID,
			Payload:  payload,
		}}
	}

	var frames []*protocol.Frame
	for i := 0; i < len(payload); i += protocol.MaxPayloadSize {
		end := i + protocol.MaxPayloadSize
		if end > len(payload) {
			end = len(payload)
		}
		msgType := protocol.TypeBodyChunk
		if i == 0 {
			msgType = firstType
		}
		frames = append(frames, &protocol.Frame{
			Type:     msgType,
			StreamID: streamID,
			Payload:  payload[i:end],
		})
	}
	return frames
}

// _collect_response reads response frames and writes the http response.
func _collect_response(w http.ResponseWriter, ch chan *protocol.Frame, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var responseData []byte
	for {
		select {
		case frame, ok := <-ch:
			if !ok {
				// channel closed, stream ended
				if len(responseData) == 0 {
					http.Error(w, "tunnel closed", http.StatusBadGateway)
					return
				}
				_write_response(w, responseData)
				return
			}
			switch frame.Type {
			case protocol.TypeHTTPResponse:
				responseData = append(responseData, frame.Payload...)
			case protocol.TypeBodyChunk:
				responseData = append(responseData, frame.Payload...)
			case protocol.TypeStreamClose:
				_write_response(w, responseData)
				return
			}
		case <-timer.C:
			slog.Warn("request timed out waiting for response")
			http.Error(w, "request timed out", http.StatusGatewayTimeout)
			return
		}
	}
}

// _write_response deserialises a tunnelled response and writes it to the http response writer.
func _write_response(w http.ResponseWriter, data []byte) {
	if len(data) == 0 {
		http.Error(w, "empty response from backend", http.StatusBadGateway)
		return
	}

	var resp TunnelledResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		slog.Error("failed to unmarshal response", "err", err)
		http.Error(w, "invalid response from backend", http.StatusBadGateway)
		return
	}

	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}
