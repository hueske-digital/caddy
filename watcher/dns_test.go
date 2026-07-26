package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestQueryDoHType_EscapesHostname: der Hostname stammt aus CADDY_ALLOWLIST.
// Ohne Kodierung koennte er zusaetzliche Query-Parameter einschleusen.
func TestQueryDoHType_EscapesHostname(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`{"Answer":[]}`))
	}))
	defer srv.Close()

	if _, err := queryDoHType(srv.URL, "evil.example.com&type=TXT&name=other.com", "A"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("type"); got != "A" {
		t.Errorf("record type was overridden by the hostname: %q", got)
	}
	if names := gotQuery["name"]; len(names) != 1 {
		t.Errorf("expected exactly one name parameter, got %v", names)
	}
}

// TestQueryDoHType_RejectsNonIPAnswers: die Antwort landet in einem
// remote_ip-Matcher. Was keine IP ist, darf dort nicht ankommen.
func TestQueryDoHType_RejectsNonIPAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"Answer": []map[string]any{
				{"type": 1, "data": "1.2.3.4"},
				{"type": 1, "data": "not-an-ip respond \"pwned\" 200"},
				{"type": 28, "data": "2001:db8::1"},
				{"type": 5, "data": "cname.example.com"},
			},
		})
	}))
	defer srv.Close()

	ips, err := queryDoHType(srv.URL, "a.example.com", "A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]bool{"1.2.3.4": true, "2001:db8::1": true}
	if len(ips) != len(want) {
		t.Fatalf("expected only valid IPs, got %v", ips)
	}
	for _, ip := range ips {
		if !want[ip] {
			t.Errorf("non-IP answer passed through: %q", ip)
		}
	}
}

// TestQueryDoHType_LimitsResponseSize: eine fehlerhafte oder boesartige
// Gegenstelle darf den Watcher nicht beliebig viel Speicher lesen lassen.
func TestQueryDoHType_LimitsResponseSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Answer":[{"type":1,"data":"`))
		w.Write([]byte(strings.Repeat("A", maxDoHResponse*2)))
		w.Write([]byte(`"}]}`))
	}))
	defer srv.Close()

	// Abgeschnittenes JSON -> Parse-Fehler statt unbegrenztem Lesen.
	if _, err := queryDoHType(srv.URL, "a.example.com", "A"); err == nil {
		t.Error("expected an error for an oversized response")
	}
}

// TestRefreshAll_DoesNotHoldLockDuringResolution: waehrend der Aufloesung
// muessen Leser durchkommen, sonst blockiert eine haengende DNS-Abfrage jede
// Statusabfrage und jedes Schreiben einer Konfiguration.
func TestRefreshAll_DoesNotHoldLockDuringResolution(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write([]byte(`{"Answer":[]}`))
	}))
	defer srv.Close()

	original := dohEndpoints
	dohEndpoints = []string{srv.URL}
	defer func() { dohEndpoints = original }()

	m := NewAllowlistManager(60, nil)
	m.configs["web_app_caddy"] = &CaddyConfig{
		Network: "app_caddy", Container: "web",
		Allowlist: []string{"slow.example.com"},
	}

	done := make(chan struct{})
	go func() { m.refreshAll(); close(done) }()

	// Der Leser darf nicht auf die haengende Aufloesung warten muessen.
	read := make(chan struct{})
	go func() { m.GetResolvedIPs("web_app_caddy"); close(read) }()

	select {
	case <-read:
	case <-time.After(2 * time.Second):
		t.Fatal("reader blocked while DNS resolution was in flight")
	}

	close(release)
	<-done
}

func TestResolveAllowlistWithFallback_KeepsPreviousOnFailure(t *testing.T) {
	am := NewAllowlistManager(0, nil)

	previousIPs := []string{"1.2.3.4", "5.6.7.8"}

	// Empty entries = no resolution possible
	result := am.resolveAllowlistWithFallback([]string{}, previousIPs)

	if len(result) != len(previousIPs) {
		t.Errorf("expected %d IPs, got %d", len(previousIPs), len(result))
	}
	for i, ip := range previousIPs {
		if result[i] != ip {
			t.Errorf("expected %s, got %s", ip, result[i])
		}
	}
}

func TestResolveAllowlistWithFallback_UsesNewOnSuccess(t *testing.T) {
	am := NewAllowlistManager(0, nil)

	previousIPs := []string{"1.2.3.4"}
	newEntries := []string{"9.9.9.9", "8.8.8.8"} // Direct IPs, will resolve

	result := am.resolveAllowlistWithFallback(newEntries, previousIPs)

	// Should have the new IPs, sorted
	if len(result) != 2 {
		t.Errorf("expected 2 IPs, got %d", len(result))
	}
	// Sorted: 8.8.8.8 comes before 9.9.9.9
	if result[0] != "8.8.8.8" || result[1] != "9.9.9.9" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestResolveEntry_DirectIP(t *testing.T) {
	am := NewAllowlistManager(0, nil)

	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{"IPv4", "1.2.3.4", "1.2.3.4"},
		{"IPv6", "2001:db8::1", "2001:db8::1"},
		{"CIDR", "10.0.0.0/8", "10.0.0.0/8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := am.resolveEntry(tt.entry)
			if len(result) != 1 || result[0] != tt.want {
				t.Errorf("expected [%s], got %v", tt.want, result)
			}
		})
	}
}

func TestFormatAllowlistMatcher(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"1.2.3.4"}, "1.2.3.4"},
		{"multiple", []string{"1.2.3.4", "5.6.7.8"}, "1.2.3.4 5.6.7.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAllowlistMatcher(tt.ips)
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"nil vs empty", nil, []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := equalStringSlices(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("expected %v, got %v", tt.want, result)
			}
		})
	}
}
