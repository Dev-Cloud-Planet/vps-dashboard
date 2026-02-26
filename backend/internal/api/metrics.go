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
	metric, err := models.GetLatestSystemMetric(h.DB)
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
// requested time range:
//
//	1h  -> raw data           (~360 points at 10s interval)
//	6h  -> 1-min averages     (~360 points)
//	24h -> 5-min averages     (~288 points)
//	7d  -> 30-min averages    (~336 points)
//	30d -> 2-hour averages    (~360 points)
func (h *MetricsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	rangeStr := r.URL.Query().Get("range")
	if rangeStr == "" {
		rangeStr = "24h"
	}

	now := time.Now().UTC()
	from, maxPoints := parseTimeRange(rangeStr, now)

	metrics, err := models.GetSystemMetricsRange(h.DB, from, now, maxPoints)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch metrics history")
		return
	}
	if metrics == nil {
		metrics = []models.SystemMetric{}
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
// of data points. These max-point values produce the desired downsampling
// granularity given typical 10-second collection intervals.
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
		// Default to 24h.
		return now.Add(-24 * time.Hour), 288
	}
}
