package main

import (
	"testing"
)

func TestParseCaddyEnv_ValidConfig(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "internal",
		"CADDY_PORT":   "8080",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	if len(cfg.Domains) != 1 || cfg.Domains[0] != "test.example.com" {
		t.Errorf("expected domain test.example.com, got %v", cfg.Domains)
	}
	if cfg.Type != "internal" {
		t.Errorf("expected type internal, got %s", cfg.Type)
	}
	if cfg.Upstream != "test-container:8080" {
		t.Errorf("expected upstream test-container:8080, got %s", cfg.Upstream)
	}
	if cfg.Network != "test_caddy" {
		t.Errorf("expected network test_caddy, got %s", cfg.Network)
	}
}

func TestParseCaddyEnv_OptionDefaults(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default options
	if cfg.Logging != false {
		t.Error("expected logging default false")
	}
	if cfg.DNSProvider != "cloudflare" {
		t.Error("expected DNSProvider default cloudflare")
	}
	if cfg.Compression != true {
		t.Error("expected compression default true")
	}
	if cfg.Header != true {
		t.Error("expected header default true")
	}
}

func TestParseCaddyEnv_NoCaddyVars(t *testing.T) {
	env := map[string]string{
		"OTHER_VAR": "value",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when no CADDY_* vars")
	}
}

func TestParseCaddyEnv_MissingDomain(t *testing.T) {
	// Without CADDY_DOMAIN, container should be skipped silently (nil, nil)
	env := map[string]string{
		"CADDY_TYPE": "internal",
		"CADDY_PORT": "8080",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Errorf("expected nil error when domain is missing, got: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when domain is missing")
	}
}

func TestParseCaddyEnv_MultipleDomains(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "a.example.com, b.example.com, c.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Domains) != 3 {
		t.Errorf("expected 3 domains, got %d", len(cfg.Domains))
	}
	expected := []string{"a.example.com", "b.example.com", "c.example.com"}
	for i, d := range expected {
		if cfg.Domains[i] != d {
			t.Errorf("expected domain %s at index %d, got %s", d, i, cfg.Domains[i])
		}
	}
}

func TestParseCaddyEnv_InvalidType(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "invalid",
		"CADDY_PORT":   "80",
	}

	_, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseCaddyEnv_InvalidPort(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "not-a-number",
	}

	_, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestParseCaddyEnv_AllTypes(t *testing.T) {
	types := []string{"internal", "external", "cloudflare"}

	for _, typ := range types {
		env := map[string]string{
			"CADDY_DOMAIN": "test.example.com",
			"CADDY_TYPE":   typ,
			"CADDY_PORT":   "80",
		}

		cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
		if err != nil {
			t.Errorf("unexpected error for type %s: %v", typ, err)
		}
		if cfg.Type != typ {
			t.Errorf("expected type %s, got %s", typ, cfg.Type)
		}
	}
}

func TestParseCaddyEnv_Options(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		logging     bool
		dnsProvider string
		compression bool
		header      bool
		auth        bool
		seo         bool
		wwwRedirect bool
		performance bool
		security    bool
		wordpress   bool
	}{
		{
			name: "all defaults",
			env: map[string]string{
				"CADDY_DOMAIN": "test.example.com",
				"CADDY_TYPE":   "external",
				"CADDY_PORT":   "80",
			},
			logging:     false,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        false,
			seo:         false,
			wwwRedirect: false,
			performance: true,
			security:    true,
			wordpress:   false,
		},
		{
			name: "logging enabled",
			env: map[string]string{
				"CADDY_DOMAIN":  "test.example.com",
				"CADDY_TYPE":    "external",
				"CADDY_PORT":    "80",
				"CADDY_LOGGING": "true",
			},
			logging:     true,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        false,
			seo:         false,
			wwwRedirect: false,
			performance: true,
			security:    true,
			wordpress:   false,
		},
		{
			name: "auth enabled",
			env: map[string]string{
				"CADDY_DOMAIN": "test.example.com",
				"CADDY_TYPE":   "external",
				"CADDY_PORT":   "80",
				"CADDY_AUTH":   "true",
			},
			logging:     false,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        true,
			seo:         false,
			wwwRedirect: false,
			performance: true,
			security:    true,
			wordpress:   false,
		},
		{
			name: "seo enabled",
			env: map[string]string{
				"CADDY_DOMAIN": "test.example.com",
				"CADDY_TYPE":   "external",
				"CADDY_PORT":   "80",
				"CADDY_SEO":    "true",
			},
			logging:     false,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        false,
			seo:         true,
			wwwRedirect: false,
			performance: true,
			security:    true,
			wordpress:   false,
		},
		{
			name: "www redirect enabled",
			env: map[string]string{
				"CADDY_DOMAIN":       "test.example.com",
				"CADDY_TYPE":         "external",
				"CADDY_PORT":         "80",
				"CADDY_WWW_REDIRECT": "true",
			},
			logging:     false,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        false,
			seo:         false,
			wwwRedirect: true,
			performance: true,
			security:    true,
			wordpress:   false,
		},
		{
			name: "wordpress enabled",
			env: map[string]string{
				"CADDY_DOMAIN":    "test.example.com",
				"CADDY_TYPE":      "external",
				"CADDY_PORT":      "80",
				"CADDY_WORDPRESS": "true",
			},
			logging:     false,
			dnsProvider: "cloudflare",
			compression: true,
			header:      true,
			auth:        false,
			seo:         false,
			wwwRedirect: false,
			performance: true,
			security:    true,
			wordpress:   true,
		},
		{
			name: "all disabled",
			env: map[string]string{
				"CADDY_DOMAIN":       "test.example.com",
				"CADDY_TYPE":         "external",
				"CADDY_PORT":         "80",
				"CADDY_DNS_PROVIDER": "http",
				"CADDY_COMPRESSION":  "false",
				"CADDY_HEADER":       "false",
				"CADDY_PERFORMANCE":  "false",
				"CADDY_SECURITY":     "false",
			},
			logging:     false,
			dnsProvider: "http",
			compression: false,
			header:      false,
			auth:        false,
			seo:         false,
			wwwRedirect: false,
			performance: false,
			security:    false,
			wordpress:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseCaddyEnv(tt.env, "test_caddy", "test-container")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Logging != tt.logging {
				t.Errorf("logging: expected %v, got %v", tt.logging, cfg.Logging)
			}
			if cfg.DNSProvider != tt.dnsProvider {
				t.Errorf("DNSProvider: expected %v, got %v", tt.dnsProvider, cfg.DNSProvider)
			}
			if cfg.Compression != tt.compression {
				t.Errorf("compression: expected %v, got %v", tt.compression, cfg.Compression)
			}
			if cfg.Header != tt.header {
				t.Errorf("header: expected %v, got %v", tt.header, cfg.Header)
			}
			if cfg.Auth != tt.auth {
				t.Errorf("auth: expected %v, got %v", tt.auth, cfg.Auth)
			}
			if cfg.SEO != tt.seo {
				t.Errorf("seo: expected %v, got %v", tt.seo, cfg.SEO)
			}
			if cfg.WWWRedirect != tt.wwwRedirect {
				t.Errorf("wwwRedirect: expected %v, got %v", tt.wwwRedirect, cfg.WWWRedirect)
			}
			if cfg.Performance != tt.performance {
				t.Errorf("performance: expected %v, got %v", tt.performance, cfg.Performance)
			}
			if cfg.Security != tt.security {
				t.Errorf("security: expected %v, got %v", tt.security, cfg.Security)
			}
			if cfg.WordPress != tt.wordpress {
				t.Errorf("wordpress: expected %v, got %v", tt.wordpress, cfg.WordPress)
			}
		})
	}
}

func TestParseCaddyEnv_Allowlist(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":    "test.example.com",
		"CADDY_TYPE":      "external",
		"CADDY_PORT":      "80",
		"CADDY_ALLOWLIST": "1.2.3.4, home.dyndns.org, 5.6.7.8",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Allowlist) != 3 {
		t.Errorf("expected 3 allowlist entries, got %d", len(cfg.Allowlist))
	}
	expected := []string{"1.2.3.4", "home.dyndns.org", "5.6.7.8"}
	for i, e := range expected {
		if cfg.Allowlist[i] != e {
			t.Errorf("expected allowlist entry %s at index %d, got %s", e, i, cfg.Allowlist[i])
		}
	}
}

func TestParseCaddyEnv_SEONoindexTypes(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":            "test.example.com",
		"CADDY_TYPE":              "external",
		"CADDY_PORT":              "80",
		"CADDY_SEO":               "true",
		"CADDY_SEO_NOINDEX_TYPES": "pdf, doc, docx",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.SEO {
		t.Error("expected SEO to be true")
	}
	if len(cfg.SEONoindexTypes) != 3 {
		t.Errorf("expected 3 SEONoindexTypes entries, got %d", len(cfg.SEONoindexTypes))
	}
	expected := []string{"pdf", "doc", "docx"}
	for i, e := range expected {
		if i >= len(cfg.SEONoindexTypes) {
			t.Errorf("missing SEONoindexTypes %s", e)
			continue
		}
		if cfg.SEONoindexTypes[i] != e {
			t.Errorf("expected SEONoindexTypes %s at index %d, got %s", e, i, cfg.SEONoindexTypes[i])
		}
	}
}

func TestParseCaddyEnv_ContainerNamePrefix(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
	}

	// Container names from Docker have leading slash
	cfg, err := ParseCaddyEnv(env, "test_caddy", "/my-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should strip the leading slash
	if cfg.Upstream != "my-container:80" {
		t.Errorf("expected upstream my-container:80, got %s", cfg.Upstream)
	}
}

func TestParseCaddyEnv_AuthPaths(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":     "test.example.com",
		"CADDY_TYPE":       "external",
		"CADDY_PORT":       "80",
		"CADDY_AUTH":       "true",
		"CADDY_AUTH_PATHS": "/admin/*, /dashboard/*, /api/private/*",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Auth {
		t.Error("expected Auth to be true")
	}
	if len(cfg.AuthPaths) != 3 {
		t.Errorf("expected 3 auth paths, got %d", len(cfg.AuthPaths))
	}
	expected := []string{"/admin/*", "/dashboard/*", "/api/private/*"}
	for i, e := range expected {
		if i >= len(cfg.AuthPaths) {
			t.Errorf("missing auth path %s", e)
			continue
		}
		if cfg.AuthPaths[i] != e {
			t.Errorf("expected auth path %s at index %d, got %s", e, i, cfg.AuthPaths[i])
		}
	}
}

func TestParseCaddyEnv_AuthPathsEmpty(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
		"CADDY_AUTH":   "true",
		// No CADDY_AUTH_PATHS = protect entire site
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Auth {
		t.Error("expected Auth to be true")
	}
	if len(cfg.AuthPaths) != 0 {
		t.Errorf("expected no auth paths, got %d", len(cfg.AuthPaths))
	}
}

func TestParseCaddyEnv_AuthURL(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":   "test.example.com",
		"CADDY_TYPE":     "external",
		"CADDY_PORT":     "80",
		"CADDY_AUTH":     "true",
		"CADDY_AUTH_URL": "https://login.example.com",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Auth {
		t.Error("expected Auth to be true")
	}
	if cfg.AuthURL != "https://login.example.com" {
		t.Errorf("expected AuthURL 'https://login.example.com', got '%s'", cfg.AuthURL)
	}
}

func TestParseCaddyEnv_AuthExcept(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":      "test.example.com",
		"CADDY_TYPE":        "external",
		"CADDY_PORT":        "80",
		"CADDY_AUTH":        "true",
		"CADDY_AUTH_EXCEPT": "/health, /api/public/*",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Auth {
		t.Error("expected Auth to be true")
	}
	if len(cfg.AuthExcept) != 2 {
		t.Errorf("expected 2 auth except paths, got %d", len(cfg.AuthExcept))
	}
	expected := []string{"/health", "/api/public/*"}
	for i, e := range expected {
		if i >= len(cfg.AuthExcept) {
			t.Errorf("missing auth except path %s", e)
			continue
		}
		if cfg.AuthExcept[i] != e {
			t.Errorf("expected auth except path %s at index %d, got %s", e, i, cfg.AuthExcept[i])
		}
	}
}

func TestParseCaddyEnv_AuthPathsAndExceptConflict(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":      "test.example.com",
		"CADDY_TYPE":        "external",
		"CADDY_PORT":        "80",
		"CADDY_AUTH":        "true",
		"CADDY_AUTH_PATHS":  "/admin/*",
		"CADDY_AUTH_EXCEPT": "/health",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// AUTH_PATHS should take precedence, AUTH_EXCEPT should be nil
	if len(cfg.AuthPaths) != 1 {
		t.Errorf("expected 1 auth path, got %d", len(cfg.AuthPaths))
	}
	if len(cfg.AuthExcept) != 0 {
		t.Errorf("expected 0 auth except paths (AUTH_PATHS takes precedence), got %d", len(cfg.AuthExcept))
	}
}

func TestParseCaddyEnv_TrustedProxies(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN":          "test.example.com",
		"CADDY_TYPE":            "external",
		"CADDY_PORT":            "80",
		"CADDY_TRUSTED_PROXIES": "192.168.1.1, 10.0.0.1",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.TrustedProxies) != 2 {
		t.Errorf("expected 2 trusted proxies, got %d", len(cfg.TrustedProxies))
	}
	if cfg.TrustedProxies[0] != "192.168.1.1" {
		t.Errorf("expected first proxy '192.168.1.1', got '%s'", cfg.TrustedProxies[0])
	}
	if cfg.TrustedProxies[1] != "10.0.0.1" {
		t.Errorf("expected second proxy '10.0.0.1', got '%s'", cfg.TrustedProxies[1])
	}
}

func TestParseCaddyEnv_TrustedProxiesEmpty(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "external",
		"CADDY_PORT":   "80",
	}

	cfg, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("expected no trusted proxies, got %d", len(cfg.TrustedProxies))
	}
}

// Multi-service tests

func TestParseAllCaddyEnv_SingleService(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com",
		"CADDY_TYPE":   "internal",
		"CADDY_PORT":   "8080",
	}

	configs, err := ParseAllCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	cfg := configs[0]
	if cfg.Domains[0] != "test.example.com" {
		t.Errorf("expected domain test.example.com, got %v", cfg.Domains)
	}
	if cfg.Type != "internal" {
		t.Errorf("expected type internal, got %s", cfg.Type)
	}
}

func TestParseAllCaddyEnv_MultiService(t *testing.T) {
	env := map[string]string{
		// Service: browser
		"CADDY_DOMAIN_browser": "browser.example.com",
		"CADDY_TYPE_browser":   "external",
		"CADDY_PORT_browser":   "3000",
		// Service: streamer
		"CADDY_DOMAIN_streamer": "streamer.example.com",
		"CADDY_TYPE_streamer":   "internal",
		"CADDY_PORT_streamer":   "3030",
	}

	configs, err := ParseAllCaddyEnv(env, "test_caddy", "vpn-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	// Find configs by service name (order is not guaranteed due to map iteration)
	var browserCfg, streamerCfg *CaddyConfig
	for _, cfg := range configs {
		if cfg.Domains[0] == "browser.example.com" {
			browserCfg = cfg
		} else if cfg.Domains[0] == "streamer.example.com" {
			streamerCfg = cfg
		}
	}

	if browserCfg == nil {
		t.Fatal("browser config not found")
	}
	if streamerCfg == nil {
		t.Fatal("streamer config not found")
	}

	// Check browser config
	if browserCfg.Type != "external" {
		t.Errorf("browser: expected type external, got %s", browserCfg.Type)
	}
	if browserCfg.Upstream != "vpn-container:3000" {
		t.Errorf("browser: expected upstream vpn-container:3000, got %s", browserCfg.Upstream)
	}
	if browserCfg.Container != "vpn-container-browser" {
		t.Errorf("browser: expected container vpn-container-browser, got %s", browserCfg.Container)
	}

	// Check streamer config
	if streamerCfg.Type != "internal" {
		t.Errorf("streamer: expected type internal, got %s", streamerCfg.Type)
	}
	if streamerCfg.Upstream != "vpn-container:3030" {
		t.Errorf("streamer: expected upstream vpn-container:3030, got %s", streamerCfg.Upstream)
	}
	if streamerCfg.Container != "vpn-container-streamer" {
		t.Errorf("streamer: expected container vpn-container-streamer, got %s", streamerCfg.Container)
	}
}

func TestParseAllCaddyEnv_MultiServiceWithOptions(t *testing.T) {
	env := map[string]string{
		// Service: browser with allowlist and auth
		"CADDY_DOMAIN_browser":    "browser.example.com",
		"CADDY_TYPE_browser":      "external",
		"CADDY_PORT_browser":      "3000",
		"CADDY_ALLOWLIST_browser": "1.2.3.4, home.dyndns.org",
		"CADDY_AUTH_browser":      "true",
		"CADDY_AUTH_URL_browser":  "https://login.example.com",
		// Service: streamer with different options
		"CADDY_DOMAIN_streamer":       "streamer.example.com",
		"CADDY_TYPE_streamer":         "internal",
		"CADDY_PORT_streamer":         "3030",
		"CADDY_LOGGING_streamer":      "true",
		"CADDY_DNS_PROVIDER_streamer": "hetzner",
	}

	configs, err := ParseAllCaddyEnv(env, "test_caddy", "vpn-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}

	// Find configs
	var browserCfg, streamerCfg *CaddyConfig
	for _, cfg := range configs {
		if cfg.Domains[0] == "browser.example.com" {
			browserCfg = cfg
		} else if cfg.Domains[0] == "streamer.example.com" {
			streamerCfg = cfg
		}
	}

	// Check browser options
	if len(browserCfg.Allowlist) != 2 {
		t.Errorf("browser: expected 2 allowlist entries, got %d", len(browserCfg.Allowlist))
	}
	if !browserCfg.Auth {
		t.Error("browser: expected Auth to be true")
	}
	if browserCfg.AuthURL != "https://login.example.com" {
		t.Errorf("browser: expected AuthURL https://login.example.com, got %s", browserCfg.AuthURL)
	}

	// Check streamer options
	if !streamerCfg.Logging {
		t.Error("streamer: expected Logging to be true")
	}
	if streamerCfg.DNSProvider != "hetzner" {
		t.Errorf("streamer: expected DNSProvider hetzner, got %s", streamerCfg.DNSProvider)
	}
}

func TestParseAllCaddyEnv_MultiServiceMissingRequired(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN_browser": "browser.example.com",
		"CADDY_TYPE_browser":   "external",
		// Missing CADDY_PORT_browser
	}

	_, err := ParseAllCaddyEnv(env, "test_caddy", "vpn-container")
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestParseAllCaddyEnv_MultiServiceInvalidType(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN_browser": "browser.example.com",
		"CADDY_TYPE_browser":   "invalid-type",
		"CADDY_PORT_browser":   "3000",
	}

	_, err := ParseAllCaddyEnv(env, "test_caddy", "vpn-container")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseAllCaddyEnv_NoCaddyVars(t *testing.T) {
	env := map[string]string{
		"OTHER_VAR": "value",
	}

	configs, err := ParseAllCaddyEnv(env, "test_caddy", "test-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configs != nil {
		t.Error("expected nil configs when no CADDY_* vars")
	}
}

func TestExtractServiceNames(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN_browser":  "browser.example.com",
		"CADDY_DOMAIN_streamer": "streamer.example.com",
		"CADDY_DOMAIN_admin":    "admin.example.com",
		"CADDY_TYPE_browser":    "external",
		"OTHER_VAR":             "value",
	}

	names := extractServiceNames(env)
	if len(names) != 3 {
		t.Errorf("expected 3 service names, got %d", len(names))
	}

	// Check all names are present (order not guaranteed)
	expected := map[string]bool{"browser": true, "streamer": true, "admin": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected service name: %s", name)
		}
	}
}

func TestExtractServiceNames_Empty(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN": "test.example.com", // Single-service, not multi-service
		"OTHER_VAR":    "value",
	}

	names := extractServiceNames(env)
	if len(names) != 0 {
		t.Errorf("expected 0 service names for single-service mode, got %d", len(names))
	}
}

func TestParseAllCaddyEnv_MultiServiceTrustedProxies(t *testing.T) {
	env := map[string]string{
		"CADDY_DOMAIN_browser":          "browser.example.com",
		"CADDY_TYPE_browser":            "external",
		"CADDY_PORT_browser":            "3000",
		"CADDY_TRUSTED_PROXIES_browser": "192.168.1.1, 10.0.0.1",
	}

	configs, err := ParseAllCaddyEnv(env, "test_caddy", "vpn-container")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}

	cfg := configs[0]
	if len(cfg.TrustedProxies) != 2 {
		t.Errorf("expected 2 trusted proxies, got %d", len(cfg.TrustedProxies))
	}
	if cfg.TrustedProxies[0] != "192.168.1.1" {
		t.Errorf("expected first proxy '192.168.1.1', got '%s'", cfg.TrustedProxies[0])
	}
}
