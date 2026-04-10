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
	name     string
	all      []string
	protocol string // "http" or "tcp"
	healthy  sync.Map
	balancer balancer.Balancer
	interval time.Duration
	client   *http.Client
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
			} else {
				log.Printf("  [ Pulse Scan ] %s  %s  is dead — removed from rotation", ps.name, addr)
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
	if ps.protocol == "tcp" {
		return ps.pingTCP(address)
	}
	return ps.pingHTTP(address)
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
