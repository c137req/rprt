package relay

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

// Server is the main relay server that accepts public http traffic
// and agent websocket connections.
type Server struct {
	cfg      *Config
	pool     *Pool
	handler  *Handler
	upgrader websocket.Upgrader
}

// NewServer creates a configured relay server.
func NewServer(cfg *Config) *Server {
	pool := NewPool()
	handler := NewHandler(pool, cfg.Tunnel.RequestTimeout)
	return &Server{
		cfg:     cfg,
		pool:    pool,
		handler: handler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Run starts the relay server and blocks until it exits.
func (s *Server) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc(s.cfg.Tunnel.Path, s._handle_tunnel)
	mux.Handle("/", s.handler)

	slog.Info("relay server starting", "addr", s.cfg.Listen.Addr, "tls", s.cfg.TLS.Enabled)

	if s.cfg.TLS.Enabled {
		return http.ListenAndServeTLS(
			s.cfg.Listen.Addr,
			s.cfg.TLS.CertFile,
			s.cfg.TLS.KeyFile,
			mux,
		)
	}
	return http.ListenAndServe(s.cfg.Listen.Addr, mux)
}

// _handle_tunnel handles websocket upgrade requests from agents.
func (s *Server) _handle_tunnel(w http.ResponseWriter, r *http.Request) {
	// validate auth token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-Auth-Token")
	}
	if err := ValidateToken(s.cfg.Auth.SharedSecret, token); err != nil {
		slog.Warn("agent auth failed", "err", err, "remote", r.RemoteAddr)
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "err", err)
		return
	}

	tunnelID := fmt.Sprintf("agent-%s", r.RemoteAddr)
	slog.Info("agent connected", "id", tunnelID, "remote", r.RemoteAddr)

	tunnel := NewTunnel(tunnelID, conn, s.cfg.Tunnel.PingInterval)
	s.pool.Add(tunnel)
}
