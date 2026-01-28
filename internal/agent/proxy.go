package agent

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyDialer creates network connections routed through a socks5 or http connect proxy.
type ProxyDialer struct {
	proxyURL *url.URL
	timeout  time.Duration
}

// NewProxyDialer parses the proxy url and returns a dialer.
// supported schemes: socks5, socks5h, http, https.
func NewProxyDialer(rawURL string, timeout time.Duration) (*ProxyDialer, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing proxy url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "socks5", "socks5h", "http", "https":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", scheme)
	}
	return &ProxyDialer{proxyURL: u, timeout: timeout}, nil
}

// DialContext establishes a connection to the target address through the proxy.
func (d *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	scheme := strings.ToLower(d.proxyURL.Scheme)
	switch scheme {
	case "socks5", "socks5h":
		return d._dial_socks5(ctx, network, addr)
	case "http", "https":
		return d._dial_http_connect(ctx, network, addr)
	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s", scheme)
	}
}

// _dial_socks5 connects through a socks5 proxy with optional authentication.
func (d *ProxyDialer) _dial_socks5(ctx context.Context, network, addr string) (net.Conn, error) {
	var auth *proxy.Auth
	if d.proxyURL.User != nil {
		password, _ := d.proxyURL.User.Password()
		auth = &proxy.Auth{
			User:     d.proxyURL.User.Username(),
			Password: password,
		}
	}

	host := d.proxyURL.Host
	dialer, err := proxy.SOCKS5("tcp", host, auth, &net.Dialer{
		Timeout: d.timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("creating socks5 dialer: %w", err)
	}

	// use context-aware dial if available
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, addr)
	}
	return dialer.Dial(network, addr)
}

// _dial_http_connect connects through an http connect proxy with optional basic auth.
func (d *ProxyDialer) _dial_http_connect(ctx context.Context, network, addr string) (net.Conn, error) {
	proxyHost := d.proxyURL.Host
	if !strings.Contains(proxyHost, ":") {
		if d.proxyURL.Scheme == "https" {
			proxyHost += ":443"
		} else {
			proxyHost += ":80"
		}
	}

	dialer := &net.Dialer{Timeout: d.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", proxyHost)
	if err != nil {
		return nil, fmt.Errorf("connecting to http proxy: %w", err)
	}

	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
	if d.proxyURL.User != nil {
		password, _ := d.proxyURL.User.Password()
		creds := base64.StdEncoding.EncodeToString(
			[]byte(d.proxyURL.User.Username() + ":" + password),
		)
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", creds)
	}
	connectReq += "\r\n"

	if _, err := conn.Write([]byte(connectReq)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sending connect request: %w", err)
	}

	resp, err := _read_http_response(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("reading connect response: %w", err)
	}
	if !strings.Contains(resp, "200") {
		conn.Close()
		return nil, fmt.Errorf("http connect failed: %s", resp)
	}

	return conn, nil
}

// _read_http_response reads the status line from an http response.
func _read_http_response(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading status line: %w", err)
	}
	// consume remaining headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return statusLine, nil
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	return statusLine, nil
}
