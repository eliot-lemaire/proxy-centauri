package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eliot-lemaire/proxy-centauri/internal/balancer"
	"github.com/eliot-lemaire/proxy-centauri/internal/config"
	"github.com/eliot-lemaire/proxy-centauri/internal/health"
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

         ✦  Navigating the cosmos, one request at a time  ✦
              v0.1.0 — Milestone 1: First Contact
`

func main() {
	fmt.Println(logo)
	fmt.Println("  [ Mission Control ] Initializing...")

	cfg, err := config.Load("centauri.yml")
	if err != nil {
		log.Fatalf("  [ Mission Control ] Failed to load centauri.yml: %v", err)
	}

	fmt.Printf("  [ Mission Control ] %d jump gate(s) configured\n", len(cfg.JumpGates))
	for _, gate := range cfg.JumpGates {
		fmt.Printf("  [ Jump Gate       ] %q  →  %s  (%s)\n", gate.Name, gate.Listen, gate.Protocol)

		addrs := make([]string, len(gate.StarSystems))
		for i, ss := range gate.StarSystems {
			addrs[i] = ss.Address
			fmt.Printf("  [ Star System     ]     %s\n", ss.Address)
		}

		lb := balancer.New(addrs)
		fmt.Printf("  [ Orbital Router  ] %d star system(s) registered\n", lb.Len())

		ps := health.New(addrs, lb, 5*time.Second)
		ps.Start()
		fmt.Printf("  [ Pulse Scan      ] health checks every 5s\n")
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
