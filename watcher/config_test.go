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
	if cfg.TLS != true {
		t.Error("expected TLS default true")
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
	env := map[string]string{
		"CADDY_TYPE": "internal",
		"CADDY_PORT": "8080",
	}

	_, err := ParseCaddyEnv(env, "test_caddy", "test-container")
	if err == nil {
		t.Error("expected error when domain is missing")
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
		tls         bool
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
			tls:         true,
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
			tls:         true,
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
			tls:         true,
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
			tls:         true,
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
			tls:         true,
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
			tls:         true,
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
				"CADDY_DOMAIN":      "test.example.com",
				"CADDY_TYPE":        "external",
				"CADDY_PORT":        "80",
				"CADDY_TLS":         "false",
				"CADDY_COMPRESSION": "false",
				"CADDY_HEADER":      "false",
				"CADDY_PERFORMANCE": "false",
				"CADDY_SECURITY":    "false",
			},
			logging:     false,
			tls:         false,
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
			if cfg.TLS != tt.tls {
				t.Errorf("TLS: expected %v, got %v", tt.tls, cfg.TLS)
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
