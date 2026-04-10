package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Maheesh09/AI-gateway/internal/model"
)

// RouteRepo provides database access for proxy routes.
type RouteRepo struct {
	pool *pgxpool.Pool
}

func NewRouteRepo(pool *pgxpool.Pool) *RouteRepo {
	return &RouteRepo{pool: pool}
}

const routeColumns = `
	id::text, name, path_pattern, target_url,
	allowed_methods, strip_prefix, timeout_ms, is_active, created_at`

func scanRoute(row pgx.Row) (*model.ProxyRoute, error) {
	var rt model.ProxyRoute
	err := row.Scan(
		&rt.ID, &rt.Name, &rt.PathPattern, &rt.TargetURL,
		&rt.AllowedMethods, &rt.StripPrefix, &rt.TimeoutMs,
		&rt.IsActive, &rt.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rt, nil
}

// ListActive returns all active routes (used by the proxy router on every request).
func (r *RouteRepo) ListActive(ctx context.Context) ([]model.ProxyRoute, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+routeColumns+`
		FROM proxy_routes WHERE is_active = true
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// List returns all routes for the admin API.
func (r *RouteRepo) List(ctx context.Context) ([]model.ProxyRoute, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+routeColumns+`
		FROM proxy_routes ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoutes(rows)
}

func scanRoutes(rows pgx.Rows) ([]model.ProxyRoute, error) {
	var routes []model.ProxyRoute
	for rows.Next() {
		var rt model.ProxyRoute
		if err := rows.Scan(
			&rt.ID, &rt.Name, &rt.PathPattern, &rt.TargetURL,
			&rt.AllowedMethods, &rt.StripPrefix, &rt.TimeoutMs,
			&rt.IsActive, &rt.CreatedAt,
		); err != nil {
			return nil, err
		}
		routes = append(routes, rt)
	}
	return routes, rows.Err()
}

// GetByID fetches a single route by UUID.
func (r *RouteRepo) GetByID(ctx context.Context, id string) (*model.ProxyRoute, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+routeColumns+`
		FROM proxy_routes WHERE id = $1::uuid
	`, id)
	return scanRoute(row)
}

// Create inserts a new proxy route.
func (r *RouteRepo) Create(ctx context.Context, name, pathPattern, targetURL string, allowedMethods []string, stripPrefix bool, timeoutMs int) (*model.ProxyRoute, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO proxy_routes (name, path_pattern, target_url, allowed_methods, strip_prefix, timeout_ms)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+routeColumns,
		name, pathPattern, targetURL, allowedMethods, stripPrefix, timeoutMs,
	)
	return scanRoute(row)
}

// Update replaces all mutable fields on a route.
func (r *RouteRepo) Update(ctx context.Context, id, name, targetURL string, allowedMethods []string, stripPrefix bool, timeoutMs int, isActive bool) (*model.ProxyRoute, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE proxy_routes
		SET name = $2, target_url = $3, allowed_methods = $4,
		    strip_prefix = $5, timeout_ms = $6, is_active = $7
		WHERE id = $1::uuid
		RETURNING `+routeColumns,
		id, name, targetURL, allowedMethods, stripPrefix, timeoutMs, isActive,
	)
	return scanRoute(row)
}

// Delete permanently removes a route.
func (r *RouteRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1::uuid`, id)
	return err
}

// GetStats returns request counts and p99 latency for a route in a time window.
func (r *RouteRepo) GetStats(ctx context.Context, routeID string, since time.Time) (map[string]interface{}, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COUNT(*) FILTER (WHERE status_code >= 400)::int,
			COALESCE(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms), 0)::int
		FROM request_logs
		WHERE route_id = $1::uuid AND timestamp >= $2
	`, routeID, since)

	var total, errors, p99 int
	if err := row.Scan(&total, &errors, &p99); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests": total,
		"error_count":    errors,
		"p99_latency_ms": p99,
	}, nil
}
