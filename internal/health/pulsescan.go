package health

import (
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
)

// PulseScan periodically checks whether each backend is alive and updates
// the balancer to only include healthy Star Systems.
type PulseScan struct {
	name      string
	all       []string
	protocol  string // "http" or "tcp"
	healthy   sync.Map
	balancer  balancer.Balancer
	interval  time.Duration
	client    *http.Client
	eventFunc func(gate, kind, detail string)
}

// New creates a PulseScan for the given addresses, protocol, and balancer.
// All backends are assumed healthy until proven otherwise.
func New(name string, addresses []string, protocol string, lb balancer.Balancer, interval time.Duration) *PulseScan {
	ps := &PulseScan{
		name:     name,
		all:      addresses,
		protocol: protocol,
		balancer: lb,
		interval: interval,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
	for _, addr := range addresses {
		ps.healthy.Store(addr, true)
	}
	return ps
}

// SetEventFunc registers a callback that is called whenever a backend changes
// health state. kind is "backend_up" or "backend_down"; detail is the address.
func (ps *PulseScan) SetEventFunc(fn func(gate, kind, detail string)) {
	ps.eventFunc = fn
}

// SetAll replaces the watched address list. New addresses are assumed healthy;
// addresses no longer present are removed. The balancer is updated immediately.
func (ps *PulseScan) SetAll(addresses []string) {
	newSet := make(map[string]struct{}, len(addresses))
	for _, a := range addresses {
		newSet[a] = struct{}{}
	}

	// Mark removed addresses as unhealthy so they disappear from healthy map.
	for _, a := range ps.all {
		if _, keep := newSet[a]; !keep {
			ps.healthy.Delete(a)
		}
	}

	// Mark new addresses as healthy by default.
	for _, a := range addresses {
		if _, exists := ps.healthy.Load(a); !exists {
			ps.healthy.Store(a, true)
		}
	}

	ps.all = addresses

	// Rebuild live list and push to balancer.
	live := make([]string, 0, len(addresses))
	for _, a := range addresses {
		if ps.Healthy(a) {
			live = append(live, a)
		}
	}
	ps.balancer.SetBackends(live)
}

// Start launches the health check loop in the background.
func (ps *PulseScan) Start() {
	go func() {
		ticker := time.NewTicker(ps.interval)
		defer ticker.Stop()
		for range ticker.C {
			ps.check()
		}
	}()
}

// Healthy reports whether the given address is currently considered alive.
func (ps *PulseScan) Healthy(address string) bool {
	v, ok := ps.healthy.Load(address)
	if !ok {
		return false
	}
	return v.(bool)
}

// check pings every backend and updates the balancer if health state changed.
func (ps *PulseScan) check() {
	changed := false
	live := make([]string, 0, len(ps.all))

	for _, addr := range ps.all {
		wasHealthy := ps.Healthy(addr)
		isHealthy := ps.ping(addr)

		ps.healthy.Store(addr, isHealthy)

		if isHealthy {
			live = append(live, addr)
		}

		if wasHealthy != isHealthy {
			changed = true
			if isHealthy {
				log.Printf("  [ Pulse Scan ] %s  %s  recovered — back in rotation", ps.name, addr)
				if ps.eventFunc != nil {
					ps.eventFunc(ps.name, "backend_up", addr)
				}
			} else {
				log.Printf("  [ Pulse Scan ] %s  %s  is dead — removed from rotation", ps.name, addr)
				if ps.eventFunc != nil {
					ps.eventFunc(ps.name, "backend_down", addr)
				}
			}
		}
	}

	if changed {
		if len(live) == 0 {
			log.Printf("  [ Pulse Scan ] %s  WARNING: all star systems are dead", ps.name)
		}
		ps.balancer.SetBackends(live)
	}
}

// ping checks if the backend is alive using the appropriate method for the protocol.
func (ps *PulseScan) ping(address string) bool {
	switch ps.protocol {
	case "tcp":
		return ps.pingTCP(address)
	case "udp":
		return ps.pingUDP(address)
	default:
		return ps.pingHTTP(address)
	}
}

// pingHTTP attempts an HTTP GET and returns true if the backend responds.
func (ps *PulseScan) pingHTTP(address string) bool {
	resp, err := ps.client.Get("http://" + address + "/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// pingTCP attempts a raw TCP dial and returns true if the connection succeeds.
func (ps *PulseScan) pingTCP(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// pingUDP sends a small datagram and waits for any reply within 2s.
func (ps *PulseScan) pingUDP(address string) bool {
	conn, err := net.DialTimeout("udp", address, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil {
		return false
	}
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	return err == nil
}
