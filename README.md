# Reverse Proxy

A Go-based reverse proxy system that tunnels HTTP traffic through WebSocket connections with optional proxy routing support.

## Features

- **WebSocket Tunnelling** - multiplexes HTTP requests over a single WebSocket connection
- **Stream Multiplexing** - handles multiple concurrent requests through one tunnel
- **Proxy Support** - routes traffic through SOCKS5 or HTTP CONNECT proxies
- **HMAC-SHA256 Authorisation** - time-based token authorisation between relay and agents
- **TLS Support** - optional TLS encryption for the relay server
- **Auto-Reconnection** - exponential backoff reconnection for agents
- **Proxy Health Checks** - periodic verification that proxy routing is working

## Architecture

```
+----------+     HTTPS      +---------+    WebSocket    +---------+     HTTP      +-----------+
|  Client  | ------------->	|  Relay  | <-------------> |  Agent  | ---------->	|  Backend  |
+----------+                +---------+                 +---------+               +-----------+
                                 |                            |
                                 |                            | (optional)
                                 |                            v
                                 |                       +----------+
                                 |                       |  Proxy   |
                                 |                       | (SOCKS5) |
                                 |                       +----------+
```

1. client sends HTTP request to the relay server
2. relay forwards request through WebSocket tunnel to a connected agent
3. agent forwards request to the local backend (optionally through a proxy)
4. response flows back through the same path

## Prerequisites

- Go 1.22 or later
- TLS certificates (for production deployment)

## Installation

1. clone the repository

```bash
git clone https://github.com/reverseproxy
cd reverseproxy
```

2. build both components

```bash
make all
```

- or build individually

```bash
make relay
make agent
```

Binaries are output to the `bin/` directory.

## Configuration

### Relay Server

Copy the example configuration and modify:

```bash
cp configs/relay.example.yaml configs/relay.yaml
```

```yaml
listen:
  addr: ":443"

tls:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"

auth:
  shared_secret: "your-secret"

tunnel:
  path: "/_tunnel/ws"
  ping_interval: 15s
  request_timeout: 60s
```

- `listen.addr` - port for incoming connections
- `tls.cert_file` / `tls.key_file` - paths to tls certificate and key
- `auth.shared_secret` - must match agent config
- `tunnel.path` - websocket endpoint
- `tunnel.ping_interval` - keepalive frequency
- `tunnel.request_timeout` - max request duration

### Agent

Copy the example configuration and modify:

```bash
cp configs/agent.example.yaml configs/agent.yaml
```

```yaml
relay:
  url: "wss://relay.example.com/_tunnel/ws"

proxy:
  url: "socks5://user:pass@proxy:1080"
  verify_routing: true
  health_timeout: 10s
  recheck_interval: 5m

backend:
  target_url: "http://127.0.0.1:8080"

auth:
  shared_secret: "your-secret"

tunnel:
  reconnect_delay: 2s
  max_reconnect_delay: 60s
  ping_interval: 15s
```

- `relay.url` - relay websocket url
- `proxy` - optional section, omit if not needed
- `proxy.url` - socks5 or http connect proxy url
- `proxy.verify_routing` - checks traffic routes via proxy
- `proxy.recheck_interval` - how often to verify proxy health
- `backend.target_url` - local service to forward to
- `auth.shared_secret` - must match relay config
- `tunnel.reconnect_delay` / `tunnel.max_reconnect_delay` - backoff settings

## Running

### Start the Relay Server

```bash
./bin/relay -config configs/relay.yaml
```

### Start the Agent

```bash
./bin/agent -config configs/agent.yaml
```

The agent will connect to the relay and begin forwarding requests to the configured backend.

## Testing

Run the test suite:

```bash
make test
```


## Dependencies

| Package | Purpose |
|---------|---------|
| [gorilla/websocket](https://github.com/gorilla/websocket) | websocket protocol implementation |
| [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) | socks5 proxy support |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | yaml configuration parsing |
