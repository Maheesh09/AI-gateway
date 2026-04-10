package proxy

import (
	"context"
	"net/http"
	"strings"

	"github.com/Maheesh09/AI-gateway/internal/model"
	"github.com/Maheesh09/AI-gateway/internal/repository"
)

type Router struct {
	routeRepo *repository.RouteRepo
}

func NewRouter(repo *repository.RouteRepo) *Router {
	return &Router{routeRepo: repo}
}

// Match finds the first active route whose path pattern matches the request.
func (r *Router) Match(ctx context.Context, req *http.Request) (*model.ProxyRoute, bool) {
	routes, err := r.routeRepo.ListActive(ctx)
	if err != nil {
		return nil, false
	}

	for _, route := range routes {
		if matchesPattern(req.URL.Path, route.PathPattern) {
			if !methodAllowed(req.Method, route.AllowedMethods) {
				continue
			}
			return &route, true
		}
	}

	return nil, false
}

// matchesPattern supports simple wildcard: /api/payments/* matches /api/payments/anything
func matchesPattern(path, pattern string) bool {
	if pattern == path {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return false
}

func methodAllowed(method string, allowed []string) bool {
	for _, m := range allowed {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}
