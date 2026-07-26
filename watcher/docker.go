package main

import (
	"context"
	"fmt"
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
		// Container not running
		if strings.Contains(err.Error(), "network sandbox") {
			return fmt.Errorf("container %s not running (check config errors)", containerName)
		}
		return err
	}
	log.Printf("Caddy connected to %s", networkName)
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
	log.Printf("Caddy disconnected from %s", networkName)
	return nil
}

// RemoveNetwork removes a Docker network. It returns an error if the network
// still has attached endpoints (Docker refuses to remove non-empty networks),
// which makes this safe to call optimistically on orphaned networks.
func (d *DockerClient) RemoveNetwork(networkName string) error {
	return d.cli.NetworkRemove(context.Background(), networkName)
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

// GetNetworkContainers returns the running containers in a network.
func (d *DockerClient) GetNetworkContainers(networkName string) ([]types.Container, error) {
	return d.listNetworkContainers(networkName, false)
}

// GetNetworkContainerNames returns the names of ALL containers in a network,
// unabhaengig vom Zustand.
//
// Fuer die Frage "gibt es diesen Container noch?" ist das die richtige Liste:
// ContainerList liefert per Default nur laufende Container, ein Container in
// created, restarting, paused oder exited fehlt dort also. Wuerde daraus auf
// "entfernt" geschlossen, verlaere ein bloss gestoppter Dienst nach Ablauf der
// Karenz seine Konfiguration, obwohl er noch existiert.
func (d *DockerClient) GetNetworkContainerNames(networkName string) (map[string]bool, error) {
	containers, err := d.listNetworkContainers(networkName, true)
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool, len(containers))
	for _, c := range containers {
		for _, n := range c.Names {
			if trimmed := strings.TrimPrefix(n, "/"); trimmed != "" {
				names[trimmed] = true
			}
		}
	}
	return names, nil
}

func (d *DockerClient) listNetworkContainers(networkName string, all bool) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("network", networkName)

	return d.cli.ContainerList(context.Background(), container.ListOptions{
		All:     all,
		Filters: filterArgs,
	})
}

// NetworkExists reports whether a network is still present.
//
// Im Zweifel wird "vorhanden" gemeldet. Nur ein ausdrueckliches "not found" ist
// ein Beleg dafuer, dass das Netzwerk weg ist - ein Daemon-, Timeout- oder
// Berechtigungsfehler ist keiner. Wuerde jeder Fehler als Abwesenheit gewertet,
// koennte eine Stoerung der Docker-API ueber die Karenz hinweg dazu fuehren,
// dass Konfigurationen bestehender Netzwerke geloescht werden.
func (d *DockerClient) NetworkExists(networkName string) bool {
	_, err := d.cli.NetworkInspect(context.Background(), networkName, network.InspectOptions{})
	if err == nil {
		return true
	}
	if client.IsErrNotFound(err) {
		return false
	}
	log.Printf("Cannot determine whether network %s still exists (%v) - assuming it does", networkName, err)
	return true
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

// IsContainerRunning checks if a container is running by name
func (d *DockerClient) IsContainerRunning(containerName string) bool {
	inspect, err := d.cli.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return false
	}
	return inspect.State != nil && inspect.State.Running
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

// cleanupGraceCycles is the number of consecutive cleanup cycles a network must
// be orphaned (no running service containers) before the watcher removes the
// network itself. At one cycle per 5 minutes this means a network must stay
// empty for ~15 minutes, far longer than any container restart or Watchtower
// update gap, which keeps removal safe from transient emptiness.
const cleanupGraceCycles = 3

// startCleanupLoop runs periodic cleanup of orphaned networks
func startCleanupLoop(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {
	log.Println("Cleanup loop scheduled (every 5 minutes)")
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// orphanCycles tracks how many consecutive cleanup cycles each network has
	// been orphaned. This state is intentionally in-memory only: losing it on a
	// watcher restart merely restarts the grace count, which delays removal but
	// can never cause a premature one.
	orphanCycles := make(map[string]int)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupOrphanedNetworks(ctx, docker, caddyMgr, statusMgr, cfg, orphanCycles)
		}
	}
}

// cleanupOrphanedNetworks finds and cleans up networks with no service containers
func cleanupOrphanedNetworks(ctx context.Context, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config, orphanCycles map[string]int) {
	// Skip cleanup if Caddy container is not running (e.g., during Watchtower updates)
	// This prevents race conditions where networks are removed while containers are restarting
	if !docker.IsContainerRunning(cfg.CaddyContainer) {
		log.Printf("Cleanup: skipping - Caddy container %s is not running", cfg.CaddyContainer)
		return
	}

	networks, err := docker.ListProxyNetworks(cfg.NetworkSuffix)
	if err != nil {
		log.Printf("Cleanup: failed to list networks: %v", err)
		return
	}

	seen := make(map[string]bool, len(networks))
	for _, networkName := range networks {
		seen[networkName] = true

		containers, err := docker.GetNetworkContainers(networkName)
		if err != nil {
			continue
		}

		// Check if only Caddy (or no containers) remain
		hasServiceContainers := false
		for _, c := range containers {
			for _, name := range c.Names {
				if strings.TrimPrefix(name, "/") != cfg.CaddyContainer {
					hasServiceContainers = true
					break
				}
			}
			if hasServiceContainers {
				break
			}
		}

		if hasServiceContainers {
			// Network is in use again - reset its orphan counter.
			delete(orphanCycles, networkName)

			// Periodischer Abgleich. Ohne ihn haengt das Aufraeumen allein an
			// Docker-Events: verschwindet ein Container dauerhaft aus einem
			// Netz, in dem andere weiterlaufen, faellt kein Event an, das einen
			// Reconcile ausloest - seine Config bliebe unbegrenzt liegen.
			// WriteConfig schreibt nur bei echter Aenderung, der Durchlauf
			// loest also keinen Reload aus, wenn sich nichts geaendert hat.
			if err := generateConfigsForNetwork(ctx, docker, caddyMgr, networkName, cfg); err != nil {
				log.Printf("Cleanup: failed to reconcile %s: %v", networkName, err)
			}
			continue
		}

		// Track how long the network has been orphaned. Nichts wird sofort
		// entfernt: waehrend eines Container-Neustarts oder eines
		// Watchtower-Updates ist ein Netzwerk kurzzeitig leer, und ein
		// sofortiges Aufraeumen wuerde eine Site abraeumen, die Sekunden
		// spaeter zurueckkommt.
		//
		// Die Karenz gilt bewusst auch fuer die KONFIGURATION, nicht nur fuer
		// das Netzwerk: die Datei zu loeschen loest einen Caddy-Reload aus und
		// nimmt die Domain aus dem Proxy - genau fuer die Dauer des Updates.
		// Der regulaere Teardown ist davon unberuehrt, "docker compose down"
		// feuert network:destroy und raeumt sofort auf.
		orphanCycles[networkName]++
		if orphanCycles[networkName] < cleanupGraceCycles {
			continue
		}

		// Karenz abgelaufen. Config entfernen (idempotent) und Caddy
		// abhaengen, aber nur loggen/Status aktualisieren, wenn sich wirklich
		// etwas geaendert hat - sonst wird jede Runde neu gemeldet.
		removed, err := caddyMgr.RemoveConfig(networkName)
		if err != nil {
			log.Printf("Cleanup: failed to remove config for %s: %v", networkName, err)
		} else if removed {
			log.Printf("Cleanup: network %s empty for %d cycles, removed config", networkName, orphanCycles[networkName])
			statusMgr.Update(caddyMgr.ListConfigs())
		}

		// Make sure Caddy is detached, otherwise the network still has an
		// active endpoint and removal would fail.
		if err := docker.DisconnectFromNetwork(networkName, cfg.CaddyContainer); err != nil {
			log.Printf("Cleanup: failed to disconnect from %s: %v", networkName, err)
		}
		if err := docker.RemoveNetwork(networkName); err != nil {
			// Network still has endpoints (e.g. a stopped container) or is
			// otherwise in use - leave it alone and retry next cycle. Only
			// log on the first attempt to avoid repeating noise.
			if orphanCycles[networkName] == cleanupGraceCycles {
				log.Printf("Cleanup: network %s orphaned for %d cycles but not removable yet: %v", networkName, orphanCycles[networkName], err)
			}
		} else {
			log.Printf("Cleanup: removed orphaned network %s (empty for %d cycles)", networkName, orphanCycles[networkName])
			delete(orphanCycles, networkName)
		}
	}

	// Drop counters for networks that no longer exist (e.g. removed by Compose).
	for name := range orphanCycles {
		if !seen[name] {
			delete(orphanCycles, name)
		}
	}

	// Configs aufraeumen, deren Netzwerk gar nicht mehr existiert. Das faengt
	// verpasste network:destroy-Events ab - etwa wenn ein Projekt abgeraeumt
	// wird, waehrend Watchtower gerade den Watcher aktualisiert. "seen" stammt
	// aus der oben erfolgreich abgefragten Netzwerkliste; bei einem Fehler ist
	// diese Funktion vorher schon zurueckgekehrt.
	pruned, err := caddyMgr.PruneUnknownNetworks(seen, docker.NetworkExists, time.Now())
	if err != nil {
		log.Printf("Cleanup: failed to remove configs of vanished networks: %v", err)
	}
	if len(pruned) > 0 {
		for _, key := range pruned {
			log.Printf("Cleanup: removed %s.conf (network no longer exists)", key)
		}
		statusMgr.Update(caddyMgr.ListConfigs())
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

		// Connect Caddy to network (configs are generated via network:connect event)
		if err := docker.ConnectToNetwork(networkName, cfg.CaddyContainer); err != nil {
			log.Printf("Failed to connect to %s: %v", networkName, err)
		}

	case "destroy":
		log.Printf("Network removed: %s", networkName)

		// Remove config for this network
		if _, err := caddyMgr.RemoveConfig(networkName); err != nil {
			log.Printf("Failed to remove config for %s: %v", networkName, err)
		} else {
			log.Printf("Removed config for %s", networkName)
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

		// Ensure Caddy itself is attached to this network. The cleanup loop
		// disconnects Caddy from networks that temporarily have no running
		// service containers; without this, a restarting service would get a
		// regenerated config while Caddy is still detached, causing 502s.
		// A single attempt is enough here: the network exists (a container just
		// joined it) and ConnectToNetwork is a no-op if Caddy is already attached.
		// If Caddy is down, its own start handler reconnects all networks anyway.
		if err := docker.ConnectToNetwork(networkName, cfg.CaddyContainer); err != nil {
			log.Printf("Failed to reconnect Caddy to %s: %v", networkName, err)
		}

		// Generate config for this container
		if err := generateConfigsForNetwork(ctx, docker, caddyMgr, networkName, cfg); err != nil {
			log.Printf("Failed to generate config for %s: %v", networkName, err)
		}

		// Update status
		statusMgr.Update(caddyMgr.ListConfigs())

	case "disconnect":
		containerID := event.Actor.Attributes["container"]

		// Try to resolve container name
		containerName, _ := docker.GetContainerNameAndStatus(containerID)
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
		// Cleanup happens via network:destroy event when docker compose down runs
	}
}

func handleContainerEvent(ctx context.Context, event events.Message, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {
	if event.Action == "start" {
		handleContainerStart(ctx, event, docker, caddyMgr, statusMgr, cfg)
	}
}

func handleContainerStart(ctx context.Context, event events.Message, docker *DockerClient, caddyMgr *CaddyManager, statusMgr *StatusManager, cfg *Config) {

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

	// For non-Caddy containers, config generation is handled by network:connect event
	// No need to duplicate it here
}
