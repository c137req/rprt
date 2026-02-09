# RPRT (Reverse-Proxy Relay Tunnel)

A Go-based reverse proxy system that tunnels HTTP traffic through WebSocket connections with optional proxy routing support.

## Why rprt instead of alternatives?
1. vs VPN
  VPN gives you:
    - Full network visibility
    - Massive blast radius
    - Customer security red flags
  rprt gives you:
    - Only HTTP
    - Only what you explicitly expose
    - One-way trust (agent -> you)

2. vs SSH tunnels
   - SSH doesn’t scale
   - Key management is painful
   - No multi-tenant routing
   - No observability
   - 
3. vs Cloudflare Tunnel / ngrok
   - No third-party dependency
   - No traffic inspection
   - Works in air-gapped-ish environments
   - Predictable cost model

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
|  Client  | ------------->	|  Relay  | <-------------> |  Agent  | ------------> |  Backend  |
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

## Coding Conventions

- functions begin with underscores and use snake_case (e.g. `_handle_request`)
- comments are lowercase and minimal

## Dependencies

| Package | Purpose |
|---------|---------|
| [gorilla/websocket](https://github.com/gorilla/websocket) | websocket protocol implementation |
| [golang.org/x/net](https://pkg.go.dev/golang.org/x/net) | socks5 proxy support |
| [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) | yaml configuration parsing |



## Todos
- [ ] add support for multiple agents/tunnels per relay (with unique IDs or sub-paths)

- [ ] implement end-to-end encryption inside the WebSocket tunnel (e.g. noise protocol or simple AES with per-session keys)

- [ ] replace plain shared_secret with proper per-tunnel asymmetric keys or JWT-style tokens

- [ ] add basic rate limiting / connection limiting on relay to prevent abuse

- [ ] support HTTP/2 or HTTP/3 on the relay listener

- [ ] add logging (structured JSON + file output) with configurable levels

- [ ] implement graceful shutdown / signal handling for both relay and agent

- [ ] write simple end-to-end integration/smoke tests (spin up relay + agent + backend)

- [ ] add Dockerfiles + docker-compose example for quick relay + agent testing

- [ ] document how to generate self-signed certs or use Let's Encrypt for relay TLS

- [ ] add metrics endpoint (Prometheus style) for monitoring tunnel health/connections

- [ ] support custom WebSocket subprotocols or headers for better reverse-proxy compatibility

- [ ] add basic access logging on relay (client IP, request path, status, latency)

- [ ] handle WebSocket close codes properly and log meaningful reconnect reasons

- [ ] add optional basic auth or IP whitelist on relay in addition to HMAC

- [ ] support environment variable substitution in config files

- [ ] create a simple usage/example guide with screenshots or ASCII diagrams

- [ ] add version command (--version) and embed build/commit info

- [ ] publish pre-built binaries via GitHub Releases (linux/amd64, arm64, windows)
