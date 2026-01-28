package relay

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Pool manages a set of agent tunnels with round-robin selection.
type Pool struct {
	mu      sync.RWMutex
	tunnels []*Tunnel
	counter atomic.Uint64
}

// NewPool creates an empty agent connection pool.
func NewPool() *Pool {
	return &Pool{}
}

// Add registers a tunnel in the pool and starts monitoring it.
func (p *Pool) Add(t *Tunnel) {
	p.mu.Lock()
	p.tunnels = append(p.tunnels, t)
	p.mu.Unlock()
	slog.Info("agent added to pool", "id", t.ID(), "pool_size", p.Size())

	// remove the tunnel when it closes
	go func() {
		<-t.Done()
		p.Remove(t)
	}()
}

// Remove removes a tunnel from the pool.
func (p *Pool) Remove(t *Tunnel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, existing := range p.tunnels {
		if existing == t {
			p.tunnels = append(p.tunnels[:i], p.tunnels[i+1:]...)
			slog.Info("agent removed from pool", "id", t.ID(), "pool_size", len(p.tunnels))
			return
		}
	}
}

// Get returns the next tunnel using round-robin selection.
func (p *Pool) Get() (*Tunnel, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.tunnels) == 0 {
		return nil, fmt.Errorf("no agents connected")
	}
	idx := p.counter.Add(1) % uint64(len(p.tunnels))
	return p.tunnels[idx], nil
}

// Size returns the number of connected tunnels.
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.tunnels)
}
