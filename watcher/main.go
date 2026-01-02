package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
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
	statusMgr := NewStatusManager(cfg.CodeEditorURL)

	// Start status server only if CADDY_DOMAIN is set (watcher discovers itself)
	if os.Getenv("CADDY_DOMAIN") != "" {
		statusServer := NewStatusServer(statusMgr, caddyMgr, 8080)
		statusServer.Start()
	}

	// Set onChange callback for allowlist manager
	// configKey format is "container_network"
	allowlistMgr.onChange = func(configKey string) {
		// Extract network from configKey (everything after the last underscore)
		network := configKey
		if idx := strings.LastIndex(configKey, "_"); idx > 0 {
			network = configKey[idx+1:]
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
		log.Printf("Generating wildcard configs for: %v", cfg.WildcardDomains)
		if err := caddyMgr.WriteWildcardConfigs(cfg.WildcardDomains); err != nil {
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

func generateConfigsForNetwork(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, network string, cfg *Config) error {
	containers, err := docker.GetNetworkContainers(network)
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		log.Printf("No containers in network %s", network)
		return nil
	}

	for _, container := range containers {
		containerName := ""
		if len(container.Names) > 0 {
			containerName = strings.TrimPrefix(container.Names[0], "/")
		}

		// Skip caddy container
		if containerName == cfg.CaddyContainer {
			continue
		}

		// Get container environment variables
		env, err := docker.GetContainerEnv(container.ID)
		if err != nil {
			log.Printf("Failed to inspect container %s: %v", container.ID[:12], err)
			continue
		}

		// Parse CADDY_* variables
		config, err := ParseCaddyEnv(env, network, container.Names[0])
		if err != nil {
			log.Printf("Invalid config for %s: %v", containerName, err)
			continue
		}
		if config == nil {
			continue // No CADDY_* variables, skip silently
		}

		// Register allowlist if present (for DNS refresh)
		if len(config.Allowlist) > 0 && caddyMgr.allowlistManager != nil {
			caddyMgr.allowlistManager.Register(config)
		}

		// Write config
		if err := caddyMgr.WriteConfig(config); err != nil {
			log.Printf("Failed to write config for %s: %v", config.ConfigKey(), err)
			continue
		}
		log.Printf("Generated config: %s/%s.conf", config.Type, config.ConfigKey())
	}

	return nil
}

// regenerateConfigForNetwork regenerates config for a specific network (used when DNS changes)
func regenerateConfigForNetwork(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, network string, cfg *Config) error {
	return generateConfigsForNetwork(ctx, docker, caddyMgr, network, cfg)
}

func generateConfigsForNetworkWithRetry(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, network string, cfg *Config) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		containers, err := docker.GetNetworkContainers(network)
		if err != nil {
			lastErr = err
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		// If we found containers, process them
		if len(containers) > 0 {
			return generateConfigsForNetwork(ctx, docker, caddyMgr, network, cfg)
		}

		// No containers yet, retry
		if i < maxRetries-1 {
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Final attempt
	if lastErr != nil {
		return lastErr
	}
	return generateConfigsForNetwork(ctx, docker, caddyMgr, network, cfg)
}
