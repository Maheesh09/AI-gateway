package proxy

import "testing"

func TestMatchesPattern2(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{"Exact match", "/api/v1/users", "/api/v1/users", true},
		{"Exact mismatch", "/api/v1/users", "/api/v1/posts", false},
		{"Wildcard match", "/api/v1/users/123", "/api/v1/users/*", true},
		{"Wildcard mismatch path", "/api/v1/users", "/api/v1/users/*", false}, 
		{"Wildcard match nested", "/api/v1/payments/webhook", "/api/v1/payments/*", true},
		{"Wildcard mismatch prefix", "/api/v2/payments", "/api/v1/payments/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPattern(tt.path, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMethodAllowed(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		allowed  []string
		expected bool
	}{
		{"Method allowed", "GET", []string{"GET", "POST"}, true},
		{"Method allowed case insensitive", "get", []string{"GET", "POST"}, true},
		{"Method allowed lowercase allowed", "GET", []string{"get", "post"}, true},
		{"Method not allowed", "PUT", []string{"GET", "POST"}, false},
		{"Empty allowed list", "GET", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := methodAllowed(tt.method, tt.allowed)
			if result != tt.expected {
				t.Errorf("methodAllowed(%q, %v) = %v, want %v", tt.method, tt.allowed, result, tt.expected)
			}
		})
	}
}
