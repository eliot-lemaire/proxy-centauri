package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"github.com/eliot-lemaire/proxy-centauri/internal/health"
	stellarlog "github.com/eliot-lemaire/proxy-centauri/internal/logger"
	"github.com/eliot-lemaire/proxy-centauri/internal/metrics"
	"github.com/eliot-lemaire/proxy-centauri/internal/proxy"
	"github.com/eliot-lemaire/proxy-centauri/internal/ratelimit"
	stellar "github.com/eliot-lemaire/proxy-centauri/internal/tls"
	"github.com/eliot-lemaire/proxy-centauri/internal/tunnel"
)

const logo = `
    ██████╗ ██████╗  ██████╗ ██╗  ██╗██╗   ██╗
    ██╔══██╗██╔══██╗██╔═══██╗╚██╗██╔╝╚██╗ ██╔╝
    ██████╔╝██████╔╝██║   ██║ ╚███╔╝  ╚████╔╝
    ██╔═══╝ ██╔══██╗██║   ██║ ██╔██╗   ╚██╔╝
    ██║     ██║  ██║╚██████╔╝██╔╝ ██╗   ██║
    ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝

     ██████╗███████╗███╗   ██╗████████╗ █████╗ ██╗   ██╗██████╗ ██╗
    ██╔════╝██╔════╝████╗  ██║╚══██╔══╝██╔══██╗██║   ██║██╔══██╗██║
    ██║     █████╗  ██╔██╗ ██║   ██║   ███████║██║   ██║██████╔╝██║
    ██║     ██╔══╝  ██║╚██╗██║   ██║   ██╔══██║██║   ██║██╔══██╗██║
    ╚██████╗███████╗██║ ╚████║   ██║   ██║  ██║╚██████╔╝██║  ██║██║
     ╚═════╝╚══════╝╚═╝  ╚═══╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝

           ✦  Your traffic, your rules, your universe  ✦
              v0.2.0 — Milestone 2: Engaging Engines
`

func main() {
	fmt.Print(logo)
	fmt.Println("  [ Mission Control ] Initializing...")

	cfg, err := config.Load("centauri.yml")
	if err != nil {
		log.Fatalf("  [ Mission Control ] Failed to load centauri.yml: %v", err)
	}

	fmt.Printf("  [ Mission Control ] %d jump gate(s) configured\n", len(cfg.JumpGates))

	// Start Prometheus metrics endpoint once, before the gate loop.
	if cfg.Metrics.Enabled {
		metrics.Init()
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Metrics.Port), metrics.Handler()); err != nil {
				log.Printf("  [ Metrics         ] server error: %v", err)
			}
		}()
		fmt.Printf("  [ Metrics         ] Prometheus endpoint on :%d/metrics\n", cfg.Metrics.Port)
	}

	for _, gate := range cfg.JumpGates {
		fmt.Printf("  [ Jump Gate       ] %q  →  %s  (%s)\n", gate.Name, gate.Listen, gate.Protocol)

		addrs := make([]string, len(gate.StarSystems))
		weights := make([]int, len(gate.StarSystems))
		for i, ss := range gate.StarSystems {
			addrs[i] = ss.Address
			weights[i] = ss.Weight
			fmt.Printf("  [ Star System     ]     %s\n", ss.Address)
		}

		algo := gate.OrbitalRouter
		if algo == "" {
			algo = "round_robin"
		}
		lb := balancer.NewFromConfig(addrs, weights, gate.OrbitalRouter)

		ps := health.New(gate.Name, addrs, gate.Protocol, lb, 5*time.Second)
		ps.Start()
		fmt.Printf("  [ Pulse Scan      ] health checks every 5s\n")

		if gate.Protocol == "http" {
			// Open the per-gate JSON log file.
			logPath := fmt.Sprintf("logs/%s.log", gate.Name)
			sl, err := stellarlog.New(logPath)
			if err != nil {
				log.Fatalf("  [ Stellar Log     ] failed to open %s: %v", logPath, err)
			}
			fmt.Printf("  [ Stellar Log     ] writing to %s\n", logPath)

			// Build middleware chain — outermost → innermost:
			// FluxShield → Metrics → StellarLog → ReverseProxy
			var h http.Handler = proxy.New(lb)
			h = sl.Middleware(gate.Name)(h)
			if cfg.Metrics.Enabled {
				h = metrics.Middleware(gate.Name)(h)
			}
			if gate.FluxShield.RequestsPerSecond > 0 {
				h = ratelimit.New(gate.FluxShield.RequestsPerSecond, gate.FluxShield.Burst).Middleware(h)
				fmt.Printf("  [ Flux Shield     ] %.0f req/s, burst %d\n",
					gate.FluxShield.RequestsPerSecond, gate.FluxShield.Burst)
			}

			srv := &http.Server{Addr: gate.Listen, Handler: h}
			switch gate.TLS.Mode {
			case "auto":
				srv.TLSConfig = stellar.AutoCert(gate.TLS.Domain, ".certs")
				go func() {
					if err := srv.ListenAndServeTLS("", ""); err != nil {
						log.Printf("  [ Jump Gate ] TLS listener on %s failed: %v", gate.Listen, err)
					}
				}()
				fmt.Printf("  [ Orbital Router  ] %s — listening on %s — TLS auto (Let's Encrypt)\n", algo, gate.Listen)
			case "manual":
				go func() {
					if err := srv.ListenAndServeTLS(gate.TLS.CertFile, gate.TLS.KeyFile); err != nil {
						log.Printf("  [ Jump Gate ] TLS listener on %s failed: %v", gate.Listen, err)
					}
				}()
				fmt.Printf("  [ Orbital Router  ] %s — listening on %s — TLS manual\n", algo, gate.Listen)
			default:
				go func() {
					if err := srv.ListenAndServe(); err != nil {
						log.Printf("  [ Jump Gate ] listener on %s failed: %v", gate.Listen, err)
					}
				}()
				fmt.Printf("  [ Orbital Router  ] %s — listening on %s — ready to route\n", algo, gate.Listen)
			}
		}

		if gate.Protocol == "tcp" {
			t := tunnel.New(lb)
			go t.Listen(gate.Listen)
			fmt.Printf("  [ Orbital Router  ] %s — listening on %s — ready to tunnel\n", algo, gate.Listen)
		}
	}

	if err := config.Watch("centauri.yml", func(newCfg *config.Config) {
		fmt.Printf("  [ Config          ] Reloaded — %d jump gate(s)\n", len(newCfg.JumpGates))
	}); err != nil {
		log.Fatalf("  [ Mission Control ] Failed to start config watcher: %v", err)
	}

	fmt.Println("  [ Mission Control ] Ready. Watching for config changes...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n  [ Mission Control ] Shutting down. Safe travels.")
}
