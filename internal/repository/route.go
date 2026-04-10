package repository

import (
	"context"

	"github.com/Maheesh09/AI-gateway/internal/model"
)

// RouteRepo provides access to configured proxy routes.
type RouteRepo struct {
}

// ListActive returns active routes. This is a minimal stub used by the
// router; replace with a DB-backed implementation as needed.
func (r *RouteRepo) ListActive(ctx context.Context) ([]model.ProxyRoute, error) {
    return []model.ProxyRoute{}, nil
}
