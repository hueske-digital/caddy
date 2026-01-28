package main

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// ServiceStatus represents a service in the status
type ServiceStatus struct {
	Network        string   `json:"network"`
	Container      string   `json:"container,omitempty"`
	Type           string   `json:"type"`
	Domains        []string `json:"domains"`
	Allowlist      []string `json:"allowlist,omitempty"`
	TrustedProxies []string `json:"trustedProxies,omitempty"`
	Logging        bool     `json:"logging"`
	DNSProvider    string   `json:"dnsProvider,omitempty"`
	Compression    bool     `json:"compression"`
	Header         bool     `json:"header"`
	Auth           bool     `json:"auth"`
	AuthPaths      []string `json:"authPaths,omitempty"`
	AuthExcept     []string `json:"authExcept,omitempty"`
	AuthGroups     []string `json:"authGroups,omitempty"`
	AuthURL        string   `json:"authUrl,omitempty"`
	SEO             bool     `json:"seo"`
	SEONoindexTypes []string `json:"seoNoindexTypes,omitempty"`
	WWWRedirect     bool     `json:"wwwRedirect"`
	Performance    bool     `json:"performance"`
	Security       bool     `json:"security"`
	WordPress      bool     `json:"wordpress"`
	Managed        bool     `json:"managed"`
	ConfigPath     string   `json:"configPath,omitempty"`
}

// Status represents the status structure
type Status struct {
	Services        []ServiceStatus `json:"services"`
	WildcardDomains []string        `json:"wildcardDomains,omitempty"`
	Summary         StatusSummary   `json:"summary"`
	Updated         string          `json:"updated"`
	CodeEditorURL   string          `json:"codeEditorUrl,omitempty"`
	StatusDomain    string          `json:"statusDomain,omitempty"`
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
	current         *Status
	codeEditorURL   string
	statusDomain    string
	wildcardDomains []string
	mu              sync.RWMutex
}

// NewStatusManager creates a new StatusManager
func NewStatusManager(codeEditorURL string, statusDomain string) *StatusManager {
	return &StatusManager{
		codeEditorURL: codeEditorURL,
		statusDomain:  statusDomain,
		current: &Status{
			Services:      []ServiceStatus{},
			Updated:       time.Now().Format(time.RFC3339),
			CodeEditorURL: codeEditorURL,
			StatusDomain:  statusDomain,
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
			Network:        cfg.Network,
			Container:      cfg.Container,
			Type:           cfg.Type,
			Domains:        cfg.Domains,
			Allowlist:      cfg.Allowlist,
			TrustedProxies: cfg.TrustedProxies,
			Logging:        cfg.Logging,
			DNSProvider:    cfg.DNSProvider,
			Compression:    cfg.Compression,
			Header:         cfg.Header,
			Auth:           cfg.Auth,
			AuthPaths:      cfg.AuthPaths,
			AuthExcept:     cfg.AuthExcept,
			AuthGroups:     cfg.AuthGroups,
			AuthURL:        cfg.AuthURL,
			SEO:             cfg.SEO,
			SEONoindexTypes: cfg.SEONoindexTypes,
			WWWRedirect:     cfg.WWWRedirect,
			Performance:    cfg.Performance,
			Security:       cfg.Security,
			WordPress:      cfg.WordPress,
			Managed:        cfg.Managed,
			ConfigPath:     cfg.Path,
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
		case TypeExternal:
			summary.External++
		case TypeInternal:
			summary.Internal++
		case TypeCloudflare:
			summary.Cloudflare++
		}
	}

	m.current = &Status{
		Services:        services,
		WildcardDomains: m.wildcardDomains,
		Summary:         summary,
		Updated:         time.Now().Format(time.RFC3339),
		CodeEditorURL:   m.codeEditorURL,
		StatusDomain:    m.statusDomain,
	}
}

// SetWildcardDomains updates the wildcard domains in the status
func (m *StatusManager) SetWildcardDomains(domains []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wildcardDomains = domains
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
