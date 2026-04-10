package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Maheesh09/AI-gateway/internal/repository"
)

// AlertHandler serves admin endpoints for anomaly alerts.
type AlertHandler struct {
	repo *repository.AlertRepo
}

func NewAlertHandler(repo *repository.AlertRepo) *AlertHandler {
	return &AlertHandler{repo: repo}
}

// GET /v1/admin/alerts?severity=HIGH&resolved=false
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	severity := r.URL.Query().Get("severity")

	var resolved *bool
	switch r.URL.Query().Get("resolved") {
	case "true":
		t := true
		resolved = &t
	case "false":
		f := false
		resolved = &f
	}

	alerts, err := h.repo.List(r.Context(), severity, resolved)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not list alerts"))
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

// GET /v1/admin/alerts/{id}
func (h *AlertHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	alert, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errBody("alert not found"))
		return
	}
	writeJSON(w, http.StatusOK, alert)
}

// PATCH /v1/admin/alerts/{id}/resolve
func (h *AlertHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Optional: re-enable the API key that was auto-blocked
	reEnable := r.URL.Query().Get("re_enable_key") == "true"

	if err := h.repo.Resolve(r.Context(), id, reEnable); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("could not resolve alert"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
