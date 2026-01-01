package main

import (
	"testing"
)

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
