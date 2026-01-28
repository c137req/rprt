package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/reverseproxy/internal/relay"
)

// RequestHandler processes tunnelled requests against the local backend.
type RequestHandler struct {
	targetURL string
	client    *http.Client
}

// NewRequestHandler creates a handler targeting the given backend url.
func NewRequestHandler(targetURL string) *RequestHandler {
	return &RequestHandler{
		targetURL: targetURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HandleRequest deserialises a tunnelled request, executes it against
// the backend, and returns the serialised response.
func (h *RequestHandler) HandleRequest(data []byte) ([]byte, error) {
	var req relay.TunnelledRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("unmarshalling request: %w", err)
	}

	backendURL := h.targetURL + req.URL
	slog.Debug("forwarding request to backend", "method", req.Method, "url", backendURL)

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, backendURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating backend request: %w", err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	// override host to match the backend
	httpReq.Host = httpReq.URL.Host

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing backend request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading backend response: %w", err)
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	tunnelledResp := relay.TunnelledResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}

	responseData, err := json.Marshal(tunnelledResp)
	if err != nil {
		return nil, fmt.Errorf("marshalling response: %w", err)
	}
	return responseData, nil
}

// _error_response creates a serialised error response with the given status and message.
func _error_response(status int, message string) []byte {
	resp := relay.TunnelledResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       []byte(message),
	}
	data, _ := json.Marshal(resp)
	return data
}
