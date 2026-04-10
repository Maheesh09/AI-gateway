package model

// ProxyRoute represents a configured route in the gateway that should be
// proxied to an upstream service. Fields are minimal and only include what
// the proxy and router currently use.
type ProxyRoute struct {
    ID             string   `json:"id"`
    PathPattern    string   `json:"path_pattern"`
    TargetURL      string   `json:"target_url"`
    StripPrefix    bool     `json:"strip_prefix"`
    TimeoutMs      int      `json:"timeout_ms"`
    AllowedMethods []string `json:"allowed_methods"`
    IsActive       bool     `json:"is_active"`
}
