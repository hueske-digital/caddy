package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// DoH endpoints
var dohEndpoints = []string{
	"https://1.1.1.1/dns-query",         // Cloudflare
	"https://dns.google/dns-query",       // Google
}

// HTTP client for DoH requests
var dohClient = &http.Client{
	Timeout: 10 * time.Second,
}

// dohResponse represents the JSON response from DoH
type dohResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		Data string `json:"data"`
	} `json:"Answer"`
}

// resolveDoH resolves a hostname using DNS over HTTPS
func resolveDoH(hostname string) ([]string, error) {
	var lastErr error

	for _, endpoint := range dohEndpoints {
		ips, err := queryDoH(endpoint, hostname)
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no IPs found for %s", hostname)
}

// queryDoH queries a single DoH endpoint
func queryDoH(endpoint, hostname string) ([]string, error) {
	ips, err := queryDoHType(endpoint, hostname, "A")
	if err != nil {
		return nil, err
	}

	// Also query AAAA for IPv6
	ipv6, _ := queryDoHType(endpoint, hostname, "AAAA")
	ips = append(ips, ipv6...)

	return ips, nil
}

// queryDoHType queries a specific record type
func queryDoHType(endpoint, hostname, recordType string) ([]string, error) {
	url := fmt.Sprintf("%s?name=%s&type=%s", endpoint, hostname, recordType)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")

	resp, err := dohClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH request failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var dohResp dohResponse
	if err := json.Unmarshal(body, &dohResp); err != nil {
		return nil, err
	}

	var ips []string
	for _, answer := range dohResp.Answer {
		// Type 1 = A record (IPv4), Type 28 = AAAA record (IPv6)
		if answer.Type == 1 || answer.Type == 28 {
			ips = append(ips, answer.Data)
		}
	}

	return ips, nil
}

// AllowlistEntry represents a hostname or IP in the allowlist
type AllowlistEntry struct {
	Original    string   // Original entry (hostname or IP)
	ResolvedIPs []string // Resolved IPs (for hostnames) or just the IP
	IsHostname  bool
}

// AllowlistManager manages DNS resolution for allowlists
type AllowlistManager struct {
	configs         map[string]*CaddyConfig // network -> config
	resolvedIPs     map[string][]string     // network -> resolved IPs
	refreshInterval time.Duration
	onChange        func(network string) // Called when IPs change for a network
	mu              sync.RWMutex
}

// NewAllowlistManager creates a new AllowlistManager
func NewAllowlistManager(refreshInterval int, onChange func(network string)) *AllowlistManager {
	return &AllowlistManager{
		configs:         make(map[string]*CaddyConfig),
		resolvedIPs:     make(map[string][]string),
		refreshInterval: time.Duration(refreshInterval) * time.Second,
		onChange:        onChange,
	}
}

// Register adds or updates an allowlist for a config
func (m *AllowlistManager) Register(cfg *CaddyConfig) {
	if len(cfg.Allowlist) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := cfg.ConfigKey()
	m.configs[key] = cfg
	// Resolve immediately
	resolved := m.resolveAllowlist(cfg.Allowlist)
	m.resolvedIPs[key] = resolved
	log.Printf("Registered allowlist for %s: %v -> %v", key, cfg.Allowlist, resolved)
}

// Unregister removes an allowlist for a config key
func (m *AllowlistManager) Unregister(configKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.configs, configKey)
	delete(m.resolvedIPs, configKey)
}

// GetResolvedIPs returns the current resolved IPs for a config key
func (m *AllowlistManager) GetResolvedIPs(configKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.resolvedIPs[configKey]
}

// GetEntries returns the original allowlist entries for a config key
func (m *AllowlistManager) GetEntries(configKey string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cfg, ok := m.configs[configKey]; ok {
		return cfg.Allowlist
	}
	return nil
}

// Start begins periodic DNS resolution
func (m *AllowlistManager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshAll()
		}
	}
}

// refreshAll resolves all registered allowlists and checks for changes
func (m *AllowlistManager) refreshAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for configKey, cfg := range m.configs {
		oldResolved := m.resolvedIPs[configKey]
		newResolved := m.resolveAllowlistWithFallback(cfg.Allowlist, oldResolved)

		if !equalStringSlices(oldResolved, newResolved) {
			log.Printf("DNS changed for %s: %v -> %v", configKey, oldResolved, newResolved)
			m.resolvedIPs[configKey] = newResolved

			// Notify about change (in goroutine to avoid deadlock)
			if m.onChange != nil {
				go m.onChange(configKey)
			}
		}
	}
}

// resolveAllowlist resolves all entries in an allowlist to IPs
func (m *AllowlistManager) resolveAllowlist(entries []string) []string {
	var allIPs []string
	seen := make(map[string]bool)

	for _, entry := range entries {
		ips := m.resolveEntry(entry)
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	// Sort for consistent comparison
	sort.Strings(allIPs)
	return allIPs
}

// resolveAllowlistWithFallback resolves allowlist entries, keeping previous IPs on failure
func (m *AllowlistManager) resolveAllowlistWithFallback(entries []string, previousIPs []string) []string {
	newResolved := m.resolveAllowlist(entries)

	// If resolution returned nothing but we had previous IPs, keep them
	if len(newResolved) == 0 && len(previousIPs) > 0 {
		log.Printf("DNS resolution failed, keeping previous IPs: %v", previousIPs)
		return previousIPs
	}

	return newResolved
}

// resolveEntry resolves a single entry (hostname or IP) to IPs
func (m *AllowlistManager) resolveEntry(entry string) []string {
	// Check if it's already an IP (v4 or v6) or CIDR
	if net.ParseIP(entry) != nil {
		return []string{entry}
	}

	// Check if it's a CIDR
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return []string{entry}
	}

	// It's a hostname, resolve it using DNS over HTTPS (no caching)
	ips, err := resolveDoH(entry)
	if err != nil {
		log.Printf("Failed to resolve %s via DoH: %v", entry, err)
		return nil
	}

	return ips
}

// equalStringSlices compares two string slices for equality
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FormatAllowlistMatcher formats resolved IPs as Caddy remote_ip matcher
func FormatAllowlistMatcher(ips []string) string {
	if len(ips) == 0 {
		return ""
	}
	return strings.Join(ips, " ")
}
