package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Type constants for CADDY_TYPE
const (
	TypeInternal   = "internal"
	TypeExternal   = "external"
	TypeCloudflare = "cloudflare"
)

// ValidTypes contains all valid CADDY_TYPE values
var ValidTypes = []string{TypeInternal, TypeExternal, TypeCloudflare}

// splitCommaSeparated splits a comma-separated string and trims whitespace
func splitCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// Config holds the application configuration from environment variables
type Config struct {
	CaddyContainer     string
	NetworkSuffix      string
	HostsDir           string
	DNSRefreshInterval int    // seconds, default 60
	CodeEditorURL      string // optional, base URL for code editor links
}

// CaddyConfig holds the parsed configuration for a service
type CaddyConfig struct {
	Network     string   // Network name
	Container   string   // Container name
	Domains     []string // From CADDY_DOMAIN (comma-separated)
	Type        string   // internal, external, cloudflare
	Upstream    string   // container:port
	Allowlist   []string // From CADDY_ALLOWLIST (comma-separated hostnames/IPs)
	Logging     bool     // From CADDY_LOGGING (optional, default false)
	TLS         bool     // From CADDY_TLS (optional, default true)
	Compression bool     // From CADDY_COMPRESSION (optional, default true)
	Header      bool     // From CADDY_HEADER (optional, default true)
	Auth        bool     // From CADDY_AUTH (optional, default false)
	AuthPaths   []string // From CADDY_AUTH_PATHS (optional, if set only these paths require auth)
	SEO         bool     // From CADDY_SEO (optional, default false)
	WWWRedirect bool     // From CADDY_WWW_REDIRECT (optional, default false)
	Performance bool     // From CADDY_PERFORMANCE (optional, default true)
	Security    bool     // From CADDY_SECURITY (optional, default true)
	WordPress   bool     // From CADDY_WORDPRESS (optional, default false)
}

// ConfigKey returns the unique key for this config (container_network)
func (c *CaddyConfig) ConfigKey() string {
	return c.Container + "_" + c.Network
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// COMPOSE_PROJECT_NAME is set automatically by Docker Compose
	projectName := os.Getenv("COMPOSE_PROJECT_NAME")
	if projectName == "" {
		projectName = "caddy" // fallback
	}
	caddyContainer := projectName + "-app-1"

	networkSuffix := os.Getenv("NETWORK_SUFFIX")
	if networkSuffix == "" {
		networkSuffix = "_caddy"
	}

	hostsDir := os.Getenv("HOSTS_DIR")
	if hostsDir == "" {
		hostsDir = "/hosts"
	}

	dnsRefreshInterval := 60 // default 60 seconds
	if interval := os.Getenv("DNS_REFRESH_INTERVAL"); interval != "" {
		if parsed, err := strconv.Atoi(interval); err == nil && parsed > 0 {
			dnsRefreshInterval = parsed
		}
	}

	// Optional code editor URL for linking to config files
	codeEditorURL := os.Getenv("CODE_EDITOR_URL")

	return &Config{
		CaddyContainer:     caddyContainer,
		NetworkSuffix:      networkSuffix,
		HostsDir:           hostsDir,
		DNSRefreshInterval: dnsRefreshInterval,
		CodeEditorURL:      codeEditorURL,
	}, nil
}

// ParseCaddyEnv parses CADDY_* environment variables from a container
// Returns nil, nil if no CADDY_* variables are set (container should be ignored)
// Returns nil, error if configuration is incomplete or invalid
func ParseCaddyEnv(env map[string]string, network string, containerName string) (*CaddyConfig, error) {
	domain := env["CADDY_DOMAIN"]
	typ := env["CADDY_TYPE"]
	port := env["CADDY_PORT"]

	// Check if any CADDY_* variable is set
	hasCaddyVars := domain != "" || typ != "" || port != ""
	if !hasCaddyVars {
		return nil, nil // No CADDY_* variables, ignore silently
	}

	// All three are required if any is set
	if domain == "" {
		return nil, fmt.Errorf("CADDY_DOMAIN is required")
	}
	if typ == "" {
		return nil, fmt.Errorf("CADDY_TYPE is required")
	}
	if port == "" {
		return nil, fmt.Errorf("CADDY_PORT is required")
	}

	// Validate type
	validType := false
	for _, t := range ValidTypes {
		if typ == t {
			validType = true
			break
		}
	}
	if !validType {
		return nil, fmt.Errorf("invalid CADDY_TYPE: %s (must be %s)", typ, strings.Join(ValidTypes, "|"))
	}

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid CADDY_PORT: %s (must be numeric)", port)
	}

	// Parse and validate domains
	domains := splitCommaSeparated(domain)
	for _, d := range domains {
		if !isValidDomain(d) {
			return nil, fmt.Errorf("invalid domain: %s", d)
		}
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("no valid domains in CADDY_DOMAIN")
	}

	// Build upstream from container name and port
	// Container name often starts with "/" from Docker API
	name := strings.TrimPrefix(containerName, "/")
	upstream := fmt.Sprintf("%s:%s", name, port)

	// Parse allowlist (optional)
	allowlist := splitCommaSeparated(env["CADDY_ALLOWLIST"])

	// Parse optional flags
	logging := env["CADDY_LOGGING"] == "true"           // default: off
	tls := env["CADDY_TLS"] != "false"                  // default: on
	compression := env["CADDY_COMPRESSION"] != "false" // default: on
	header := env["CADDY_HEADER"] != "false"           // default: on
	auth := env["CADDY_AUTH"] == "true"                 // default: off
	seo := env["CADDY_SEO"] == "true"                   // default: off
	wwwRedirect := env["CADDY_WWW_REDIRECT"] == "true" // default: off
	performance := env["CADDY_PERFORMANCE"] != "false" // default: on
	security := env["CADDY_SECURITY"] != "false"       // default: on
	wordpress := env["CADDY_WORDPRESS"] == "true"       // default: off

	// Parse auth paths (optional)
	authPaths := splitCommaSeparated(env["CADDY_AUTH_PATHS"])

	return &CaddyConfig{
		Network:     network,
		Container:   name,
		Domains:     domains,
		Type:        typ,
		Upstream:    upstream,
		Allowlist:   allowlist,
		Logging:     logging,
		TLS:         tls,
		Compression: compression,
		Header:      header,
		Auth:        auth,
		AuthPaths:   authPaths,
		SEO:         seo,
		WWWRedirect: wwwRedirect,
		Performance: performance,
		Security:    security,
		WordPress:   wordpress,
	}, nil
}

// isValidDomain performs basic domain validation
func isValidDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}

	// Basic pattern: alphanumeric, hyphens, dots
	// Must have at least one dot for a valid domain
	if !strings.Contains(domain, ".") {
		return false
	}

	// Check each label
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		// Labels can't start or end with hyphen
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		// Only alphanumeric and hyphens
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}

	return true
}
