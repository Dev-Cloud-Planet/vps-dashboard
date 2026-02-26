package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// ContainersHandler holds dependencies for container endpoints.
type ContainersHandler struct {
	DB *sql.DB
}

// List handles GET /api/containers?status=running&search=n8n.
// It returns all containers matching the optional filters.
func (h *ContainersHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	containers, err := models.ListContainerInfos(h.DB, status, search)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not list containers")
		return
	}
	if containers == nil {
		containers = []models.ContainerInfo{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  containers,
		"total": len(containers),
	})
}

// Get handles GET /api/containers/{id}.
// It returns a single container's details.
func (h *ContainersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "container id is required")
		return
	}

	container, err := models.GetContainerInfo(h.DB, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch container")
		return
	}
	if container == nil {
		respondError(w, http.StatusNotFound, "container not found")
		return
	}

	respondJSON(w, http.StatusOK, container)
}

// GetMetrics handles GET /api/containers/{id}/metrics?range=24h.
// It returns container metrics history with automatic downsampling.
func (h *ContainersHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "container id is required")
		return
	}

	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	now := time.Now().UTC()
	from, maxPoints := parseTimeRange(rangeStr, now)

	metrics, err := models.GetContainerMetricsRange(h.DB, id, from, now, maxPoints)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch container metrics")
		return
	}
	if metrics == nil {
		metrics = []models.ContainerMetricPoint{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"container_id": id,
		"range":        rangeStr,
		"from":         from,
		"to":           now,
		"points":       len(metrics),
		"data":         metrics,
	})
}
