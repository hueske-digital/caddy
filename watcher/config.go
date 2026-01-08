package main

import (
	"fmt"
	"log"
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
	CaddyContainer      string
	NetworkSuffix       string
	HostsDir            string
	DNSRefreshInterval  int      // seconds, default 60
	CodeEditorURL       string   // optional, base URL for code editor links
	WildcardDomains     []string // optional, domains to generate wildcard certs for
	WildcardDNSProvider string   // WILDCARD_DNS_PROVIDER (cloudflare|hetzner, default: cloudflare)
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
	DNSProvider string   // From CADDY_DNS_PROVIDER (cloudflare|hetzner|http, default: cloudflare)
	Compression bool     // From CADDY_COMPRESSION (optional, default true)
	Header      bool     // From CADDY_HEADER (optional, default true)
	Auth        bool     // From CADDY_AUTH (optional, default false)
	AuthURL     string   // From CADDY_AUTH_URL (optional, custom auth server URL)
	AuthPaths   []string // From CADDY_AUTH_PATHS (optional, if set only these paths require auth)
	SEO            bool     // From CADDY_SEO (optional, default false = noindex)
	WWWRedirect    bool     // From CADDY_WWW_REDIRECT (optional, default false)
	Performance    bool     // From CADDY_PERFORMANCE (optional, default true)
	Security       bool     // From CADDY_SECURITY (optional, default true)
	WordPress      bool     // From CADDY_WORDPRESS (optional, default false)
	TrustedProxies []string // From CADDY_TRUSTED_PROXIES (optional, IPs/hostnames for reverse_proxy)
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

	// Optional wildcard domains for automatic wildcard cert generation
	wildcardDomains := splitCommaSeparated(os.Getenv("WILDCARD_DOMAINS"))

	// DNS provider for wildcard certs (cloudflare or hetzner, default: cloudflare)
	wildcardDNSProvider := os.Getenv("WILDCARD_DNS_PROVIDER")
	if wildcardDNSProvider == "" {
		wildcardDNSProvider = "cloudflare"
	}

	return &Config{
		CaddyContainer:      caddyContainer,
		NetworkSuffix:       networkSuffix,
		HostsDir:            hostsDir,
		DNSRefreshInterval:  dnsRefreshInterval,
		CodeEditorURL:       codeEditorURL,
		WildcardDomains:     wildcardDomains,
		WildcardDNSProvider: wildcardDNSProvider,
	}, nil
}

// ParseCaddyEnv parses CADDY_* environment variables from a container
// Returns nil, nil if no CADDY_* variables are set (container should be ignored)
// Returns nil, error if configuration is incomplete or invalid
func ParseCaddyEnv(env map[string]string, network string, containerName string) (*CaddyConfig, error) {
	domain := env["CADDY_DOMAIN"]
	typ := env["CADDY_TYPE"]
	port := env["CADDY_PORT"]

	// CADDY_DOMAIN is the trigger - without it, skip
	if domain == "" {
		log.Printf("Skipping %s (no CADDY_DOMAIN)", strings.TrimPrefix(containerName, "/"))
		return nil, nil
	}

	// With CADDY_DOMAIN set, TYPE and PORT are required
	if typ == "" || port == "" {
		var missing []string
		if typ == "" {
			missing = append(missing, "CADDY_TYPE")
		}
		if port == "" {
			missing = append(missing, "CADDY_PORT")
		}
		return nil, fmt.Errorf("missing %s (CADDY_DOMAIN=%s)", strings.Join(missing, ", "), domain)
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
	dnsProvider := env["CADDY_DNS_PROVIDER"]            // cloudflare|hetzner|http, default: cloudflare
	if dnsProvider == "" {
		dnsProvider = "cloudflare"
	}
	compression := env["CADDY_COMPRESSION"] != "false" // default: on
	header := env["CADDY_HEADER"] != "false"           // default: on
	auth := env["CADDY_AUTH"] == "true"                 // default: off
	authURL := env["CADDY_AUTH_URL"]                    // optional: custom auth server URL
	seo := env["CADDY_SEO"] == "true"                   // default: off (= noindex)
	wwwRedirect := env["CADDY_WWW_REDIRECT"] == "true" // default: off
	performance := env["CADDY_PERFORMANCE"] != "false" // default: on
	security := env["CADDY_SECURITY"] != "false"       // default: on
	wordpress := env["CADDY_WORDPRESS"] == "true"       // default: off

	// Parse auth paths (optional)
	authPaths := splitCommaSeparated(env["CADDY_AUTH_PATHS"])

	// Parse trusted proxies (optional, for reverse_proxy block)
	trustedProxies := splitCommaSeparated(env["CADDY_TRUSTED_PROXIES"])

	return &CaddyConfig{
		Network:        network,
		Container:      name,
		Domains:        domains,
		Type:           typ,
		Upstream:       upstream,
		Allowlist:      allowlist,
		Logging:        logging,
		DNSProvider:    dnsProvider,
		Compression:    compression,
		Header:         header,
		Auth:           auth,
		AuthURL:        authURL,
		AuthPaths:      authPaths,
		SEO:            seo,
		WWWRedirect:    wwwRedirect,
		Performance:    performance,
		Security:       security,
		WordPress:      wordpress,
		TrustedProxies: trustedProxies,
	}, nil
}

// ParseAllCaddyEnv parses CADDY_* environment variables from a container
// Supports both single-service (CADDY_DOMAIN) and multi-service (CADDY_DOMAIN_servicename) modes
// Returns nil, nil if no CADDY_* variables are set (container should be ignored)
// Returns nil, error if configuration is incomplete or invalid
func ParseAllCaddyEnv(env map[string]string, network string, containerName string) ([]*CaddyConfig, error) {
	// Check for multi-service mode: look for CADDY_DOMAIN_* pattern
	serviceNames := extractServiceNames(env)

	if len(serviceNames) > 0 {
		// Multi-service mode
		return parseMultiServiceEnv(env, network, containerName, serviceNames)
	}

	// Single-service mode (backwards compatible)
	config, err := ParseCaddyEnv(env, network, containerName)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}
	return []*CaddyConfig{config}, nil
}

// extractServiceNames finds all service names from CADDY_DOMAIN_* variables
func extractServiceNames(env map[string]string) []string {
	seen := make(map[string]bool)
	var names []string

	for key := range env {
		if strings.HasPrefix(key, "CADDY_DOMAIN_") {
			serviceName := strings.TrimPrefix(key, "CADDY_DOMAIN_")
			if serviceName != "" && !seen[serviceName] {
				seen[serviceName] = true
				names = append(names, serviceName)
			}
		}
	}

	return names
}

// parseMultiServiceEnv parses multi-service environment variables
func parseMultiServiceEnv(env map[string]string, network string, containerName string, serviceNames []string) ([]*CaddyConfig, error) {
	var configs []*CaddyConfig

	for _, serviceName := range serviceNames {
		config, err := parseSingleServiceEnv(env, network, containerName, serviceName)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", serviceName, err)
		}
		if config != nil {
			configs = append(configs, config)
		}
	}

	if len(configs) == 0 {
		return nil, nil
	}

	return configs, nil
}

// parseSingleServiceEnv parses environment variables for a single service in multi-service mode
func parseSingleServiceEnv(env map[string]string, network string, containerName string, serviceName string) (*CaddyConfig, error) {
	// Helper to get service-specific env var
	getEnv := func(key string) string {
		return env[key+"_"+serviceName]
	}

	domain := getEnv("CADDY_DOMAIN")
	typ := getEnv("CADDY_TYPE")
	port := getEnv("CADDY_PORT")

	// CADDY_DOMAIN is required
	if domain == "" {
		return nil, fmt.Errorf("missing CADDY_DOMAIN_%s", serviceName)
	}

	// TYPE and PORT are required
	if typ == "" || port == "" {
		var missing []string
		if typ == "" {
			missing = append(missing, "CADDY_TYPE_"+serviceName)
		}
		if port == "" {
			missing = append(missing, "CADDY_PORT_"+serviceName)
		}
		return nil, fmt.Errorf("missing %s", strings.Join(missing, ", "))
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
		return nil, fmt.Errorf("invalid CADDY_TYPE_%s: %s (must be %s)", serviceName, typ, strings.Join(ValidTypes, "|"))
	}

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid CADDY_PORT_%s: %s (must be numeric)", serviceName, port)
	}

	// Parse and validate domains
	domains := splitCommaSeparated(domain)
	for _, d := range domains {
		if !isValidDomain(d) {
			return nil, fmt.Errorf("invalid domain: %s", d)
		}
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("no valid domains in CADDY_DOMAIN_%s", serviceName)
	}

	// Build upstream from container name and port
	name := strings.TrimPrefix(containerName, "/")
	upstream := fmt.Sprintf("%s:%s", name, port)

	// Parse optional flags with service suffix
	allowlist := splitCommaSeparated(getEnv("CADDY_ALLOWLIST"))
	logging := getEnv("CADDY_LOGGING") == "true"
	dnsProvider := getEnv("CADDY_DNS_PROVIDER")
	if dnsProvider == "" {
		dnsProvider = "cloudflare"
	}
	compression := getEnv("CADDY_COMPRESSION") != "false"
	header := getEnv("CADDY_HEADER") != "false"
	auth := getEnv("CADDY_AUTH") == "true"
	authURL := getEnv("CADDY_AUTH_URL")
	authPaths := splitCommaSeparated(getEnv("CADDY_AUTH_PATHS"))
	seo := getEnv("CADDY_SEO") == "true"
	wwwRedirect := getEnv("CADDY_WWW_REDIRECT") == "true"
	performance := getEnv("CADDY_PERFORMANCE") != "false"
	security := getEnv("CADDY_SECURITY") != "false"
	wordpress := getEnv("CADDY_WORDPRESS") == "true"
	trustedProxies := splitCommaSeparated(getEnv("CADDY_TRUSTED_PROXIES"))

	return &CaddyConfig{
		Network:        network,
		Container:      name + "-" + serviceName, // Unique container name per service
		Domains:        domains,
		Type:           typ,
		Upstream:       upstream,
		Allowlist:      allowlist,
		Logging:        logging,
		DNSProvider:    dnsProvider,
		Compression:    compression,
		Header:         header,
		Auth:           auth,
		AuthURL:        authURL,
		AuthPaths:      authPaths,
		SEO:            seo,
		WWWRedirect:    wwwRedirect,
		Performance:    performance,
		Security:       security,
		WordPress:      wordpress,
		TrustedProxies: trustedProxies,
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
