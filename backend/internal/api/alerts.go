package api

import (
	"database/sql"
	"net/http"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// AlertsHandler holds dependencies for alert endpoints.
type AlertsHandler struct {
	DB *sql.DB
}

// List handles GET /api/alerts?type=resource&status=sent&page=1&per_page=50.
// It returns a paginated list of alerts with optional filters.
func (h *AlertsHandler) List(w http.ResponseWriter, r *http.Request) {
	alertType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	pg := ParsePagination(r)

	alerts, total, err := models.ListAlertRows(h.DB, alertType, status, pg.Page, pg.PerPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not list alerts")
		return
	}
	if alerts == nil {
		alerts = []models.AlertRow{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":        alerts,
		"total":       total,
		"page":        pg.Page,
		"per_page":    pg.PerPage,
		"total_pages": TotalPages(total, pg.PerPage),
	})
}

// GetStats handles GET /api/alerts/stats.
// It returns aggregated alert statistics: counts by type, by status, and
// counts for the last 24h, 7d, and 30d.
func (h *AlertsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetAlertRowStats(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch alert stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}
