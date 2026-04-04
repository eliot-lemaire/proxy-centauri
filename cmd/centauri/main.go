package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/eliot-lemaire/proxy-centauri/internal/config"
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
	configPath := flag.String("config", "centauri.yml", "path to centauri.yml")
	flag.Parse()

	fmt.Println(logo)
	fmt.Println("  [ Mission Control ] Initializing...")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("  [ Mission Control ] Failed to load centauri.yml: %v", err)
	}

	fmt.Printf("  [ Mission Control ] %d jump gate(s) configured\n", len(cfg.JumpGates))
	for _, gate := range cfg.JumpGates {
		fmt.Printf("  [ Jump Gate       ] %q  →  %s  (%s)\n", gate.Name, gate.Listen, gate.Protocol)
		for _, ss := range gate.StarSystems {
			fmt.Printf("  [ Star System     ]     %s\n", ss.Address)
		}
	}

	if err := config.Watch(*configPath, func(newCfg *config.Config) {
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
