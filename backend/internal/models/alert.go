package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Alert represents a parsed line from the alerts log.
type Alert struct {
	ID        int64     `json:"id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	AlertKey  string    `json:"alert_key"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	HTTPCode  int       `json:"http_code"`
	Details   string    `json:"details"`
	RawLine   string    `json:"raw_line,omitempty"`
}

// AlertStats contains aggregate counters for the alerts summary endpoint.
type AlertStats struct {
	Total    int            `json:"total"`
	ByType   map[string]int `json:"by_type"`
	ByStatus map[string]int `json:"by_status"`
	ByKey    map[string]int `json:"by_key"`
}

// InsertAlert stores an alert event and returns the row id.
func InsertAlert(db *sql.DB, a *Alert) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO alerts (
			timestamp, type, alert_key, message, status, http_code, details, raw_line
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Timestamp.UTC(), a.Type, a.AlertKey, a.Message, a.Status,
		a.HTTPCode, a.Details, a.RawLine,
	)
	if err != nil {
		return 0, fmt.Errorf("insert alert: %w", err)
	}
	return res.LastInsertId()
}

// ListAlerts returns a paginated list of alerts with optional filters on type
// and status.
func ListAlerts(db *sql.DB, alertType, status string, page, perPage int) ([]Alert, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}

	where, args := buildAlertWhere(alertType, status)

	// Total count.
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}

	offset := (page - 1) * perPage
	query := `
		SELECT id, timestamp, type, alert_key, message, status, http_code, details, raw_line
		FROM alerts` + where + `
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(
			&a.ID, &a.Timestamp, &a.Type, &a.AlertKey, &a.Message,
			&a.Status, &a.HTTPCode, &a.Details, &a.RawLine,
		); err != nil {
			return nil, 0, fmt.Errorf("scan alert: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// GetAlertStats returns aggregate statistics about alert events.
func GetAlertStats(db *sql.DB) (*AlertStats, error) {
	stats := &AlertStats{
		ByType:   make(map[string]int),
		ByStatus: make(map[string]int),
		ByKey:    make(map[string]int),
	}

	db.QueryRow(`SELECT COUNT(*) FROM alerts`).Scan(&stats.Total)

	typeRows, err := db.Query(`SELECT type, COUNT(*) FROM alerts GROUP BY type`)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var t string
			var c int
			if typeRows.Scan(&t, &c) == nil {
				stats.ByType[t] = c
			}
		}
	}

	statusRows, err := db.Query(`SELECT status, COUNT(*) FROM alerts WHERE status != '' GROUP BY status`)
	if err == nil {
		defer statusRows.Close()
		for statusRows.Next() {
			var s string
			var c int
			if statusRows.Scan(&s, &c) == nil {
				stats.ByStatus[s] = c
			}
		}
	}

	keyRows, err := db.Query(`SELECT alert_key, COUNT(*) FROM alerts WHERE alert_key != '' GROUP BY alert_key ORDER BY COUNT(*) DESC LIMIT 20`)
	if err == nil {
		defer keyRows.Close()
		for keyRows.Next() {
			var k string
			var c int
			if keyRows.Scan(&k, &c) == nil {
				stats.ByKey[k] = c
			}
		}
	}

	return stats, nil
}

func buildAlertWhere(alertType, status string) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	if alertType != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, alertType)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	where := " WHERE " + clauses[0]
	for _, c := range clauses[1:] {
		where += " AND " + c
	}
	return where, args
}
