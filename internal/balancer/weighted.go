package balancer

import "sync"

// weightedBackend holds a backend address alongside its static and running weights.
type weightedBackend struct {
	addr          string
	weight        int // configured weight (never changes after init)
	currentWeight int // running tally used by the smooth algorithm
}

// Weighted distributes requests proportionally to backend weights using the
// Nginx smooth weighted round-robin algorithm. This avoids the burst effect
// of naive weighted round-robin by spreading selections evenly.
type Weighted struct {
	mu       sync.Mutex
	backends []*weightedBackend
	total    int // sum of all weights
}

// NewWeighted creates a Weighted balancer. weights[i] corresponds to addrs[i].
// If all weights are 0 (or the slice is shorter than addrs), equal weights of 1 are used.
func NewWeighted(addrs []string, weights []int) *Weighted {
	w := &Weighted{}
	w.init(addrs, weights)
	return w
}

func (w *Weighted) init(addrs []string, weights []int) {
	hasPositive := false
	for _, wt := range weights {
		if wt > 0 {
			hasPositive = true
			break
		}
	}

	backends := make([]*weightedBackend, len(addrs))
	total := 0
	for i, addr := range addrs {
		wt := 1
		if hasPositive && i < len(weights) && weights[i] > 0 {
			wt = weights[i]
		}
		backends[i] = &weightedBackend{addr: addr, weight: wt}
		total += wt
	}
	w.backends = backends
	w.total = total
}

// Next picks the backend with the highest currentWeight, then subtracts the
// total weight from the winner. This is the Nginx smooth weighted round-robin.
func (w *Weighted) Next() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.backends) == 0 {
		return ""
	}
	// Step 1: add each backend's weight to its currentWeight
	for _, b := range w.backends {
		b.currentWeight += b.weight
	}
	// Step 2: pick the backend with the highest currentWeight
	best := w.backends[0]
	for _, b := range w.backends[1:] {
		if b.currentWeight > best.currentWeight {
			best = b
		}
	}
	// Step 3: reduce winner's currentWeight by the total
	best.currentWeight -= w.total
	return best.addr
}

// SetBackends replaces the backend list. Existing weights are preserved for
// backends that remain; new backends get weight 1.
func (w *Weighted) SetBackends(addrs []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	existing := make(map[string]int, len(w.backends))
	for _, b := range w.backends {
		existing[b.addr] = b.weight
	}
	backends := make([]*weightedBackend, len(addrs))
	total := 0
	for i, addr := range addrs {
		wt := 1
		if prev, ok := existing[addr]; ok {
			wt = prev
		}
		backends[i] = &weightedBackend{addr: addr, weight: wt}
		total += wt
	}
	w.backends = backends
	w.total = total
}

// Len returns the number of active backends.
func (w *Weighted) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.backends)
}
