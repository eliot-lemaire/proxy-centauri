package balancer

import (
	"sync"
	"sync/atomic"
)

// RoundRobin distributes requests across a list of backends in order,
// cycling back to the first after reaching the last.
type RoundRobin struct {
	mu       sync.RWMutex
	backends []string
	counter  atomic.Uint64
}

// New creates a RoundRobin balancer with the given list of backend addresses.
func New(backends []string) *RoundRobin {
	return &RoundRobin{backends: backends}
}

// Next returns the address of the next backend in the rotation.
// It is safe to call from multiple goroutines simultaneously.
func (r *RoundRobin) Next() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.backends) == 0 {
		return ""
	}
	n := r.counter.Add(1) - 1
	return r.backends[n%uint64(len(r.backends))]
}

// SetBackends replaces the backend list. Called by PulseScan when health changes.
// It is safe to call from multiple goroutines simultaneously.
func (r *RoundRobin) SetBackends(backends []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backends = backends
}

// Len returns the number of backends currently registered in this balancer.
func (r *RoundRobin) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.backends)
}
