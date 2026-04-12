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
	"github.com/eliot-lemaire/proxy-centauri/internal/oracle"
	"github.com/eliot-lemaire/proxy-centauri/internal/proxy"
	"github.com/eliot-lemaire/proxy-centauri/internal/ratelimit"
	stellar "github.com/eliot-lemaire/proxy-centauri/internal/tls"
	"github.com/eliot-lemaire/proxy-centauri/internal/tunnel"
	_ "modernc.org/sqlite"
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
              v0.3.0 — Milestone 3: Quantum Link Established
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runWizard(); err != nil {
			log.Fatalf("centauri init failed: %v", err)
		}
		return
	}

	fmt.Print(logo)
	fmt.Println("  [ Mission Control ] Initializing...")

	cfg, err := config.Load("centauri.yml")
	if err != nil {
		log.Fatalf("  [ Mission Control ] Failed to load centauri.yml: %v", err)
	}

	fmt.Printf("  [ Mission Control ] %d jump gate(s) configured\n", len(cfg.JumpGates))

	// Collect gate names upfront — needed by the metrics flush ticker.
	gateNames := make([]string, len(cfg.JumpGates))
	for i, g := range cfg.JumpGates {
		gateNames[i] = g.Name
	}

	// Start Prometheus metrics endpoint + SQLite store (both guarded by Metrics.Enabled).
	var store *metrics.Store
	if cfg.Metrics.Enabled {
		metrics.Init()

		store, err = metrics.OpenStore("data/metrics.db")
		if err != nil {
			log.Fatalf("  [ Store           ] failed to open data/metrics.db: %v", err)
		}
		if err := store.Init(); err != nil {
			log.Fatalf("  [ Store           ] failed to init schema: %v", err)
		}
		defer store.Close()
		fmt.Println("  [ Store           ] SQLite metrics store at data/metrics.db")

		ora := oracle.New(cfg.Oracle, store, gateNames)
		ora.Start()
		if ora != nil {
			fmt.Println("  [ The Oracle      ] AI engine online")
		}

		fmt.Printf("  [ Metrics         ] Prometheus endpoint on :%d/metrics\n", cfg.Metrics.Port)
		httpMux := http.NewServeMux()
		httpMux.Handle("/metrics", metrics.Handler())
		if ora != nil {
			sh := oracle.SignalsHandler(store)
			httpMux.Handle("/oracle/signals", sh)
			httpMux.Handle("/oracle/signals/", sh)
			fmt.Printf("  [ The Oracle      ] signals at :%d/oracle/signals\n", cfg.Metrics.Port)
		}
		go func() {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Metrics.Port), httpMux); err != nil {
				log.Printf("  [ Metrics         ] server error: %v", err)
			}
		}()

		// Flush a snapshot per gate every 30 seconds.
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				for _, snap := range metrics.Snapshots(gateNames) {
					if err := store.Flush(snap); err != nil {
						log.Printf("  [ Store           ] flush error: %v", err)
					}
				}
				// Let The Oracle check for threshold breaches after each flush.
				ora.Check(oracle.BuildSnapshot(gateNames, nil, 30))
			}
		}()
	}

	// gateRegistry maps gate name → running PulseScan for hot-reload backend updates.
	type gateState struct {
		ps *health.PulseScan
	}
	gateRegistry := make(map[string]*gateState, len(cfg.JumpGates))

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
		if store != nil {
			ps.SetEventFunc(func(g, kind, detail string) {
				if err := store.LogEvent(g, kind, detail); err != nil {
					log.Printf("  [ Store           ] event log error: %v", err)
				}
			})
		}
		ps.Start()
		gateRegistry[gate.Name] = &gateState{ps: ps}
		fmt.Printf("  [ Pulse Scan      ] health checks every 5s\n")

		if gate.Protocol == "http" {
			if cfg.Metrics.Enabled {
				metrics.InitGate(gate.Name)
			}

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

		if gate.Protocol == "udp" {
			t := tunnel.NewUDP(lb)
			go t.Listen(gate.Listen)
			fmt.Printf("  [ Orbital Router  ] %s — listening on %s — ready to tunnel (UDP)\n", algo, gate.Listen)
		}
	}

	if err := config.Watch("centauri.yml", func(newCfg *config.Config) {
		fmt.Printf("  [ Config          ] Reloaded — %d jump gate(s)\n", len(newCfg.JumpGates))
		if store != nil {
			store.LogEvent("*", "config_reload", "centauri.yml")
		}
		// Apply updated star_systems to running health checkers (and their balancers).
		// Adding/removing entire gates requires a restart.
		for _, newGate := range newCfg.JumpGates {
			if gs, ok := gateRegistry[newGate.Name]; ok {
				newAddrs := make([]string, len(newGate.StarSystems))
				for i, ss := range newGate.StarSystems {
					newAddrs[i] = ss.Address
				}
				gs.ps.SetAll(newAddrs)
				fmt.Printf("  [ Config          ] %q star systems updated (%d backend(s))\n",
					newGate.Name, len(newAddrs))
			}
		}
	}); err != nil {
		log.Fatalf("  [ Mission Control ] Failed to start config watcher: %v", err)
	}

	fmt.Println("  [ Mission Control ] Ready. Watching for config changes...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n  [ Mission Control ] Shutting down. Safe travels.")
}
