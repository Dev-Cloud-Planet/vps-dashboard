package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// MetricsHandler holds dependencies for metrics endpoints.
type MetricsHandler struct {
	DB *sql.DB
}

// GetCurrent handles GET /api/metrics/current.
// It returns the latest system metric snapshot.
func (h *MetricsHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	metric, err := models.GetLatestMetric(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch metrics")
		return
	}
	if metric == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"data": nil, "message": "no metrics available"})
		return
	}
	respondJSON(w, http.StatusOK, metric)
}

// GetHistory handles GET /api/metrics/history?range=24h.
// It returns historical metrics with automatic downsampling based on the
// requested time range.
func (h *MetricsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	now := time.Now().UTC()
	from, maxPoints := parseTimeRange(rangeStr, now)

	metrics, err := models.GetMetricsRange(h.DB, from, now, maxPoints)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch metrics history")
		return
	}
	if metrics == nil {
		metrics = []models.Metric{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"range":  rangeStr,
		"from":   from,
		"to":     now,
		"points": len(metrics),
		"data":   metrics,
	})
}

// parseTimeRange converts a range string into a start time and target number
// of data points.
func parseTimeRange(rangeStr string, now time.Time) (from time.Time, maxPoints int) {
	switch rangeStr {
	case "1h":
		return now.Add(-1 * time.Hour), 360
	case "6h":
		return now.Add(-6 * time.Hour), 360
	case "24h":
		return now.Add(-24 * time.Hour), 288
	case "7d":
		return now.Add(-7 * 24 * time.Hour), 336
	case "30d":
		return now.Add(-30 * 24 * time.Hour), 360
	default:
		return now.Add(-24 * time.Hour), 288
	}
}
