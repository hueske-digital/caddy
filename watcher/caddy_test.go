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
		TLS:         true,
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
	if !strings.Contains(contentStr, "import tls") {
		t.Error("expected import tls")
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
				TLS:         true,
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
			if !strings.Contains(contentStr, "import auth") {
				t.Errorf("expected import auth for type %s", typ)
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
		TLS:         false,
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
	if strings.Contains(contentStr, "import tls") {
		t.Error("unexpected import tls")
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
		TLS:         true,
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
		TLS:         true,
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
		{Network: "test1_caddy", Container: "c1", Domains: []string{"test1.example.com"}, Type: "internal", Upstream: "c1:80", TLS: true, Compression: true, Header: true},
		{Network: "test2_caddy", Container: "c2", Domains: []string{"test2.example.com"}, Type: "external", Upstream: "c2:80", TLS: true, Compression: true, Header: true},
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
		TLS:         true,
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
		TLS:         true,
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
	// Must have abort (fail-closed)
	if !strings.Contains(contentStr, "abort") {
		t.Error("expected abort when DNS fails (fail-closed)")
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
	result := generateWWWRedirectBlocks(domains)

	if !strings.Contains(result, "https://www.example.com") {
		t.Error("expected www.example.com in result")
	}
	if !strings.Contains(result, "https://www.test.org") {
		t.Error("expected www.test.org in result")
	}
	if !strings.Contains(result, "import tls") {
		t.Error("expected import tls in redirect blocks")
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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
		TLS:         true,
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

	cfg := &CaddyConfig{
		Network:     "test_caddy",
		Container:   "test-container",
		Domains:     []string{"test.example.com"},
		Type:        "internal",
		Upstream:    "test-container:80",
		TLS:         true,
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
	// Should have import auth for full site
	if !strings.Contains(contentStr, "import auth") {
		t.Error("expected import auth when AuthPaths is empty")
	}
	// Should NOT have path-based auth
	if strings.Contains(contentStr, "@auth-paths") {
		t.Error("unexpected @auth-paths when AuthPaths is empty")
	}
}

func TestGenerateAuthBlock(t *testing.T) {
	t.Run("path-based with local auth", func(t *testing.T) {
		paths := []string{"/admin/*", "/api/private/*"}
		result := generateAuthBlock("", paths)

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
		result := generateAuthBlock("", nil)

		if strings.Contains(result, "@auth-paths") {
			t.Error("unexpected path matcher for full site auth")
		}
		if !strings.Contains(result, "forward_auth {env.COMPOSE_PROJECT_NAME}-tinyauth-1:3000") {
			t.Error("expected local tinyauth server")
		}
	})

	t.Run("full site with custom auth URL", func(t *testing.T) {
		result := generateAuthBlock("https://login.example.com", nil)

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
		result := generateAuthBlock("https://login.example.com", paths)

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
		result := generateAuthBlock("", nil)

		if strings.Contains(result, "header_up") {
			t.Error("local auth should not have header_up")
		}
	})
}
