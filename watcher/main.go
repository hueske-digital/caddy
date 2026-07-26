package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	log.SetPrefix("[watcher] ")
	log.SetFlags(log.Ldate | log.Ltime)

	// Load configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create Docker client
	docker, err := NewDockerClient()
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}
	defer docker.Close()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create allowlist manager with DNS refresh
	// onChange will be set after caddyMgr is created
	allowlistMgr := NewAllowlistManager(cfg.DNSRefreshInterval, nil)

	// Create managers
	caddyMgr := NewCaddyManager(cfg.HostsDir, allowlistMgr)
	caddyMgr.absentGrace = cfg.CleanupGrace
	log.Printf("Stale config cleanup grace: %s", cfg.CleanupGrace)
	statusDomain := os.Getenv("CADDY_DOMAIN")
	statusMgr := NewStatusManager(cfg.CodeEditorURL, statusDomain)

	// Start status server only if CADDY_DOMAIN is set (watcher discovers itself)
	if os.Getenv("CADDY_DOMAIN") != "" {
		statusServer := NewStatusServer(statusMgr, caddyMgr, 8080)
		statusServer.Start()
	}

	// Set onChange callback for allowlist manager
	allowlistMgr.onChange = func(configKey string) {
		network := allowlistMgr.GetNetwork(configKey)
		if network == "" {
			log.Printf("Warning: no network found for %s", configKey)
			return
		}
		log.Printf("Allowlist IPs changed for %s, regenerating config...", configKey)
		if err := regenerateConfigForNetwork(ctx, docker, caddyMgr, network, cfg); err != nil {
			log.Printf("Failed to regenerate config for %s: %v", network, err)
		}
	}

	// Start allowlist DNS refresh goroutine
	go allowlistMgr.Start(ctx)

	// Handle SIGINT/SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	// Generate wildcard certificate configs if configured
	if len(cfg.WildcardDomains) > 0 {
		log.Printf("Generating wildcard configs for: %v (DNS: %s)", cfg.WildcardDomains, cfg.WildcardDNSProvider)
		if err := caddyMgr.WriteWildcardConfigs(cfg.WildcardDomains, cfg.WildcardDNSProvider); err != nil {
			log.Printf("Warning: failed to write wildcard configs: %v", err)
		}
		statusMgr.SetWildcardDomains(cfg.WildcardDomains)
	}

	// Initial processing of existing networks
	log.Println("Starting up, processing existing networks...")
	if err := processExistingNetworks(ctx, docker, caddyMgr, statusMgr, cfg); err != nil {
		log.Printf("Warning during initial processing: %v", err)
	}
	statusMgr.Update(caddyMgr.ListConfigs())
	log.Println("Initial processing complete")

	// Start cleanup loop for orphaned networks (every 5 minutes)
	go startCleanupLoop(ctx, docker, caddyMgr, statusMgr, cfg)

	// Start event loop
	log.Println("Watching for events...")
	if err := watchEvents(ctx, docker, caddyMgr, statusMgr, cfg); err != nil && err != context.Canceled {
		log.Fatalf("Event watcher error: %v", err)
	}
}

func processExistingNetworks(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) error {
	networks, err := docker.ListProxyNetworks(cfg.NetworkSuffix)
	if err != nil {
		return err
	}

	for _, network := range networks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Connect Caddy to this network
		if err := docker.ConnectToNetwork(network, cfg.CaddyContainer); err != nil {
			log.Printf("Failed to connect to %s: %v", network, err)
		}

		// Generate configs for containers in this network
		if err := generateConfigsForNetwork(ctx, docker, caddyMgr, network, cfg); err != nil {
			log.Printf("Failed to generate config for %s: %v", network, err)
		}
	}

	return nil
}

// reconcileMu serialisiert die Abgleiche. Sie werden aus drei Richtungen
// angestossen - Docker-Events, Cleanup-Loop und DNS-Aenderungen - und jeder
// Lauf baut sich vorher einen eigenen Schnappschuss der Container. Ohne
// Serialisierung koennte ein langsamer alter Lauf am Ende eine Konfiguration
// entfernen, die ein neuerer Lauf gerade erst geschrieben hat.
var reconcileMu sync.Mutex

func generateConfigsForNetwork(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, network string, cfg *Config) error {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()

	containers, err := docker.GetNetworkContainers(network)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		log.Printf("No containers in network %s", network)
		return nil
	}

	// keep sammelt alle Config-Keys, die dieses Netzwerk aktuell noch haben
	// soll. Alles andere wird am Ende entfernt, damit eine entfernte
	// CADDY_DOMAIN nicht als verwaiste Datei weiterroutet.
	keep := make(map[string]bool)

	// Aufgeraeumt wird nur, wenn wirklich jeder Container zugeordnet werden
	// konnte. Ein einziger nicht zuordenbarer Container macht "keep"
	// unvollstaendig - dann waere Pruning ein Loeschen auf Verdacht.
	inventoryComplete := true

	// Container, deren Konfiguration nicht gelesen werden konnte. Ihre Dateien
	// bleiben unangetastet - unabhaengig davon, ob der Watcher sie im Speicher
	// hat. Ohne das wuerde nach einem Neustart ein einziger fehlgeschlagener
	// Inspect reichen, um eine laufende Site sofort abzuraeumen.
	protected := make(map[string]bool)

	for _, container := range containers {
		containerName := ""
		if len(container.Names) > 0 {
			containerName = strings.TrimPrefix(container.Names[0], "/")
		}

		// Skip caddy container
		if containerName == cfg.CaddyContainer {
			continue
		}

		// Container ohne Namen kann nicht adressiert werden - Docker liefert das
		// bei Containern, die waehrend der Abfrage verschwinden.
		if containerName == "" {
			log.Printf("Skipping container %s (no name)", container.ID[:12])
			inventoryComplete = false
			continue
		}
		// Get container environment variables
		env, err := docker.GetContainerEnv(container.ID)
		if err != nil {
			// Transienter Docker-Fehler: bestehende Configs dieses Containers
			// behalten, statt eine laufende Site abzuraeumen.
			log.Printf("Failed to inspect container %s: %v", container.ID[:12], err)
			protected[containerName] = true
			continue
		}

		// Parse CADDY_* variables (supports both single and multi-service modes)
		configs, err := ParseAllCaddyEnv(env, network, containerName)
		if err != nil {
			// Fehlerhafte ENV ist meist ein Tippfehler. Die letzte gueltige
			// Konfiguration bleibt bestehen, damit ein Tippfehler nicht die
			// Site offline nimmt.
			log.Printf("Invalid config for %s: %v", containerName, err)
			protected[containerName] = true
			continue
		}
		if configs == nil {
			continue // No CADDY_* variables - darf aufgeraeumt werden
		}

		// Process each config (single-service: 1 config, multi-service: multiple configs)
		for _, config := range configs {
			// Register allowlist if present (for DNS refresh)
			if len(config.Allowlist) > 0 && caddyMgr.allowlistManager != nil {
				caddyMgr.allowlistManager.Register(config)
			}

			// Write config
			if err := caddyMgr.WriteConfig(config); err != nil {
				log.Printf("Failed to write config for %s: %v", config.ConfigKey(), err)
				keep[config.ConfigKey()] = true // nicht wegen Schreibfehler loeschen
				continue
			}
			keep[config.ConfigKey()] = true
			log.Printf("Generated config: %s/%s.conf", config.Type, config.ConfigKey())
		}
	}

	// Bei unvollstaendigem Bestand wird nicht geloescht - die Uhren muessen
	// aber trotzdem gepflegt werden, damit eine zurueckgekehrte Config ihre
	// alte Uhr verliert.
	if !inventoryComplete {
		log.Printf("Skipping cleanup for %s (container inventory incomplete)", network)
	}

	pruned, err := caddyMgr.PruneNetwork(network, keep, protected, inventoryComplete, time.Now())
	if err != nil {
		return err
	}
	for _, key := range pruned {
		log.Printf("Removed stale config: %s.conf (no longer declared)", key)
	}

	return nil
}

// regenerateConfigForNetwork regenerates config for a specific network (used when DNS changes)
func regenerateConfigForNetwork(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, network string, cfg *Config) error {
	return generateConfigsForNetwork(ctx, docker, caddyMgr, network, cfg)
}
