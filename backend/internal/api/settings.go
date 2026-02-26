package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// SettingsHandler holds dependencies for settings endpoints.
type SettingsHandler struct {
	DB *sql.DB
}

// Get handles GET /api/settings.
// It returns all settings as a key-value map.
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := models.GetSettingsMap(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch settings")
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

// Update handles PUT /api/settings.
// It accepts a JSON body with key-value pairs and upserts each setting.
//
// Example request body:
//
//	{
//	  "metrics_retention_days": "30",
//	  "alert_cooldown_seconds": "600"
//	}
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: expected JSON object with string values")
		return
	}

	if len(body) == 0 {
		respondError(w, http.StatusBadRequest, "no settings provided")
		return
	}

	for key, value := range body {
		if key == "" {
			continue
		}
		if err := models.UpsertSettingKV(h.DB, key, value); err != nil {
			respondError(w, http.StatusInternalServerError, "could not update setting: "+key)
			return
		}
	}

	// Return the updated settings.
	settings, err := models.GetSettingsMap(h.DB)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "settings updated but could not read back")
		return
	}

	respondJSON(w, http.StatusOK, settings)
}
