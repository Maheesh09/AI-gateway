package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LogRepo provides access to request logs and aggregated statistics.
type LogRepo struct {
	pool *pgxpool.Pool
}

func NewLogRepo(pool *pgxpool.Pool) *LogRepo {
	return &LogRepo{pool: pool}
}

// Insert persists a request log row.
func (r *LogRepo) Insert(ctx context.Context, entry LogEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO request_logs
			(api_key_id, route_id, method, path, status_code, latency_ms, ip_address)
		VALUES
			($1::uuid, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7::inet)
	`,
		entry.APIKeyID,
		entry.RouteID,
		entry.Method,
		entry.Path,
		entry.StatusCode,
		entry.LatencyMs,
		entry.IPAddress,
	)
	return err
}

// GetStats returns aggregated traffic metrics for an API key since `since`.
func (r *LogRepo) GetStats(ctx context.Context, apiKeyID string, since time.Time) (*RequestStats, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int                                         AS total,
			COUNT(*) FILTER (WHERE status_code >= 400)::int       AS errors,
			COUNT(DISTINCT ip_address::text)::int                  AS unique_ips
		FROM request_logs
		WHERE api_key_id = $1::uuid
		  AND timestamp  >= $2
	`, apiKeyID, since)

	var total, errors, uniqueIPs int
	if err := row.Scan(&total, &errors, &uniqueIPs); err != nil {
		return nil, err
	}

	window := time.Since(since)
	minutes := window.Minutes()

	var rpm float64
	if minutes > 0 {
		rpm = float64(total) / minutes
	}

	var errRate float64
	if total > 0 {
		errRate = float64(errors) / float64(total)
	}

	return &RequestStats{
		TotalRequests:  total,
		ErrorCount:     errors,
		ErrorRate:      errRate,
		UniqueIPs:      uniqueIPs,
		RequestsPerMin: rpm,
		Window:         window,
	}, nil
}

// GetKeyStats returns request-level stats plus p99 latency for the admin API.
func (r *LogRepo) GetKeyStats(ctx context.Context, apiKeyID string, since time.Time) (map[string]interface{}, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status_code >= 400)::int,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms), 0)::int
		FROM request_logs
		WHERE api_key_id = $1::uuid AND timestamp >= $2
	`, apiKeyID, since)

	var total, errorCount, p99 int
	if err := row.Scan(&total, &errorCount, &p99); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests": total,
		"error_count":    errorCount,
		"p99_latency_ms": p99,
		"since":          since,
	}, nil
}
