package repository

import "time"

// LogEntry holds the data for a single proxied request to be persisted.
type LogEntry struct {
	APIKeyID   string
	RouteID    string
	Method     string
	Path       string
	StatusCode int
	LatencyMs  int
	IPAddress  string
}

// AnomalyAlert mirrors the anomaly_alerts table row.
type AnomalyAlert struct {
	ID            string     `json:"id"`
	APIKeyID      string     `json:"api_key_id"`
	Severity      string     `json:"severity"`
	TriggerType   string     `json:"trigger_type"`
	AIExplanation string     `json:"ai_explanation"`
	AutoBlocked   bool       `json:"auto_blocked"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
