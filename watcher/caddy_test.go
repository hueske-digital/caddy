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
}
