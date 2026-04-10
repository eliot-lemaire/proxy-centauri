package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// FluxShield is a per-IP token-bucket rate limiter.
type FluxShield struct {
	rps   float64
	burst int
	mu    sync.Mutex
	peers map[string]*entry
}

// New creates a FluxShield and starts the background cleanup goroutine.
func New(rps float64, burst int) *FluxShield {
	fs := &FluxShield{
		rps:   rps,
		burst: burst,
		peers: make(map[string]*entry),
	}
	go fs.cleanup()
	return fs
}

// limiter returns the rate.Limiter for the given IP, creating one if needed.
func (fs *FluxShield) limiter(ip string) *rate.Limiter {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	e, ok := fs.peers[ip]
	if !ok {
		e = &entry{limiter: rate.NewLimiter(rate.Limit(fs.rps), fs.burst)}
		fs.peers[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// cleanup runs every 60 s and evicts limiters that have been idle for over 60 s.
func (fs *FluxShield) cleanup() {
	for range time.NewTicker(60 * time.Second).C {
		cutoff := time.Now().Add(-60 * time.Second)
		fs.mu.Lock()
		for ip, e := range fs.peers {
			if e.lastSeen.Before(cutoff) {
				delete(fs.peers, ip)
			}
		}
		fs.mu.Unlock()
	}
}

// extractIP returns the client IP from the request, preferring X-Forwarded-For.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address — the original client.
		ip := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
		if ip != "" {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Middleware wraps next with per-IP rate limiting.
// Requests that exceed the limit receive 429 Too Many Requests.
func (fs *FluxShield) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !fs.limiter(ip).Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
