package model

import "time"

// ProxyRoute represents a configured upstream proxy route.
type ProxyRoute struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	PathPattern    string    `json:"path_pattern"`
	TargetURL      string    `json:"target_url"`
	AllowedMethods []string  `json:"allowed_methods"`
	StripPrefix    bool      `json:"strip_prefix"`
	TimeoutMs      int       `json:"timeout_ms"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}
