package balancer

import (
	"sync"
	"sync/atomic"
)

// LeastConn routes each request to the backend with the fewest active connections.
// Call Acquire when opening a connection to a backend and Release when closing it.
type LeastConn struct {
	mu       sync.RWMutex
	backends []string
	conns    map[string]*atomic.Int64 // addr → active connection count
}

// NewLeastConn creates a LeastConn balancer with the given backend addresses.
func NewLeastConn(addrs []string) *LeastConn {
	lc := &LeastConn{
		backends: addrs,
		conns:    make(map[string]*atomic.Int64, len(addrs)),
	}
	for _, addr := range addrs {
		lc.conns[addr] = new(atomic.Int64)
	}
	return lc
}

// Next returns the backend with the fewest active connections.
// It is safe to call from multiple goroutines simultaneously.
func (lc *LeastConn) Next() string {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	if len(lc.backends) == 0 {
		return ""
	}
	best := lc.backends[0]
	bestCount := lc.count(best)
	for _, addr := range lc.backends[1:] {
		if c := lc.count(addr); c < bestCount {
			best = addr
			bestCount = c
		}
	}
	return best
}

// Acquire increments the active connection count for addr.
// Call this immediately after selecting a backend and before opening the connection.
func (lc *LeastConn) Acquire(addr string) {
	lc.mu.RLock()
	c := lc.conns[addr]
	lc.mu.RUnlock()
	if c != nil {
		c.Add(1)
	}
}

// Release decrements the active connection count for addr.
// Call this when the connection or request to addr is complete.
func (lc *LeastConn) Release(addr string) {
	lc.mu.RLock()
	c := lc.conns[addr]
	lc.mu.RUnlock()
	if c != nil {
		c.Add(-1)
	}
}

// SetBackends replaces the backend list. Called by PulseScan when health changes.
// Existing connection counts are preserved for backends that remain in the list.
func (lc *LeastConn) SetBackends(addrs []string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	newConns := make(map[string]*atomic.Int64, len(addrs))
	for _, addr := range addrs {
		if existing, ok := lc.conns[addr]; ok {
			newConns[addr] = existing // preserve live count
		} else {
			newConns[addr] = new(atomic.Int64)
		}
	}
	lc.backends = addrs
	lc.conns = newConns
}

// Len returns the number of active backends.
func (lc *LeastConn) Len() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.backends)
}

// count returns the active connection count for addr. Caller must hold at least RLock.
func (lc *LeastConn) count(addr string) int64 {
	if c := lc.conns[addr]; c != nil {
		return c.Load()
	}
	return 0
}
