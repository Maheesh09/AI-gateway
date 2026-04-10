package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Maheesh09/AI-gateway/internal/model"
)

// NewUpstreamProxy creates a reverse proxy for the given route.
func NewUpstreamProxy(route *model.ProxyRoute) (*httputil.ReverseProxy, error) {
	targetURL, err := url.Parse(route.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("parse target url: %w", err)
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Rewrite scheme + host
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host

			// Strip gateway prefix if configured
			if route.StripPrefix {
				prefix := extractPrefix(route.PathPattern)
				req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
			}

			// Inject tracing + identity headers
			req.Header.Set("X-Forwarded-Host", req.Host)
			req.Header.Set("X-Gateway-Request-ID", req.Header.Get("X-Request-ID"))
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)

			// Remove internal headers before proxying
			req.Header.Del("X-API-Key")
			req.Header.Del("Authorization")
		},

		Transport: &http.Transport{
			ResponseHeaderTimeout: time.Duration(route.TimeoutMs) * time.Millisecond,
			MaxIdleConns:          100,
			MaxConnsPerHost:       10,
			IdleConnTimeout:       90 * time.Second,
		},

		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		},
	}

	return proxy, nil
}

func extractPrefix(pattern string) string {
	// "/api/payments/*" → "/api/payments"
	return strings.TrimSuffix(pattern, "/*")
}
