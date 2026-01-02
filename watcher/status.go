package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// ServiceStatus represents a service in the status
type ServiceStatus struct {
	Network     string   `json:"network"`
	Container   string   `json:"container,omitempty"`
	Type        string   `json:"type"`
	Domains     []string `json:"domains"`
	Allowlist   []string `json:"allowlist,omitempty"`
	Logging     bool     `json:"logging"`
	TLS         bool     `json:"tls"`
	Compression bool     `json:"compression"`
	Header      bool     `json:"header"`
	Auth        bool     `json:"auth"`
	Managed     bool     `json:"managed"`
	ConfigPath  string   `json:"configPath,omitempty"`
}

// Status represents the status structure
type Status struct {
	Services      []ServiceStatus `json:"services"`
	Summary       StatusSummary   `json:"summary"`
	Updated       string          `json:"updated"`
	CodeEditorURL string          `json:"codeEditorUrl,omitempty"`
}

// StatusSummary provides quick stats
type StatusSummary struct {
	Total      int `json:"total"`
	Managed    int `json:"managed"`
	Manual     int `json:"manual"`
	External   int `json:"external"`
	Internal   int `json:"internal"`
	Cloudflare int `json:"cloudflare"`
}

// StatusManager manages the status in memory
type StatusManager struct {
	current       *Status
	codeEditorURL string
	mu            sync.RWMutex
}

// NewStatusManager creates a new StatusManager
func NewStatusManager(codeEditorURL string) *StatusManager {
	return &StatusManager{
		codeEditorURL: codeEditorURL,
		current: &Status{
			Services:      []ServiceStatus{},
			Updated:       time.Now().Format(time.RFC3339),
			CodeEditorURL: codeEditorURL,
		},
	}
}

// Update updates the in-memory status
func (m *StatusManager) Update(configs []ConfigInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var services []ServiceStatus

	// Add all configs (managed and manual)
	for _, cfg := range configs {
		services = append(services, ServiceStatus{
			Network:     cfg.Network,
			Container:   cfg.Container,
			Type:        cfg.Type,
			Domains:     cfg.Domains,
			Allowlist:   cfg.Allowlist,
			Logging:     cfg.Logging,
			TLS:         cfg.TLS,
			Compression: cfg.Compression,
			Header:      cfg.Header,
			Auth:        cfg.Auth,
			Managed:     cfg.Managed,
			ConfigPath:  cfg.Path,
		})
	}

	// Calculate summary
	summary := StatusSummary{
		Total: len(services),
	}
	for _, svc := range services {
		if svc.Managed {
			summary.Managed++
		} else {
			summary.Manual++
		}
		switch svc.Type {
		case "external":
			summary.External++
		case "internal":
			summary.Internal++
		case "cloudflare":
			summary.Cloudflare++
		}
	}

	m.current = &Status{
		Services:      services,
		Summary:       summary,
		Updated:       time.Now().Format(time.RFC3339),
		CodeEditorURL: m.codeEditorURL,
	}
}

// GetJSON returns the current status as JSON bytes
func (m *StatusManager) GetJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return json.MarshalIndent(m.current, "", "  ")
}

// extractDomainsFromContent extracts domains from config content
func extractDomainsFromContent(content string) []string {
	var domains []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") {
			line = strings.TrimSuffix(line, "{")
			line = strings.TrimSpace(line)
			parts := strings.Split(line, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				part = strings.TrimPrefix(part, "https://")
				if part != "" {
					domains = append(domains, part)
				}
			}
			break
		}
	}

	return domains
}
