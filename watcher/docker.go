package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// DockerClient wraps the Docker API client
type DockerClient struct {
	cli *client.Client
}

// NewDockerClient creates a new Docker client
func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerClient{cli: cli}, nil
}

// Close closes the Docker client
func (d *DockerClient) Close() error {
	return d.cli.Close()
}

// ListProxyNetworks returns all networks matching the suffix
func (d *DockerClient) ListProxyNetworks(suffix string) ([]string, error) {
	networks, err := d.cli.NetworkList(context.Background(), network.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, n := range networks {
		if strings.HasSuffix(n.Name, suffix) {
			result = append(result, n.Name)
		}
	}
	return result, nil
}

// ConnectToNetwork connects a container to a network
func (d *DockerClient) ConnectToNetwork(networkName, containerName string) error {
	err := d.cli.NetworkConnect(context.Background(), networkName, containerName, nil)
	if err != nil {
		// Check if already connected
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	log.Printf("Connected to %s", networkName)
	return nil
}

// DisconnectFromNetwork disconnects a container from a network
func (d *DockerClient) DisconnectFromNetwork(networkName, containerName string) error {
	err := d.cli.NetworkDisconnect(context.Background(), networkName, containerName, false)
	if err != nil {
		// Check if not connected
		if strings.Contains(err.Error(), "is not connected") {
			return nil
		}
		return err
	}
	log.Printf("Disconnected from %s", networkName)
	return nil
}

// RemoveNetwork removes a network
func (d *DockerClient) RemoveNetwork(networkName string) error {
	err := d.cli.NetworkRemove(context.Background(), networkName)
	if err != nil {
		// Ignore if already gone or still in use
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "has active endpoints") {
			return nil
		}
		return err
	}
	log.Printf("Removed network %s", networkName)
	return nil
}

// ConnectToNetworkWithRetry connects a container to a network with retry logic
func (d *DockerClient) ConnectToNetworkWithRetry(ctx context.Context, networkName, containerName string) error {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := d.ConnectToNetwork(networkName, containerName)
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return d.ConnectToNetwork(networkName, containerName) // Final attempt
}

// GetNetworkContainers returns all containers in a network
func (d *DockerClient) GetNetworkContainers(networkName string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", networkName)

	return d.cli.ContainerList(context.Background(), container.ListOptions{
		Filters: filterArgs,
	})
}

// GetContainerEnv returns environment variables for a container
func (d *DockerClient) GetContainerEnv(containerID string) (map[string]string, error) {
	inspect, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, err
	}

	env := make(map[string]string)
	for _, e := range inspect.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env, nil
}

// GetContainerNetworks returns all network names a container is connected to
func (d *DockerClient) GetContainerNetworks(containerID string) ([]string, error) {
	inspect, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, err
	}

	var networks []string
	for name := range inspect.NetworkSettings.Networks {
		networks = append(networks, name)
	}
	return networks, nil
}

// GetContainerName returns the name of a container by ID
func (d *DockerClient) GetContainerName(containerID string) (string, error) {
	inspect, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(inspect.Name, "/"), nil
}

// GetContainerNameAndStatus returns the name of a container and whether it still exists
func (d *DockerClient) GetContainerNameAndStatus(containerID string) (string, bool) {
	inspect, err := d.cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "", false
	}
	return strings.TrimPrefix(inspect.Name, "/"), true
}

// WatchEvents starts watching Docker events and calls the handler for each relevant event
func (d *DockerClient) WatchEvents(ctx context.Context) (<-chan events.Message, <-chan error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("type", "network")
	filterArgs.Add("type", "container")

	return d.cli.Events(ctx, events.ListOptions{
		Filters: filterArgs,
	})
}

// watchEvents is the main event loop
func watchEvents(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) error {
	eventsChan, errorsChan := docker.WatchEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errorsChan:
			return err

		case event := <-eventsChan:
			handleEvent(ctx, event, docker, caddyMgr, statusMgr, cfg)
		}
	}
}

func handleEvent(ctx context.Context, event events.Message, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {
	switch event.Type {
	case "network":
		handleNetworkEvent(ctx, event, docker, caddyMgr, statusMgr, cfg)
	case "container":
		handleContainerEvent(ctx, event, docker, caddyMgr, statusMgr, cfg)
	}
}

func handleNetworkEvent(ctx context.Context, event events.Message, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {
	networkName := event.Actor.Attributes["name"]

	// Check if network matches pattern
	if !strings.HasSuffix(networkName, cfg.NetworkSuffix) {
		return
	}

	switch event.Action {
	case "create":
		log.Printf("New network detected: %s", networkName)

		// Connect Caddy to network
		if err := docker.ConnectToNetwork(networkName, cfg.CaddyContainer); err != nil {
			log.Printf("Failed to connect to %s: %v", networkName, err)
		}

		// Generate configs for containers in this network (with retry)
		if err := generateConfigsForNetworkWithRetry(ctx, docker, caddyMgr, networkName, cfg); err != nil {
			log.Printf("Failed to generate config for %s: %v", networkName, err)
		}

		// Update status
		statusMgr.Update(caddyMgr.ListConfigs())

	case "destroy":
		log.Printf("Network removed: %s", networkName)

		// Remove config for this network
		if err := caddyMgr.RemoveConfig(networkName); err != nil {
			log.Printf("Failed to remove config for %s: %v", networkName, err)
		}

		// Update status
		statusMgr.Update(caddyMgr.ListConfigs())

	case "connect":
		containerID := event.Actor.Attributes["container"]

		// Resolve container name from ID
		containerName, err := docker.GetContainerName(containerID)
		if err != nil {
			// Container might be gone already
			return
		}

		// Ignore if Caddy itself is connecting
		if containerName == cfg.CaddyContainer {
			return
		}

		log.Printf("Container %s connected to network: %s", containerName, networkName)

		// Generate configs for containers in this network (with retry)
		if err := generateConfigsForNetworkWithRetry(ctx, docker, caddyMgr, networkName, cfg); err != nil {
			log.Printf("Failed to generate config for %s: %v", networkName, err)
		}

		// Update status
		statusMgr.Update(caddyMgr.ListConfigs())

	case "disconnect":
		containerID := event.Actor.Attributes["container"]

		// Try to resolve container name and check if it still exists
		containerName, containerExists := docker.GetContainerNameAndStatus(containerID)
		if containerName == "" {
			// Container is gone, use short ID for logging
			if len(containerID) > 12 {
				containerName = containerID[:12]
			} else {
				containerName = containerID
			}
		}

		// Ignore if Caddy itself is disconnecting
		if containerName == cfg.CaddyContainer {
			return
		}

		log.Printf("Container %s disconnected from network: %s", containerName, networkName)

		// If container still exists (just stopped), don't cleanup - user might start it again
		if containerExists {
			log.Printf("Container %s still exists (stopped), keeping network %s", containerName, networkName)
			return
		}

		// Check if any non-Caddy containers remain in this network
		containers, err := docker.GetNetworkContainers(networkName)
		if err != nil {
			// Network might already be gone
			return
		}

		hasOtherContainers := false
		for _, c := range containers {
			// Check all names (container can have multiple)
			for _, name := range c.Names {
				// Names have leading slash
				if strings.TrimPrefix(name, "/") != cfg.CaddyContainer {
					hasOtherContainers = true
					break
				}
			}
			if hasOtherContainers {
				break
			}
		}

		// If only Caddy (or no containers) remain, disconnect Caddy and cleanup
		if !hasOtherContainers {
			log.Printf("No service containers in %s, cleaning up", networkName)

			// Disconnect Caddy from network
			if err := docker.DisconnectFromNetwork(networkName, cfg.CaddyContainer); err != nil {
				log.Printf("Failed to disconnect from %s: %v", networkName, err)
			}

			// Remove the network (so docker compose down doesn't error)
			if err := docker.RemoveNetwork(networkName); err != nil {
				log.Printf("Failed to remove network %s: %v", networkName, err)
			}

			// Remove config for this network
			if err := caddyMgr.RemoveConfig(networkName); err != nil {
				log.Printf("Failed to remove config for %s: %v", networkName, err)
			}

			// Update status
			statusMgr.Update(caddyMgr.ListConfigs())
		}
	}
}

func handleContainerEvent(ctx context.Context, event events.Message, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {
	if event.Action != "start" {
		return
	}

	containerName := event.Actor.Attributes["name"]

	// Check if this is the Caddy container (match against configured name)
	if containerName == cfg.CaddyContainer {
		log.Printf("Caddy container started: %s - reconnecting all networks...", containerName)

		// Reconnect to all proxy networks (with retry for each)
		networks, err := docker.ListProxyNetworks(cfg.NetworkSuffix)
		if err != nil {
			log.Printf("Failed to list networks: %v", err)
			return
		}

		for _, network := range networks {
			if err := docker.ConnectToNetworkWithRetry(ctx, network, cfg.CaddyContainer); err != nil {
				log.Printf("Failed to connect to %s: %v", network, err)
			}
		}
		return
	}

	// Get container's networks
	networks, err := docker.GetContainerNetworks(event.Actor.ID)
	if err != nil {
		log.Printf("Failed to get networks for %s: %v", containerName, err)
		return
	}

	// Check each network
	hasProxyNetwork := false
	for _, network := range networks {
		if !strings.HasSuffix(network, cfg.NetworkSuffix) {
			continue
		}
		hasProxyNetwork = true

		log.Printf("Container %s started in %s - checking for CADDY_* config...", containerName, network)

		// Generate config for this network (with retry)
		if err := generateConfigsForNetworkWithRetry(ctx, docker, caddyMgr, network, cfg); err != nil {
			log.Printf("Failed to generate config for %s: %v", network, err)
		}
	}

	if !hasProxyNetwork {
		log.Printf("Ignoring container %s: not in any *_proxy network", containerName)
		return
	}

	// Update status
	statusMgr.Update(caddyMgr.ListConfigs())
}
