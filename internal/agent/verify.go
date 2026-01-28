package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// ip lookup service endpoints.
const _ip_check_url = "https://api.ipify.org"

// Verifier checks that proxy routing is working correctly.
type Verifier struct {
	dialer  *ProxyDialer
	timeout time.Duration
}

// NewVerifier creates a proxy routing verifier.
func NewVerifier(dialer *ProxyDialer, timeout time.Duration) *Verifier {
	return &Verifier{dialer: dialer, timeout: timeout}
}

// VerifyRouting confirms traffic routes through the proxy by comparing
// the direct public ip with the proxied public ip.
func (v *Verifier) VerifyRouting(ctx context.Context) error {
	directIP, err := v._get_direct_ip(ctx)
	if err != nil {
		return fmt.Errorf("getting direct ip: %w", err)
	}

	proxiedIP, err := v._get_proxied_ip(ctx)
	if err != nil {
		return fmt.Errorf("getting proxied ip: %w", err)
	}

	slog.Info("proxy routing check", "direct_ip", directIP, "proxied_ip", proxiedIP)

	if directIP == proxiedIP {
		return fmt.Errorf("proxy not routing traffic: direct ip %s matches proxied ip %s", directIP, proxiedIP)
	}

	slog.Info("proxy routing verified successfully")
	return nil
}

// CheckHealth verifies the proxy is still functional by making a request through it.
func (v *Verifier) CheckHealth(ctx context.Context) error {
	_, err := v._get_proxied_ip(ctx)
	if err != nil {
		return fmt.Errorf("proxy health check failed: %w", err)
	}
	return nil
}

// _get_direct_ip fetches the public ip without using the proxy.
func (v *Verifier) _get_direct_ip(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: v.timeout}
	return _fetch_ip(ctx, client)
}

// _get_proxied_ip fetches the public ip through the proxy.
func (v *Verifier) _get_proxied_ip(ctx context.Context) (string, error) {
	transport := &http.Transport{
		DialContext: v.dialer.DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   v.timeout,
	}
	return _fetch_ip(ctx, client)
}

// _fetch_ip makes a request to the ip check service and returns the ip.
func _fetch_ip(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, _ip_check_url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching ip: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid ip address returned: %q", ip)
	}
	return ip, nil
}

// StartPeriodicCheck runs proxy health checks at the given interval.
// returns a stop function and an error channel that signals when verification fails.
func StartPeriodicCheck(v *Verifier, interval time.Duration) (stop func(), failed <-chan error) {
	done := make(chan struct{})
	errCh := make(chan error, 1)
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
				if err := v.CheckHealth(ctx); err != nil {
					cancel()
					slog.Error("periodic proxy check failed", "err", err)
					select {
					case errCh <- err:
					default:
					}
					return
				}
				cancel()
				slog.Debug("periodic proxy check passed")
			case <-done:
				return
			}
		}
	}()

	return func() {
		close(done)
	}, errCh
}
