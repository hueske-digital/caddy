package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfig_Internal(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:8080",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check file exists (container_network.conf)
	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	// Check content
	contentStr := string(content)
	if !strings.Contains(contentStr, "https://test.example.com") {
		t.Error("expected domain in config")
	}
	if !strings.Contains(contentStr, "import internal") {
		t.Error("expected import internal")
	}
	if !strings.Contains(contentStr, "@internal") {
		t.Error("expected @internal matcher")
	}
	if !strings.Contains(contentStr, "reverse_proxy test-container:8080") {
		t.Error("expected reverse_proxy directive")
	}
	if !strings.Contains(contentStr, "import tls-cloudflare") {
		t.Error("expected import tls-cloudflare")
	}
	if !strings.Contains(contentStr, "import compression") {
		t.Error("expected import compression")
	}
	if !strings.Contains(contentStr, "import header") {
		t.Error("expected import header")
	}
}

func TestWriteConfig_External(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "reverse_proxy test-container:80") {
		t.Error("expected reverse_proxy directive")
	}
	// External without allowlist should not have @allowed matcher
	if strings.Contains(contentStr, "@allowed") {
		t.Error("unexpected @allowed matcher without allowlist")
	}
}

func TestWriteConfig_Cloudflare(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "cloudflare",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "cloudflare", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "import cloudflare") {
		t.Error("expected import cloudflare")
	}
	if !strings.Contains(contentStr, "@cloudflare") {
		t.Error("expected @cloudflare matcher")
	}
}

func TestWriteConfig_MultipleDomains(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"a.example.com", "b.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "https://a.example.com") {
		t.Error("expected first domain")
	}
	if !strings.Contains(contentStr, "https://b.example.com") {
		t.Error("expected second domain")
	}
}

func TestWriteConfig_NoLogging(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		Logging:     false,
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if strings.Contains(string(content), "import logging") {
		t.Error("unexpected import logging when logging is false")
	}
}

func TestWriteConfig_WithLogging(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		Logging:     true,
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "import logging") {
		t.Error("expected import logging when logging is true")
	}
}

func TestWriteConfig_WithAuth(t *testing.T) {
	// Test auth for all three types
	// NOTE: internal and cloudflare have auth INSIDE handle block (for stealth)
	//       external without allowlist has auth in imports
	types := []string{"internal", "external", "cloudflare"}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			tmpDir := t.TempDir()
			mgr := NewCaddyManager(tmpDir, nil)

			cfg := &CaddyConfig{
				Network:     "test_caddy",
				Container:   "test-container",
				Domains:     []string{"test.example.com"},
				Type:        typ,
				Upstream:    "test-container:80",
				DNSProvider: "cloudflare",
				Compression: true,
				Header:      true,
				Auth:        true,
			}

			err := mgr.WriteConfig(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			path := filepath.Join(tmpDir, typ, "test-container_test_caddy.conf")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read config: %v", err)
			}

			contentStr := string(content)

			if typ == "external" {
				// External without allowlist uses import auth
				if !strings.Contains(contentStr, "import auth") {
					t.Errorf("expected import auth for type %s", typ)
				}
			} else {
				// Internal and cloudflare have forward_auth inside handle block
				if !strings.Contains(contentStr, "forward_auth") {
					t.Errorf("expected forward_auth for type %s", typ)
				}
				// Should NOT have import auth (that would run before IP check)
				if strings.Contains(contentStr, "import auth") {
					t.Errorf("unexpected import auth for type %s (should be forward_auth inside handle)", typ)
				}
			}
		})
	}
}

func TestWriteConfig_DisabledOptions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		Logging:     false,
		DNSProvider: "http",
		Compression: false,
		Header:      false,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "import logging") {
		t.Error("unexpected import logging")
	}
	if strings.Contains(contentStr, "import tls-") {
		t.Error("unexpected import tls-*")
	}
	if strings.Contains(contentStr, "import compression") {
		t.Error("unexpected import compression")
	}
	if strings.Contains(contentStr, "import header") {
		t.Error("unexpected import header")
	}
}

func TestRemoveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Create a config first
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Verify it exists
	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file should exist")
	}

	// Remove it (by network name - removes all configs for that network)
	err = mgr.RemoveConfig("test_caddy")
	if err != nil {
		t.Fatalf("failed to remove config: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("config file should be deleted")
	}
}

func TestRemoveConfig_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Should not error when removing non-existent config
	err := mgr.RemoveConfig("non_existent")
	if err != nil {
		t.Errorf("unexpected error removing non-existent config: %v", err)
	}
}

func TestWriteConfig_TypeChange(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Create initial config as internal
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	internalPath := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	if _, err := os.Stat(internalPath); os.IsNotExist(err) {
		t.Fatal("expected internal config file to exist")
	}

	// Change type to external
	cfg.Type = "external"
	err = mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old internal file should be removed
	if _, err := os.Stat(internalPath); !os.IsNotExist(err) {
		t.Error("expected old internal config file to be removed")
	}

	// New external file should exist
	externalPath := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	if _, err := os.Stat(externalPath); os.IsNotExist(err) {
		t.Fatal("expected external config file to exist")
	}
}

func TestListConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Create configs
	configs := []*CaddyConfig{
		{Network: "test1_caddy", Container: "c1", Domains: []string{"test1.example.com"}, Type: "internal", Upstream: "c1:80", DNSProvider: "cloudflare", Compression: true, Header: true},
		{Network: "test2_caddy", Container: "c2", Domains: []string{"test2.example.com"}, Type: "external", Upstream: "c2:80", DNSProvider: "cloudflare", Compression: true, Header: true},
	}

	for _, cfg := range configs {
		if err := mgr.WriteConfig(cfg); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}
	}

	// List configs
	list := mgr.ListConfigs()
	if len(list) != 2 {
		t.Errorf("expected 2 configs, got %d", len(list))
	}

	// Check managed flag and container
	for _, info := range list {
		if !info.Managed {
			t.Errorf("expected config %s to be managed", info.Network)
		}
		if info.Container == "" {
			t.Errorf("expected container name for %s", info.Network)
		}
	}
}

func TestListConfigs_ManualConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Create directory and manual config file (not via WriteConfig)
	dir := filepath.Join(tmpDir, "internal")
	os.MkdirAll(dir, 0755)
	manualContent := `https://manual.example.com {
    reverse_proxy manual-container:80
}`
	os.WriteFile(filepath.Join(dir, "manual.conf"), []byte(manualContent), 0644)

	// List configs
	list := mgr.ListConfigs()
	if len(list) != 1 {
		t.Errorf("expected 1 config, got %d", len(list))
	}

	// Check it's marked as manual (not managed)
	if list[0].Managed {
		t.Error("expected manual config to not be managed")
	}
	if list[0].Network != "manual" {
		t.Errorf("expected network 'manual', got %s", list[0].Network)
	}
}

func TestExtractDomainsFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	content := `# test config
https://test.example.com {
    reverse_proxy test:80
}`
	os.WriteFile(path, []byte(content), 0644)

	domains := extractDomainsFromConfig(path)
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}
	if domains[0] != "test.example.com" {
		t.Errorf("expected test.example.com, got %s", domains[0])
	}
}

func TestExtractDomainsFromConfig_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	content := `# test config
https://a.example.com, https://b.example.com {
    reverse_proxy test:80
}`
	os.WriteFile(path, []byte(content), 0644)

	domains := extractDomainsFromConfig(path)
	if len(domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(domains))
	}
	if domains[0] != "a.example.com" {
		t.Errorf("expected a.example.com, got %s", domains[0])
	}
	if domains[1] != "b.example.com" {
		t.Errorf("expected b.example.com, got %s", domains[1])
	}
}

func TestExtractDomainsFromConfig_MultipleBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.conf")

	content := `# Multiple server blocks in one file
https://movies.example.com {
    import internal
    handle @internal {
        reverse_proxy vpn:7878
    }
    abort
}

https://series.example.com {
    import internal
    handle @internal {
        reverse_proxy vpn:8989
    }
    abort
}

https://downloads.example.com {
    import internal
    handle @internal {
        reverse_proxy vpn:8080
    }
    abort
}`
	os.WriteFile(path, []byte(content), 0644)

	domains := extractDomainsFromConfig(path)
	if len(domains) != 3 {
		t.Errorf("expected 3 domains, got %d", len(domains))
	}

	expected := []string{"movies.example.com", "series.example.com", "downloads.example.com"}
	for i, exp := range expected {
		if i >= len(domains) {
			t.Errorf("missing domain %s", exp)
			continue
		}
		if domains[i] != exp {
			t.Errorf("expected %s, got %s", exp, domains[i])
		}
	}
}

func TestInvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:   "test_caddy",
		Container: "test-container",
		Domains:   []string{"test.example.com"},
		Type:      "invalid_type",
		Upstream:  "test-container:80",
	}

	err := mgr.WriteConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestWriteConfig_Allowlist(t *testing.T) {
	tmpDir := t.TempDir()
	// Create AllowlistManager to test allowlist functionality
	am := NewAllowlistManager(0, nil) // 0 = no auto-refresh, nil = no callback
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		Allowlist:   []string{"1.2.3.4", "5.6.7.8"},
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "@allowed") {
		t.Error("expected @allowed matcher")
	}
	if !strings.Contains(contentStr, "remote_ip") {
		t.Error("expected remote_ip directive")
	}
	if !strings.Contains(contentStr, "1.2.3.4") {
		t.Error("expected first IP in allowlist")
	}
	if !strings.Contains(contentStr, "5.6.7.8") {
		t.Error("expected second IP in allowlist")
	}
	if !strings.Contains(contentStr, "private_ranges") {
		t.Error("expected private_ranges for internal network access")
	}
}

func TestWriteConfig_AllowlistDNSFails(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	// Use unresolvable hostname - should fail-closed with private_ranges only
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		Allowlist:   []string{"this-hostname-does-not-exist-xyz123.invalid"},
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	// Must have private_ranges (fail-closed, not open)
	if !strings.Contains(contentStr, "private_ranges") {
		t.Error("expected private_ranges even when DNS fails")
	}
	// Must have import stealth (fail-closed)
	if !strings.Contains(contentStr, "import stealth") {
		t.Error("expected import stealth when DNS fails (fail-closed)")
	}
	// Should NOT have the unresolved hostname as an IP
	if strings.Contains(contentStr, "this-hostname-does-not-exist") {
		t.Error("hostname should not appear in config")
	}
}

func TestWriteConfig_AllowlistWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		Allowlist:   []string{"1.2.3.4"},
		Auth:        true,
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Auth should be inside handle @allowed, not in imports
	if strings.Contains(contentStr, "import auth") {
		t.Error("auth should not be in imports for external with allowlist")
	}

	// forward_auth should appear after handle @allowed
	handleIdx := strings.Index(contentStr, "handle @allowed")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth")
	reverseProxyIdx := strings.Index(contentStr, "reverse_proxy")

	if handleIdx == -1 {
		t.Fatal("expected handle @allowed block")
	}
	if forwardAuthIdx == -1 {
		t.Fatal("expected forward_auth directive")
	}
	if forwardAuthIdx < handleIdx {
		t.Error("forward_auth should be inside handle @allowed block")
	}
	if reverseProxyIdx < forwardAuthIdx {
		t.Error("reverse_proxy should come after forward_auth")
	}
}

func TestWriteConfig_AllowlistWithAuthURL(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		Allowlist:   []string{"1.2.3.4"},
		Auth:        true,
		AuthURL:     "https://login.example.com",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Custom auth URL should be inside handle @allowed
	if !strings.Contains(contentStr, "forward_auth https://login.example.com") {
		t.Error("expected custom auth URL in forward_auth")
	}

	// External HTTPS auth needs header_up Host
	if !strings.Contains(contentStr, "header_up Host {http.reverse_proxy.upstream.hostport}") {
		t.Error("expected header_up Host for external HTTPS auth")
	}

	// Should be inside handle block
	handleIdx := strings.Index(contentStr, "handle @allowed")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth https://login.example.com")

	if forwardAuthIdx < handleIdx {
		t.Error("forward_auth should be inside handle @allowed block")
	}
}

func TestWriteConfig_ExternalWithAuthNoAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		Auth:        true,
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Without allowlist, auth should be in imports (before reverse_proxy)
	if !strings.Contains(contentStr, "import auth") {
		t.Error("expected import auth for external without allowlist")
	}

	// Should not have handle @allowed block
	if strings.Contains(contentStr, "handle @allowed") {
		t.Error("unexpected handle @allowed block without allowlist")
	}
}

func TestWriteConfig_WithSEO(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// SEO=true means indexable (no noindex import)
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		SEO:         true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if strings.Contains(string(content), "import noindex") {
		t.Error("expected NO import noindex when SEO is true")
	}
}

func TestWriteConfig_WithoutSEO(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// SEO=false (default) means not indexable (has noindex import)
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		SEO:         false,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "import noindex") {
		t.Error("expected import noindex when SEO is false")
	}
}

func TestWriteConfig_WithWWWRedirect(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		WWWRedirect: true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	// Check for www redirect block
	if !strings.Contains(contentStr, "https://www.example.com") {
		t.Error("expected www redirect domain")
	}
	if !strings.Contains(contentStr, "redir https://example.com{uri} permanent") {
		t.Error("expected redirect directive")
	}
	if !strings.Contains(contentStr, "# www redirect") {
		t.Error("expected www redirect comment")
	}
}

func TestWriteConfig_WWWRedirect_MultipleDomains(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"example.com", "other.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		WWWRedirect: true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	// Check for both www redirect blocks
	if !strings.Contains(contentStr, "https://www.example.com") {
		t.Error("expected www.example.com redirect")
	}
	if !strings.Contains(contentStr, "https://www.other.com") {
		t.Error("expected www.other.com redirect")
	}
	if !strings.Contains(contentStr, "redir https://example.com{uri} permanent") {
		t.Error("expected redirect to example.com")
	}
	if !strings.Contains(contentStr, "redir https://other.com{uri} permanent") {
		t.Error("expected redirect to other.com")
	}
}

func TestWriteConfig_NoWWWRedirect(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		WWWRedirect: false,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "www.example.com") {
		t.Error("unexpected www redirect when WWWRedirect is false")
	}
}

func TestGenerateWWWRedirectBlocks(t *testing.T) {
	domains := []string{"example.com", "test.org"}
	result := generateWWWRedirectBlocks(domains, "cloudflare")

	if !strings.Contains(result, "https://www.example.com") {
		t.Error("expected www.example.com in result")
	}
	if !strings.Contains(result, "https://www.test.org") {
		t.Error("expected www.test.org in result")
	}
	if !strings.Contains(result, "import tls-cloudflare") {
		t.Error("expected import tls-cloudflare in redirect blocks")
	}
	if !strings.Contains(result, "redir https://example.com{uri} permanent") {
		t.Error("expected redirect to example.com")
	}
	if !strings.Contains(result, "redir https://test.org{uri} permanent") {
		t.Error("expected redirect to test.org")
	}
}

func TestWriteConfig_WithPerformance(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Performance: true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "import performance") {
		t.Error("expected import performance")
	}
}

func TestWriteConfig_WithSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Security:    true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "import security") {
		t.Error("expected import security")
	}
}

func TestWriteConfig_WithWordPress(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		WordPress:   true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	if !strings.Contains(string(content), "import wordpress") {
		t.Error("expected import wordpress")
	}
}

func TestWriteConfig_DefaultsIncludePerformanceAndSecurity(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Using defaults: performance and security should be true
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Performance: true, // default on
		Security:    true, // default on
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "import performance") {
		t.Error("expected import performance by default")
	}
	if !strings.Contains(contentStr, "import security") {
		t.Error("expected import security by default")
	}
	if strings.Contains(contentStr, "import wordpress") {
		t.Error("unexpected import wordpress (should be off by default)")
	}
}

func TestWriteConfig_AuthWithPaths(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthPaths:   []string{"/admin/*", "/dashboard/*"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	// Should NOT have import auth (that's for full site)
	if strings.Contains(contentStr, "import auth") {
		t.Error("unexpected import auth when AuthPaths is set")
	}
	// Should have path-based auth
	if !strings.Contains(contentStr, "@auth-paths path /admin/* /dashboard/*") {
		t.Error("expected @auth-paths matcher")
	}
	if !strings.Contains(contentStr, "forward_auth @auth-paths") {
		t.Error("expected forward_auth with @auth-paths matcher")
	}
}

func TestWriteConfig_AuthWithoutPaths(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	// Test with internal type (auth inside handle block)
	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthPaths:   nil, // No paths = full site auth
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)
	// Internal type has forward_auth inside handle block (for stealth)
	if !strings.Contains(contentStr, "forward_auth") {
		t.Error("expected forward_auth for internal type")
	}
	// Should NOT have import auth (that would run before IP check)
	if strings.Contains(contentStr, "import auth") {
		t.Error("unexpected import auth for internal type (should be forward_auth inside handle)")
	}
	// Should NOT have path-based auth
	if strings.Contains(contentStr, "@auth-paths") {
		t.Error("unexpected @auth-paths when AuthPaths is empty")
	}
}

func TestGenerateAuthBlock(t *testing.T) {
	t.Run("path-based with local auth", func(t *testing.T) {
		paths := []string{"/admin/*", "/api/private/*"}
		result := generateAuthBlock("", paths, nil, nil)

		if !strings.Contains(result, "@auth-paths path /admin/* /api/private/*") {
			t.Error("expected path matcher with all paths")
		}
		if !strings.Contains(result, "forward_auth @auth-paths") {
			t.Error("expected forward_auth with matcher")
		}
		if !strings.Contains(result, "{env.COMPOSE_PROJECT_NAME}-tinyauth-1:3000") {
			t.Error("expected local tinyauth server")
		}
	})

	t.Run("full site with local auth", func(t *testing.T) {
		result := generateAuthBlock("", nil, nil, nil)

		if strings.Contains(result, "@auth-paths") {
			t.Error("unexpected path matcher for full site auth")
		}
		if !strings.Contains(result, "forward_auth {env.COMPOSE_PROJECT_NAME}-tinyauth-1:3000") {
			t.Error("expected local tinyauth server")
		}
	})

	t.Run("full site with custom auth URL", func(t *testing.T) {
		result := generateAuthBlock("https://login.example.com", nil, nil, nil)

		if !strings.Contains(result, "forward_auth https://login.example.com") {
			t.Error("expected custom auth URL")
		}
		if !strings.Contains(result, "header_up Host {http.reverse_proxy.upstream.hostport}") {
			t.Error("expected header_up Host for external HTTPS auth")
		}
		if strings.Contains(result, "tinyauth") {
			t.Error("unexpected tinyauth reference with custom URL")
		}
	})

	t.Run("path-based with custom auth URL", func(t *testing.T) {
		paths := []string{"/admin", "/admin/*"}
		result := generateAuthBlock("https://login.example.com", paths, nil, nil)

		if !strings.Contains(result, "@auth-paths path /admin /admin/*") {
			t.Error("expected path matcher")
		}
		if !strings.Contains(result, "forward_auth @auth-paths https://login.example.com") {
			t.Error("expected custom auth URL with path matcher")
		}
		if !strings.Contains(result, "header_up Host") {
			t.Error("expected header_up Host for external HTTPS auth")
		}
	})

	t.Run("local auth has no header_up", func(t *testing.T) {
		result := generateAuthBlock("", nil, nil, nil)

		if strings.Contains(result, "header_up") {
			t.Error("local auth should not have header_up")
		}
	})

	t.Run("except paths with local auth", func(t *testing.T) {
		except := []string{"/health", "/api/public/*"}
		result := generateAuthBlock("", nil, except, nil)

		if !strings.Contains(result, "@auth-paths not path /health /api/public/*") {
			t.Error("expected 'not path' matcher with except paths")
		}
		if !strings.Contains(result, "forward_auth @auth-paths") {
			t.Error("expected forward_auth with matcher")
		}
		if !strings.Contains(result, "{env.COMPOSE_PROJECT_NAME}-tinyauth-1:3000") {
			t.Error("expected local tinyauth server")
		}
	})

	t.Run("except paths with custom auth URL", func(t *testing.T) {
		except := []string{"/health", "/metrics"}
		result := generateAuthBlock("https://login.example.com", nil, except, nil)

		if !strings.Contains(result, "@auth-paths not path /health /metrics") {
			t.Error("expected 'not path' matcher")
		}
		if !strings.Contains(result, "forward_auth @auth-paths https://login.example.com") {
			t.Error("expected custom auth URL with except matcher")
		}
		if !strings.Contains(result, "header_up Host") {
			t.Error("expected header_up Host for external HTTPS auth")
		}
	})

	t.Run("paths takes precedence over except", func(t *testing.T) {
		paths := []string{"/admin/*"}
		// Note: The warning and precedence is handled in ParseCaddyEnv, not here
		// When paths is set, except should be nil after ParseCaddyEnv
		result := generateAuthBlock("", paths, nil, nil)

		if !strings.Contains(result, "@auth-paths path /admin/*") {
			t.Error("expected path matcher when paths is set")
		}
		if strings.Contains(result, "not path") {
			t.Error("unexpected 'not path' when paths is set")
		}
	})
}

func TestWriteConfig_TrustedProxies_Internal(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:        "test_caddy",
		Container:      "test-container",
		Domains:        []string{"test.example.com"},
		Type:           "internal",
		Upstream:       "test-container:8080",
		DNSProvider:    "cloudflare",
		Compression:    true,
		Header:         true,
		TrustedProxies: []string{"1.2.3.4", "5.6.7.8"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have trusted_proxies directive
	if !strings.Contains(contentStr, "trusted_proxies private_ranges 1.2.3.4 5.6.7.8") {
		t.Error("expected trusted_proxies directive with IPs")
	}
}

func TestWriteConfig_TrustedProxies_Cloudflare(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:        "test_caddy",
		Container:      "test-container",
		Domains:        []string{"test.example.com"},
		Type:           "cloudflare",
		Upstream:       "test-container:8080",
		DNSProvider:    "cloudflare",
		Compression:    true,
		Header:         true,
		TrustedProxies: []string{"10.0.0.1"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "cloudflare", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have trusted_proxies directive
	if !strings.Contains(contentStr, "trusted_proxies private_ranges 10.0.0.1") {
		t.Error("expected trusted_proxies directive")
	}
	// Should also have header_up for CF
	if !strings.Contains(contentStr, "header_up X-Real-IP {header.CF-Connecting-IP}") {
		t.Error("expected header_up for cloudflare type")
	}
}

func TestWriteConfig_TrustedProxies_External(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:        "test_caddy",
		Container:      "test-container",
		Domains:        []string{"test.example.com"},
		Type:           "external",
		Upstream:       "test-container:8080",
		DNSProvider:    "cloudflare",
		Compression:    true,
		Header:         true,
		TrustedProxies: []string{"192.168.1.1"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have trusted_proxies directive in block format
	if !strings.Contains(contentStr, "trusted_proxies private_ranges 192.168.1.1") {
		t.Error("expected trusted_proxies directive")
	}
}

func TestWriteConfig_TrustedProxies_ExternalWithAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:        "test_caddy",
		Container:      "test-container",
		Domains:        []string{"test.example.com"},
		Type:           "external",
		Upstream:       "test-container:8080",
		DNSProvider:    "cloudflare",
		Compression:    true,
		Header:         true,
		Allowlist:      []string{"1.1.1.1"},
		TrustedProxies: []string{"192.168.1.1"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have both allowlist and trusted_proxies
	if !strings.Contains(contentStr, "@allowed") {
		t.Error("expected @allowed matcher")
	}
	if !strings.Contains(contentStr, "trusted_proxies private_ranges 192.168.1.1") {
		t.Error("expected trusted_proxies directive")
	}
}

func TestWriteConfig_NoTrustedProxies(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:8080",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should not have trusted_proxies directive
	if strings.Contains(contentStr, "trusted_proxies") {
		t.Error("unexpected trusted_proxies directive when none configured")
	}
}

func TestParseImportsFromContent_TrustedProxies(t *testing.T) {
	content := `# test config
https://test.example.com {
    import tls
    reverse_proxy myapp:8080 {
        trusted_proxies private_ranges 1.2.3.4 5.6.7.8
    }
}`

	info := &ConfigInfo{}
	parseImportsFromContent(content, info)

	if len(info.TrustedProxies) != 2 {
		t.Errorf("expected 2 trusted proxies, got %d", len(info.TrustedProxies))
	}
	if info.TrustedProxies[0] != "1.2.3.4" || info.TrustedProxies[1] != "5.6.7.8" {
		t.Errorf("unexpected trusted proxies: %v", info.TrustedProxies)
	}
}

func TestParseImportsFromContent_TrustedProxiesNoPrivateRanges(t *testing.T) {
	content := `# test config
https://test.example.com {
    reverse_proxy myapp:8080 {
        trusted_proxies 10.0.0.1
    }
}`

	info := &ConfigInfo{}
	parseImportsFromContent(content, info)

	if len(info.TrustedProxies) != 1 {
		t.Errorf("expected 1 trusted proxy, got %d", len(info.TrustedProxies))
	}
	if info.TrustedProxies[0] != "10.0.0.1" {
		t.Errorf("unexpected trusted proxies: %v", info.TrustedProxies)
	}
}

// AUTH_GROUPS tests - Security critical: ensure unauthorized users cannot access resources
func TestGenerateAuthBlock_WithGroups(t *testing.T) {
	t.Run("full site auth with single group", func(t *testing.T) {
		groups := []string{"admin"}
		result := generateAuthBlock("", nil, nil, groups)

		// Must have forward_auth
		if !strings.Contains(result, "forward_auth") {
			t.Error("expected forward_auth directive")
		}
		// Must have group denial matcher
		if !strings.Contains(result, "@auth-groups-denied") {
			t.Error("SECURITY: expected @auth-groups-denied matcher for group restriction")
		}
		// Must have correct regex
		if !strings.Contains(result, "(^|,)(admin)(,|$)") {
			t.Error("SECURITY: expected correct regex for group matching")
		}
		// Must have error 403 for denied groups
		if !strings.Contains(result, "error @auth-groups-denied 403") {
			t.Error("SECURITY: expected error 403 for denied groups")
		}
		// Should not have path restriction in matcher (full site)
		if strings.Contains(result, "@auth-groups-denied {\n        path") {
			t.Error("unexpected path restriction for full site auth")
		}
	})

	t.Run("full site auth with multiple groups", func(t *testing.T) {
		groups := []string{"admin", "moderator", "staff"}
		result := generateAuthBlock("", nil, nil, groups)

		// Must have regex with all groups
		if !strings.Contains(result, "(^|,)(admin|moderator|staff)(,|$)") {
			t.Error("SECURITY: expected regex with all allowed groups")
		}
	})

	t.Run("path-based auth with groups", func(t *testing.T) {
		paths := []string{"/admin/*", "/dashboard/*"}
		groups := []string{"admin"}
		result := generateAuthBlock("", paths, nil, groups)

		// Must have path-based auth matcher
		if !strings.Contains(result, "@auth-paths path /admin/* /dashboard/*") {
			t.Error("expected path matcher")
		}
		// Must have path restriction in group denial matcher
		if !strings.Contains(result, "path /admin/* /dashboard/*") {
			t.Error("SECURITY: group denial must be restricted to auth paths")
		}
		// Must have group denial
		if !strings.Contains(result, "@auth-groups-denied") {
			t.Error("SECURITY: expected group denial matcher")
		}
	})

	t.Run("except-based auth with groups", func(t *testing.T) {
		except := []string{"/health", "/api/public/*"}
		groups := []string{"admin", "users"}
		result := generateAuthBlock("", nil, except, groups)

		// Must have 'not path' for auth
		if !strings.Contains(result, "@auth-paths not path /health /api/public/*") {
			t.Error("expected 'not path' matcher for except paths")
		}
		// Must have 'not path' in group denial matcher too
		if !strings.Contains(result, "not path /health /api/public/*") {
			t.Error("SECURITY: group denial must exclude public paths")
		}
	})

	t.Run("no groups - no group restriction", func(t *testing.T) {
		result := generateAuthBlock("", nil, nil, nil)

		// Should NOT have group denial matcher
		if strings.Contains(result, "@auth-groups-denied") {
			t.Error("unexpected group denial when no groups specified")
		}
		if strings.Contains(result, "error") {
			t.Error("unexpected error directive when no groups specified")
		}
	})

	t.Run("empty groups slice - no group restriction", func(t *testing.T) {
		result := generateAuthBlock("", nil, nil, []string{})

		if strings.Contains(result, "@auth-groups-denied") {
			t.Error("unexpected group denial with empty groups slice")
		}
	})
}

func TestWriteConfig_AuthWithGroups_Internal(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthGroups:  []string{"admin", "developers"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should NOT have import auth (that's for full site without groups)
	if strings.Contains(contentStr, "import auth") {
		t.Error("unexpected import auth when AuthGroups is set")
	}
	// Must have forward_auth
	if !strings.Contains(contentStr, "forward_auth") {
		t.Error("expected forward_auth directive")
	}
	// Must have group restriction
	if !strings.Contains(contentStr, "@auth-groups-denied") {
		t.Error("SECURITY: expected @auth-groups-denied matcher")
	}
	if !strings.Contains(contentStr, "(^|,)(admin|developers)(,|$)") {
		t.Error("SECURITY: expected correct group regex")
	}
	if !strings.Contains(contentStr, "error @auth-groups-denied 403") {
		t.Error("SECURITY: expected error 403 for denied groups")
	}
	// Internal type has stealth (which imports errors)
	if !strings.Contains(contentStr, "import stealth") {
		t.Error("expected import stealth for internal type")
	}
}

func TestWriteConfig_AuthWithGroups_ExternalNoAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthGroups:  []string{"admin"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// CRITICAL: External with AUTH_GROUPS needs import errors for the 403 to work
	if !strings.Contains(contentStr, "import errors") {
		t.Error("SECURITY: expected import errors for external type with AUTH_GROUPS")
	}
	// Must have group restriction
	if !strings.Contains(contentStr, "@auth-groups-denied") {
		t.Error("SECURITY: expected @auth-groups-denied matcher")
	}
	if !strings.Contains(contentStr, "error @auth-groups-denied 403") {
		t.Error("SECURITY: expected error 403")
	}
}

func TestWriteConfig_AuthWithGroups_ExternalWithAllowlist(t *testing.T) {
	tmpDir := t.TempDir()
	am := NewAllowlistManager(0, nil)
	mgr := NewCaddyManager(tmpDir, am)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthGroups:  []string{"admin", "users"},
		Allowlist:   []string{"1.2.3.4"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// External with allowlist has stealth (which imports errors)
	if !strings.Contains(contentStr, "import stealth") {
		t.Error("expected import stealth for external with allowlist")
	}
	// Auth with groups should be inside handle @allowed block
	handleIdx := strings.Index(contentStr, "handle @allowed")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth")

	if handleIdx == -1 || forwardAuthIdx < handleIdx {
		t.Error("forward_auth should be inside handle @allowed block")
	}
	// Must have group restriction
	if !strings.Contains(contentStr, "@auth-groups-denied") {
		t.Error("SECURITY: expected @auth-groups-denied in config")
	}
}

func TestWriteConfig_AuthWithGroupsAndPaths(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthPaths:   []string{"/admin/*"},
		AuthGroups:  []string{"admin"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have path-based auth
	if !strings.Contains(contentStr, "@auth-paths path /admin/*") {
		t.Error("expected @auth-paths matcher")
	}
	// Group denial should also have path restriction
	if !strings.Contains(contentStr, "@auth-groups-denied") {
		t.Error("SECURITY: expected @auth-groups-denied")
	}
	// The group denial must include the path to avoid blocking unauthenticated paths
	if !strings.Contains(contentStr, "path /admin/*") {
		t.Error("SECURITY: group denial must be restricted to auth paths")
	}
}

func TestWriteConfig_AuthWithGroupsAndExcept(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthExcept:  []string{"/health", "/api/public/*"},
		AuthGroups:  []string{"admin", "users"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Should have except-based auth
	if !strings.Contains(contentStr, "@auth-paths not path /health /api/public/*") {
		t.Error("expected @auth-paths with not path matcher")
	}
	// Group denial should also exclude the public paths
	if !strings.Contains(contentStr, "@auth-groups-denied") {
		t.Error("SECURITY: expected @auth-groups-denied")
	}
	// Count occurrences of "not path" - should appear twice (once for auth, once for group denial)
	notPathCount := strings.Count(contentStr, "not path /health /api/public/*")
	if notPathCount < 2 {
		t.Error("SECURITY: group denial must also exclude public paths")
	}
}

// Test that the regex correctly handles edge cases
func TestAuthGroupsRegex(t *testing.T) {
	testCases := []struct {
		name     string
		groups   []string
		expected string
	}{
		{
			name:     "single group",
			groups:   []string{"admin"},
			expected: "(^|,)(admin)(,|$)",
		},
		{
			name:     "two groups",
			groups:   []string{"admin", "users"},
			expected: "(^|,)(admin|users)(,|$)",
		},
		{
			name:     "multiple groups",
			groups:   []string{"admin", "moderator", "users", "guests"},
			expected: "(^|,)(admin|moderator|users|guests)(,|$)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := generateAuthBlock("", nil, nil, tc.groups)
			if !strings.Contains(result, tc.expected) {
				t.Errorf("expected regex %s in result", tc.expected)
			}
		})
	}
}

// SECURITY CRITICAL: Test that internal type with auth has auth INSIDE handle block
// This ensures external requests get 404 (stealth) instead of 401 (leaks existence)
func TestWriteConfig_InternalWithAuth_AuthInsideHandle(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// SECURITY: Auth must be INSIDE handle @internal block, not in imports
	// Otherwise external requests get 401 instead of 404 (stealth)
	if strings.Contains(contentStr, "import auth") {
		t.Error("SECURITY: 'import auth' should NOT be in imports for internal type - auth must be inside handle block")
	}

	// Must have forward_auth directive
	if !strings.Contains(contentStr, "forward_auth") {
		t.Error("expected forward_auth directive")
	}

	// forward_auth must come AFTER "handle @internal" (i.e., inside the handle block)
	handleIdx := strings.Index(contentStr, "handle @internal")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth")
	stealthIdx := strings.Index(contentStr, "import stealth")

	if handleIdx == -1 {
		t.Fatal("expected handle @internal block")
	}
	if forwardAuthIdx == -1 {
		t.Fatal("expected forward_auth directive")
	}
	if stealthIdx == -1 {
		t.Fatal("expected import stealth for fallback 404")
	}

	// SECURITY: forward_auth must be AFTER handle @internal (inside the block)
	if forwardAuthIdx < handleIdx {
		t.Error("SECURITY: forward_auth must be INSIDE handle @internal block, not before it")
	}

	// SECURITY: stealth must be AFTER handle block closes (for fallback 404)
	if stealthIdx < forwardAuthIdx {
		t.Error("SECURITY: import stealth must come after handle block")
	}
}

// SECURITY CRITICAL: Test that cloudflare type with auth has auth INSIDE handle block
func TestWriteConfig_CloudflareWithAuth_AuthInsideHandle(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "cloudflare",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "cloudflare", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// SECURITY: Auth must be INSIDE handle @cloudflare block, not in imports
	if strings.Contains(contentStr, "import auth") {
		t.Error("SECURITY: 'import auth' should NOT be in imports for cloudflare type - auth must be inside handle block")
	}

	// Must have forward_auth directive
	if !strings.Contains(contentStr, "forward_auth") {
		t.Error("expected forward_auth directive")
	}

	// forward_auth must come AFTER "handle @cloudflare"
	handleIdx := strings.Index(contentStr, "handle @cloudflare")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth")
	stealthIdx := strings.Index(contentStr, "import stealth")

	if handleIdx == -1 {
		t.Fatal("expected handle @cloudflare block")
	}
	if forwardAuthIdx == -1 {
		t.Fatal("expected forward_auth directive")
	}
	if stealthIdx == -1 {
		t.Fatal("expected import stealth for fallback 404")
	}

	// SECURITY: forward_auth must be AFTER handle @cloudflare
	if forwardAuthIdx < handleIdx {
		t.Error("SECURITY: forward_auth must be INSIDE handle @cloudflare block, not before it")
	}

	// SECURITY: stealth must be AFTER handle block
	if stealthIdx < forwardAuthIdx {
		t.Error("SECURITY: import stealth must come after handle block")
	}
}

// SECURITY: Test internal with auth + groups - auth must still be inside handle
func TestWriteConfig_InternalWithAuthGroups_AuthInsideHandle(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		AuthGroups:  []string{"admin"},
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "internal", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	handleIdx := strings.Index(contentStr, "handle @internal")
	forwardAuthIdx := strings.Index(contentStr, "forward_auth")
	groupsDeniedIdx := strings.Index(contentStr, "@auth-groups-denied")

	if handleIdx == -1 {
		t.Fatal("expected handle @internal block")
	}

	// SECURITY: forward_auth must be inside handle block
	if forwardAuthIdx < handleIdx {
		t.Error("SECURITY: forward_auth must be INSIDE handle @internal block")
	}

	// SECURITY: group denial must also be inside handle block
	if groupsDeniedIdx != -1 && groupsDeniedIdx < handleIdx {
		t.Error("SECURITY: @auth-groups-denied must be INSIDE handle @internal block")
	}
}

// Test that external type without allowlist and without groups does NOT import errors
func TestWriteConfig_ExternalNoGroupsNoErrors(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewCaddyManager(tmpDir, nil)

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "external",
		Upstream:    "test-container:80",
		DNSProvider: "cloudflare",
		Compression: true,
		Header:      true,
		Auth:        true,
		// No AuthGroups
	}

	err := mgr.WriteConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(tmpDir, "external", "test-container_test_caddy.conf")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}

	contentStr := string(content)

	// Without groups, should not import errors (and should not have stealth)
	if strings.Contains(contentStr, "import errors") {
		t.Error("unexpected import errors when no AUTH_GROUPS")
	}
	if strings.Contains(contentStr, "import stealth") {
		t.Error("unexpected import stealth for external without allowlist")
	}
}
