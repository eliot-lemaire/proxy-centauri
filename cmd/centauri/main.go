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
	"github.com/eliot-lemaire/proxy-centauri/internal/proxy"
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
			p := proxy.New(lb)
			go func(listen string) {
				if err := http.ListenAndServe(listen, p); err != nil {
					log.Printf("  [ Jump Gate ] listener on %s failed: %v", listen, err)
				}
			}(gate.Listen)
			fmt.Printf("  [ Orbital Router  ] %s — listening on %s — ready to route\n", algo, gate.Listen)
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
