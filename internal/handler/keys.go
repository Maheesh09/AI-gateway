package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Maheesh09/AI-gateway/internal/repository"
	"github.com/Maheesh09/AI-gateway/internal/service"
)

// KeyHandler serves admin endpoints for API key management.
type KeyHandler struct {
	repo *repository.APIKeyRepo
	svc  *service.APIKeyService
}

func NewKeyHandler(repo *repository.APIKeyRepo) *KeyHandler {
	return &KeyHandler{repo: repo, svc: service.NewAPIKeyService(repo)}
}

// POST /v1/admin/keys
func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input service.CreateKeyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request body"))
		return
	}
	if input.Name == "" || input.OwnerID == "" {
		writeJSON(w, http.StatusBadRequest, errBody("name and owner_id are required"))
		return
	}

	result, err := h.svc.Create(r.Context(), input)
	if err != nil {
		log.Printf("error creating api key: %v", err)
		writeJSON(w, http.StatusInternalServerError, errBody("could not create key"))
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// GET /v1/admin/keys
func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	keys, err := h.repo.List(r.Context(), page, limit)
	if err != nil {
		log.Printf("error listing api keys: %v", err)
		writeJSON(w, http.StatusInternalServerError, errBody("could not list keys"))
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

// GET /v1/admin/keys/{id}
func (h *KeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	key, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not fetch key"))
		return
	}
	if key == nil {
		writeJSON(w, http.StatusNotFound, errBody("key not found"))
		return
	}
	writeJSON(w, http.StatusOK, key)
}

// PATCH /v1/admin/keys/{id}
func (h *KeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		RateLimitRPM *int `json:"rate_limit_rpm"`
		IsActive     *bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid request body"))
		return
	}

	if body.RateLimitRPM != nil {
		if err := h.repo.UpdateRateLimit(r.Context(), id, *body.RateLimitRPM); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("could not update rate limit"))
			return
		}
	}
	if body.IsActive != nil {
		if err := h.repo.SetActive(r.Context(), id, *body.IsActive); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("could not update status"))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /v1/admin/keys/{id}
func (h *KeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.SetActive(r.Context(), id, false); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not revoke key"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GET /v1/admin/keys/{id}/stats
func (h *KeyHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
		hours = n
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// KeyStats is on LogRepo, which we don't hold here.
	// Return a placeholder until the handler is wired with a logRepo reference.
	// (See NewKeyHandler — extend it to also accept a *repository.LogRepo.)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"api_key_id": id,
		"since":      since,
		"note":       "wire LogRepo into KeyHandler for real stats",
	})
}

// ── shared helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func errBody(msg string) map[string]string {
	return map[string]string{"error": msg}
}
