package agent

import (
	"context"
	"log/slog"
	"time"
)

// Agent manages the lifecycle of the tunnel connection to the relay,
// including proxy verification and automatic reconnection.
type Agent struct {
	cfg    *Config
	dialer *ProxyDialer
}

// New creates a new agent from the given configuration.
func New(cfg *Config) (*Agent, error) {
	var dialer *ProxyDialer
	if cfg.Proxy.URL != "" {
		var err error
		dialer, err = NewProxyDialer(cfg.Proxy.URL, cfg.Proxy.HealthTimeout)
		if err != nil {
			return nil, err
		}
	}
	return &Agent{cfg: cfg, dialer: dialer}, nil
}

// Run starts the agent. it verifies proxy routing, then enters the
// reconnect loop. blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	if a.dialer != nil && a.cfg.Proxy.VerifyRouting {
		slog.Info("verifying proxy routing before connecting")
		if err := a._verify_proxy(ctx); err != nil {
			return err
		}
	}

	return a._reconnect_loop(ctx)
}

// _verify_proxy checks that traffic is properly routed through the proxy.
func (a *Agent) _verify_proxy(ctx context.Context) error {
	verifier := NewVerifier(a.dialer, a.cfg.Proxy.HealthTimeout)
	return verifier.VerifyRouting(ctx)
}

// _reconnect_loop continuously attempts to connect and maintain the tunnel.
func (a *Agent) _reconnect_loop(ctx context.Context) error {
	delay := a.cfg.Tunnel.ReconnectDelay
	for {
		err := a._run_tunnel(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		slog.Warn("tunnel disconnected, reconnecting", "err", err, "delay", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// exponential backoff
		delay = delay * 2
		if delay > a.cfg.Tunnel.MaxReconnectDelay {
			delay = a.cfg.Tunnel.MaxReconnectDelay
		}
	}
}

// _run_tunnel connects to the relay and processes frames until disconnection.
func (a *Agent) _run_tunnel(ctx context.Context) error {
	tunnel, err := ConnectTunnel(ctx, a.cfg, a.dialer)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	// start periodic proxy health checks if configured
	var stopCheck func()
	var checkFailed <-chan error
	if a.dialer != nil && a.cfg.Proxy.RecheckInterval > 0 {
		verifier := NewVerifier(a.dialer, a.cfg.Proxy.HealthTimeout)
		stopCheck, checkFailed = StartPeriodicCheck(verifier, a.cfg.Proxy.RecheckInterval)
		defer stopCheck()
	}

	// run tunnel in a goroutine
	tunnelErr := make(chan error, 1)
	go func() {
		tunnelErr <- tunnel.Run()
	}()

	// wait for tunnel error, health check failure, or context cancellation
	select {
	case err := <-tunnelErr:
		return err
	case err := <-checkFailed:
		slog.Error("proxy health check failed, closing tunnel", "err", err)
		tunnel.Close()
		return err
	case <-ctx.Done():
		tunnel.Close()
		return ctx.Err()
	}
}
