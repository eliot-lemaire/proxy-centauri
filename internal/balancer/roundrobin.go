package balancer

import "sync/atomic"

// RoundRobin distributes requests across a list of backends in order,
// cycling back to the first after reaching the last.
type RoundRobin struct {
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
	n := r.counter.Add(1) - 1
	return r.backends[n%uint64(len(r.backends))]
}

// Len returns the number of backends registered in this balancer.
func (r *RoundRobin) Len() int {
	return len(r.backends)
}
