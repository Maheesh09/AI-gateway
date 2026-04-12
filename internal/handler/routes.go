package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Maheesh09/AI-gateway/internal/repository"
)

// RouteHandler serves admin endpoints for proxy route management.
type RouteHandler struct {
	repo *repository.RouteRepo
}

func NewRouteHandler(repo *repository.RouteRepo) *RouteHandler {
	return &RouteHandler{repo: repo}
}

// GET /v1/admin/routes
func (h *RouteHandler) List(w http.ResponseWriter, r *http.Request) {
	routes, err := h.repo.List(r.Context())
	if err != nil {
		log.Printf("error listing routes: %v", err)
		writeJSON(w, http.StatusInternalServerError, errBody("could not list routes"))
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

// POST /v1/admin/routes
func (h *RouteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string   `json:"name"`
		PathPattern    string   `json:"path_pattern"`
		TargetURL      string   `json:"target_url"`
		AllowedMethods []string `json:"allowed_methods"`
		StripPrefix    bool     `json:"strip_prefix"`
		TimeoutMs      int      `json:"timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request body"))
		return
	}
	if body.Name == "" || body.PathPattern == "" || body.TargetURL == "" {
		writeJSON(w, http.StatusBadRequest, errBody("name, path_pattern, and target_url are required"))
		return
	}
	if len(body.AllowedMethods) == 0 {
		body.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	}
	if body.TimeoutMs <= 0 {
		body.TimeoutMs = 5000
	}

	route, err := h.repo.Create(r.Context(),
		body.Name, body.PathPattern, body.TargetURL,
		body.AllowedMethods, body.StripPrefix, body.TimeoutMs,
	)
	if err != nil {
		log.Printf("error creating route: %v", err)
		writeJSON(w, http.StatusInternalServerError, errBody("could not create route"))
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

// PUT /v1/admin/routes/{id}
func (h *RouteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Fetch existing to use as defaults for optional fields
	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil || existing == nil {
		writeJSON(w, http.StatusNotFound, errBody("route not found"))
		return
	}

	var body struct {
		Name           string   `json:"name"`
		TargetURL      string   `json:"target_url"`
		AllowedMethods []string `json:"allowed_methods"`
		StripPrefix    *bool    `json:"strip_prefix"`
		TimeoutMs      int      `json:"timeout_ms"`
		IsActive       *bool    `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request body"))
		return
	}

	// Merge with existing
	name := coalesce(body.Name, existing.Name)
	targetURL := coalesce(body.TargetURL, existing.TargetURL)
	methods := body.AllowedMethods
	if len(methods) == 0 {
		methods = existing.AllowedMethods
	}
	stripPrefix := existing.StripPrefix
	if body.StripPrefix != nil {
		stripPrefix = *body.StripPrefix
	}
	timeoutMs := existing.TimeoutMs
	if body.TimeoutMs > 0 {
		timeoutMs = body.TimeoutMs
	}
	isActive := existing.IsActive
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	route, err := h.repo.Update(r.Context(), id, name, targetURL, methods, stripPrefix, timeoutMs, isActive)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not update route"))
		return
	}
	writeJSON(w, http.StatusOK, route)
}

// DELETE /v1/admin/routes/{id}
func (h *RouteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not delete route"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
