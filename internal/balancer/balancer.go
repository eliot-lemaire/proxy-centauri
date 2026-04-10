package balancer

// Balancer selects the next backend address for an incoming request or connection.
// All implementations must be safe for concurrent use.
type Balancer interface {
	Next() string         // returns next backend address; "" if none available
	SetBackends([]string) // replaces live backend list (called by PulseScan)
	Len() int             // number of currently active backends
}

// NewFromConfig returns the right Balancer based on the algorithm name from config.
//
//	"least_connections" → LeastConn
//	"weighted"          → Weighted (uses weights slice; falls back to equal if all 0)
//	anything else       → RoundRobin (default)
func NewFromConfig(addrs []string, weights []int, algorithm string) Balancer {
	switch algorithm {
	case "least_connections":
		return NewLeastConn(addrs)
	case "weighted":
		return NewWeighted(addrs, weights)
	default:
		return New(addrs)
	}
}
