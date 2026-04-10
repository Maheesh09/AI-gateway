package repository

import (
	"context"
	"time"
)

// LogRepo provides access to request logs and statistics.
// This is a small stub used to satisfy detector.go references.
// Replace with a real DB-backed implementation as needed.
type LogRepo struct{}

// GetStats returns aggregated request statistics for an API key since `since`.
// : implement real query against logs/store.
func (r *LogRepo) GetStats(ctx context.Context, apiKeyID string, since time.Time) (*RequestStats, error) {
	// Return empty/default stats for now.
	stats := &RequestStats{
		TotalRequests:  0,
		ErrorCount:     0,
		ErrorRate:      0.0,
		UniqueIPs:      0,
		RequestsPerMin: 0.0,
		Window:         time.Since(since),
	}
	return stats, nil
}

// Insert persists a proxied request record. The payload type is kept generic
// to avoid import cycles with the `ai` package; replace with a concrete
// parameter or DTO when wiring to a real DB layer.
func (r *LogRepo) Insert(ctx context.Context, payload interface{}) error {
	// TODO: persist payload to logs table
	return nil
}
