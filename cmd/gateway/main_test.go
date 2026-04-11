package main

import "testing"

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{"IPv4 with port", "192.168.1.1:8080", "192.168.1.1"},
		{"IPv4 no port or bad format", "192.168.1.1", "192.168.1.1"},
		{"IPv6 with port", "[2001:db8::1]:8080", "2001:db8::1"},
		{"IPv6 no port", "2001:db8::1", "2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIP(tt.remoteAddr)
			if result != tt.expected {
				t.Errorf("extractIP(%q) = %q, want %q", tt.remoteAddr, result, tt.expected)
			}
		})
	}
}
