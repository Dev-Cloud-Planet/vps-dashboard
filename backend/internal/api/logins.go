package api

import (
	"database/sql"
	"net/http"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// LoginsHandler holds dependencies for login event endpoints.
type LoginsHandler struct {
	DB *sql.DB
}

// List handles GET /api/logins?type=login_fail&ip=1.2.3.4&page=1&per_page=50.
// It returns a paginated list of login events with optional filters.
func (h *LoginsHandler) List(w http.ResponseWriter, r *http.Request) {
	eventType := r.URL.Query().Get("type")
	ip := r.URL.Query().Get("ip")
	pg := ParsePagination(r)

	events, total, err := models.ListLoginEvents(h.DB, eventType, ip, pg.Page, pg.PerPage)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not list login events")
		return
	}
	if events == nil {
		events = []models.LoginEvent{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":        events,
		"total":       total,
		"page":        pg.Page,
		"per_page":    pg.PerPage,
		"total_pages": TotalPages(total, pg.PerPage),
	})
}

// GetStats handles GET /api/logins/stats.
// It returns aggregated login statistics: counts by event type, top 10
// attacking IPs, and logins today vs yesterday.
func (h *LoginsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := models.GetLoginEventStats(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch login stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetBannedIPs handles GET /api/banned-ips.
// It returns the list of currently banned IPs from fail2ban.
func (h *LoginsHandler) GetBannedIPs(w http.ResponseWriter, r *http.Request) {
	ips, err := models.ListBannedIPsSimple(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not list banned IPs")
		return
	}
	if ips == nil {
		ips = []models.BannedIPSimple{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  ips,
		"total": len(ips),
	})
}
