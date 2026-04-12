package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/hibiken/asynq"

	"github.com/Maheesh09/AI-gateway/internal/ai"
	"github.com/Maheesh09/AI-gateway/internal/config"
	"github.com/Maheesh09/AI-gateway/internal/db"
	"github.com/Maheesh09/AI-gateway/internal/handler"
	appMiddleware "github.com/Maheesh09/AI-gateway/internal/middleware"
	"github.com/Maheesh09/AI-gateway/internal/proxy"
	"github.com/Maheesh09/AI-gateway/internal/repository"
)

func main() {
	cfg := config.Load()

	// ── Infrastructure ─────────────────────────────────────────────────────
	pool, err := db.NewPool(cfg.DBUrl)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	redisClient, err := db.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisClient.Close()

	// ── Repositories ───────────────────────────────────────────────────────
	apiKeyRepo := repository.NewAPIKeyRepo(pool)
	routeRepo := repository.NewRouteRepo(pool)
	alertRepo := repository.NewAlertRepo(pool)

	// ── Asynq client (enqueue analysis jobs) ───────────────────────────────
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})
	defer asynqClient.Close()

	// ── Middleware ─────────────────────────────────────────────────────────
	authMW := appMiddleware.NewAuth(cfg.JWTSecret, apiKeyRepo)
	rateMW := appMiddleware.NewRateLimiter(redisClient, cfg.RateLimitDefaultRPM, cfg.RateLimitWindowSeconds)

	// ── Admin handlers ─────────────────────────────────────────────────────
	keyHandler := handler.NewKeyHandler(apiKeyRepo)
	routeHandler := handler.NewRouteHandler(routeRepo)
	alertHandler := handler.NewAlertHandler(alertRepo)

	// ── Proxy router ───────────────────────────────────────────────────────
	proxyRouter := proxy.NewRouter(routeRepo)

	// ── HTTP router ────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(appMiddleware.Logger)
	r.Use(appMiddleware.CORS)

	// Health — no auth required
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	})

	// Admin routes — protected by X-Admin-Key header
	r.Route("/v1/admin", func(r chi.Router) {
		r.Use(appMiddleware.AdminOnly(cfg.AdminAPIKey))

		r.Get("/keys", keyHandler.List)
		r.Post("/keys", keyHandler.Create)
		r.Get("/keys/{id}", keyHandler.Get)
		r.Get("/keys/{id}/stats", keyHandler.Stats)
		r.Patch("/keys/{id}", keyHandler.Update)
		r.Delete("/keys/{id}", keyHandler.Delete)

		r.Get("/routes", routeHandler.List)
		r.Post("/routes", routeHandler.Create)
		r.Put("/routes/{id}", routeHandler.Update)
		r.Delete("/routes/{id}", routeHandler.Delete)

		r.Get("/alerts", alertHandler.List)
		r.Get("/alerts/{id}", alertHandler.Get)
		r.Patch("/alerts/{id}/resolve", alertHandler.Resolve)
	})

	// Proxy — auth + rate limit applied to all /api/* paths
	r.Group(func(r chi.Router) {
		r.Use(authMW.Handle)
		r.Use(rateMW.Handle)

		r.HandleFunc("/api/*", func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			route, ok := proxyRouter.Match(req.Context(), req)
			if !ok {
				http.Error(w, `{"error":"route not found"}`, http.StatusNotFound)
				return
			}

			up, err := proxy.NewUpstreamProxy(route)
			if err != nil {
				http.Error(w, `{"error":"proxy configuration error"}`, http.StatusInternalServerError)
				return
			}

			// Wrap w to capture status code for async logging
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			up.ServeHTTP(sw, req)

			// Fire-and-forget: enqueue AI analysis job
			keyID, _ := req.Context().Value(appMiddleware.ContextKeyAPIKeyID).(string)
			_ = ai.EnqueueAnalysis(asynqClient, ai.AnalyzePayload{
				APIKeyID:   keyID,
				RouteID:    route.ID,
				RequestID:  chiMiddleware.GetReqID(req.Context()),
				Path:       req.URL.Path,
				Method:     req.Method,
				StatusCode: sw.status,
				LatencyMs:  int(time.Since(start).Milliseconds()),
				IPAddress:  extractIP(req.RemoteAddr),
			})
		})
	})

	log.Printf("Gateway starting on :%s (env=%s)", cfg.AppPort, cfg.AppEnv)
	if err := http.ListenAndServe(":"+cfg.AppPort, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// statusWriter wraps http.ResponseWriter to capture the written status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

// extractIP strips the port from a "host:port" remote address string.
func extractIP(remoteAddr string) string {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return ip
}
