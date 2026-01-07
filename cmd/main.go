package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/pperesbr/conduit/internal/config"
	"github.com/pperesbr/conduit/internal/manager"
	"github.com/pperesbr/conduit/internal/watcher"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	log.Printf("conduit: starting with config %s", *configPath)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("conduit: failed to load config: %v", err)
	}

	log.Printf("conduit: loaded %d tunnel(s) via %s@%s:%d",
		len(cfg.TunnelConfigs), cfg.SSH.User, cfg.SSH.Host, cfg.SSH.Port)

	mgr := manager.NewManager(&cfg.SSH)

	for _, tunnelCfg := range cfg.TunnelConfigs {
		if err := mgr.Add(tunnelCfg); err != nil {
			log.Printf("conduit: failed to add tunnel %s: %v", tunnelCfg.Name, err)
			continue
		}
		log.Printf("conduit: added tunnel %s (%s:%d -> localhost:%d)",
			tunnelCfg.Name, tunnelCfg.RemoteHost, tunnelCfg.RemotePort, tunnelCfg.LocalPort)
	}

	errors := mgr.StartAll()
	if len(errors) > 0 {
		for name, err := range errors {
			log.Printf("conduit: failed to start tunnel %s: %v", name, err)
		}
	}

	for name, status := range mgr.Status() {
		log.Printf("conduit: tunnel %s status: %s", name, status)
	}

	w, err := watcher.New(*configPath, mgr)
	if err != nil {
		log.Fatalf("conduit: failed to create watcher: %v", err)
	}

	if err := w.Start(); err != nil {
		log.Fatalf("conduit: failed to start watcher: %v", err)
	}

	log.Printf("conduit: watching config file for changes")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("conduit: received signal %s, shutting down...", sig)

	w.Stop()
	mgr.StopAll()

	log.Printf("conduit: stopped")
}
