package relay_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/reverseproxy/internal/agent"
	"github.com/reverseproxy/internal/relay"
)

// _start_backend creates a simple http server for testing.
func _start_backend(t *testing.T) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "passed")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "hello from backend")
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}

	srv := &http.Server{Handler: mux}
	go srv.Serve(listener)

	addr := fmt.Sprintf("http://%s", listener.Addr().String())
	return addr, func() { srv.Close() }
}

// _start_relay creates and starts a relay server for testing.
func _start_relay(t *testing.T, secret string) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind relay: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	cfg := &relay.Config{
		Listen: relay.ListenConfig{Addr: addr},
		TLS:    relay.TLSConfig{Enabled: false},
		Auth:   relay.AuthConfig{SharedSecret: secret},
		Tunnel: relay.TunnelConfig{
			Path:           "/_tunnel/ws",
			PingInterval:   5 * time.Second,
			RequestTimeout: 10 * time.Second,
		},
	}

	srv := relay.NewServer(cfg)
	go srv.Run()

	// give the server a moment to start
	time.Sleep(100 * time.Millisecond)
	return addr, func() { /* server shuts down when test ends */ }
}

func Test_integration_end_to_end(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	secret := "integration-test-secret"

	// start backend
	backendURL, stopBackend := _start_backend(t)
	defer stopBackend()

	// start relay
	relayAddr, stopRelay := _start_relay(t, secret)
	defer stopRelay()

	// configure and start agent (no proxy for local testing)
	agentCfg := &agent.Config{
		Relay:   agent.RelayConfig{URL: fmt.Sprintf("ws://%s/_tunnel/ws", relayAddr)},
		Backend: agent.BackendConfig{TargetURL: backendURL},
		Auth:    agent.AuthConfig{SharedSecret: secret},
		Proxy:   agent.ProxyConfig{VerifyRouting: false, HealthTimeout: 5 * time.Second},
		Tunnel: agent.TunnelConfig{
			ReconnectDelay:    1 * time.Second,
			MaxReconnectDelay: 5 * time.Second,
			PingInterval:      5 * time.Second,
		},
	}

	a, err := agent.New(agentCfg)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go a.Run(ctx)

	// give the agent time to connect
	time.Sleep(500 * time.Millisecond)

	// test: send request through the relay
	relayURL := fmt.Sprintf("http://%s/hello", relayAddr)
	resp, err := http.Get(relayURL)
	if err != nil {
		t.Fatalf("request through relay failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}

	if string(body) != "hello from backend" {
		t.Errorf("expected %q, got %q", "hello from backend", string(body))
	}

	if resp.Header.Get("X-Test") != "passed" {
		t.Errorf("expected X-Test header 'passed', got %q", resp.Header.Get("X-Test"))
	}
}
