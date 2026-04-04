package health

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
)

// PulseScan periodically checks whether each backend is alive and updates
// the balancer to only include healthy Star Systems.
type PulseScan struct {
	all      []string
	healthy  sync.Map
	balancer *balancer.RoundRobin
	interval time.Duration
	client   *http.Client
}

// New creates a PulseScan for the given addresses and balancer.
// All backends are assumed healthy until proven otherwise.
func New(addresses []string, lb *balancer.RoundRobin, interval time.Duration) *PulseScan {
	ps := &PulseScan{
		all:      addresses,
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
				log.Printf("  [ Pulse Scan ] %s  recovered — back in rotation", addr)
			} else {
				log.Printf("  [ Pulse Scan ] %s  is dead — removed from rotation", addr)
			}
		}
	}

	if changed {
		if len(live) == 0 {
			log.Println("  [ Pulse Scan ] WARNING: all star systems are dead")
		}
		ps.balancer.SetBackends(live)
	}
}

// ping attempts an HTTP GET to the address and returns true if it responds.
func (ps *PulseScan) ping(address string) bool {
	resp, err := ps.client.Get("http://" + address + "/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}
