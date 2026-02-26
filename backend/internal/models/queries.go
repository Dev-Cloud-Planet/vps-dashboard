package models

import (
	"database/sql"
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Overview is the response type for the GET /api/overview endpoint.
// ---------------------------------------------------------------------------

type Overview struct {
	CPUPercent          float64 `json:"cpu_percent"`
	MemoryPercent       float64 `json:"memory_percent"`
	DiskPercent         float64 `json:"disk_percent"`
	LoadAvg1            float64 `json:"load_avg_1"`
	LoadAvg5            float64 `json:"load_avg_5"`
	LoadAvg15           float64 `json:"load_avg_15"`
	ContainersTotal     int     `json:"containers_total"`
	ContainersRunning   int     `json:"containers_running"`
	ContainersStopped   int     `json:"containers_stopped"`
	ContainersUnhealthy int     `json:"containers_unhealthy"`
	RecentAlerts        int     `json:"recent_alerts"`
	RecentLogins        int     `json:"recent_logins"`
	UptimeSeconds       float64 `json:"uptime_seconds"`
	ActiveSSHSessions   int     `json:"active_ssh_sessions"`
}

// ---------------------------------------------------------------------------
// Container state counts.
// ---------------------------------------------------------------------------

// CountContainersByState returns (total, running, stopped, unhealthy) counts.
func CountContainersByState(db *sql.DB) (total, running, stopped, unhealthy int, err error) {
	rows, err := db.Query(`SELECT status, health FROM containers`)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var status, health string
		if err := rows.Scan(&status, &health); err != nil {
			return 0, 0, 0, 0, err
		}
		total++
		if isContainerRunning(status) {
			running++
		} else {
			stopped++
		}
		if health == "unhealthy" {
			unhealthy++
		}
	}
	return total, running, stopped, unhealthy, rows.Err()
}

func isContainerRunning(status string) bool {
	if len(status) >= 2 && status[:2] == "Up" {
		return true
	}
	return status == "running"
}

// ---------------------------------------------------------------------------
// Container metric queries for the API layer.
// ---------------------------------------------------------------------------

// ContainerMetricPoint is the API response type for container metric history.
type ContainerMetricPoint struct {
	ID          int64     `json:"id,omitempty"`
	ContainerID string    `json:"container_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	MemUsageMB  float64   `json:"mem_usage_mb"`
	Timestamp   time.Time `json:"timestamp"`
}

// GetContainerMetricsRange returns container metrics between from and to
// with automatic downsampling.
func GetContainerMetricsRange(db *sql.DB, containerID string, from, to time.Time, maxPoints int) ([]ContainerMetricPoint, error) {
	if maxPoints <= 0 {
		maxPoints = 500
	}

	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM container_metrics WHERE container_id = ? AND timestamp BETWEEN ? AND ?`,
		containerID, from.UTC(), to.UTC(),
	).Scan(&count)
	if err != nil {
		return nil, err
	}

	if count <= maxPoints {
		return queryContainerMetricPointsRaw(db, containerID, from, to)
	}

	rangeSec := to.Sub(from).Seconds()
	bucketSec := int(rangeSec) / maxPoints
	if bucketSec < 1 {
		bucketSec = 1
	}

	q := fmt.Sprintf(`
		SELECT 0 AS id, container_id,
		       AVG(cpu_percent), AVG(mem_percent), AVG(mem_usage_mb),
		       datetime((strftime('%%%%s', timestamp) / %d) * %d, 'unixepoch') AS bucket
		FROM container_metrics
		WHERE container_id = ? AND timestamp BETWEEN ? AND ?
		GROUP BY strftime('%%%%s', timestamp) / %d
		ORDER BY bucket ASC`, bucketSec, bucketSec, bucketSec)

	rows, err := db.Query(q, containerID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContainerMetricPoints(rows)
}

func queryContainerMetricPointsRaw(db *sql.DB, cid string, from, to time.Time) ([]ContainerMetricPoint, error) {
	rows, err := db.Query(`
		SELECT id, container_id, cpu_percent, mem_percent, mem_usage_mb, timestamp
		FROM container_metrics
		WHERE container_id = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC`, cid, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContainerMetricPoints(rows)
}

func scanContainerMetricPoints(rows *sql.Rows) ([]ContainerMetricPoint, error) {
	var out []ContainerMetricPoint
	for rows.Next() {
		var m ContainerMetricPoint
		if err := rows.Scan(&m.ID, &m.ContainerID, &m.CPUPercent, &m.MemPercent,
			&m.MemUsageMB, &m.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Login event queries (logins table).
// ---------------------------------------------------------------------------

// LoginEvent is the type used by the logins table.
type LoginEvent struct {
	ID         int64     `json:"id,omitempty"`
	EventType  string    `json:"event_type"`
	Username   string    `json:"username"`
	IP         string    `json:"ip"`
	Method     string    `json:"method"`
	Attempts   int       `json:"attempts"`
	GeoCountry string    `json:"country"`
	GeoCity    string    `json:"city"`
	GeoISP     string    `json:"isp"`
	Timestamp  time.Time `json:"timestamp"`
}

// ListLoginEvents returns paginated login events with filters.
func ListLoginEvents(db *sql.DB, eventType, ip string, page, perPage int) ([]LoginEvent, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}

	where := " WHERE 1=1"
	args := []interface{}{}
	if eventType != "" {
		where += " AND event_type = ?"
		args = append(args, eventType)
	}
	if ip != "" {
		where += " AND ip = ?"
		args = append(args, ip)
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM logins"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	q := `SELECT id, event_type, username, ip, method, attempts,
	             geo_country, geo_city, geo_isp, timestamp
	      FROM logins` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	rowArgs := make([]interface{}, len(args))
	copy(rowArgs, args)
	rowArgs = append(rowArgs, perPage, offset)

	rows, err := db.Query(q, rowArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []LoginEvent
	for rows.Next() {
		var e LoginEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Username, &e.IP, &e.Method,
			&e.Attempts, &e.GeoCountry, &e.GeoCity, &e.GeoISP, &e.Timestamp); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// LoginEventStats holds aggregate statistics for login events.
type LoginEventStats struct {
	CountByType     map[string]int `json:"count_by_type"`
	TopAttackingIPs []IPCountStat  `json:"top_attacking_ips"`
	TodayCount      int            `json:"today_count"`
	YesterdayCount  int            `json:"yesterday_count"`
}

// IPCountStat pairs an IP with a count.
type IPCountStat struct {
	IP      string `json:"ip"`
	Count   int    `json:"count"`
	Country string `json:"country"`
}

// GetLoginEventStats returns aggregate stats from the logins table.
func GetLoginEventStats(db *sql.DB) (*LoginEventStats, error) {
	stats := &LoginEventStats{CountByType: make(map[string]int)}

	typeRows, err := db.Query(`SELECT event_type, COUNT(*) FROM logins GROUP BY event_type`)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var t string
		var c int
		if err := typeRows.Scan(&t, &c); err != nil {
			return nil, err
		}
		stats.CountByType[t] = c
	}
	if err := typeRows.Err(); err != nil {
		return nil, err
	}

	ipRows, err := db.Query(`
		SELECT ip, COUNT(*) AS cnt, COALESCE(geo_country, '') AS country
		FROM logins WHERE event_type IN ('login_fail', 'LOGIN_FAIL')
		GROUP BY ip ORDER BY cnt DESC LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer ipRows.Close()
	for ipRows.Next() {
		var s IPCountStat
		if err := ipRows.Scan(&s.IP, &s.Count, &s.Country); err != nil {
			return nil, err
		}
		stats.TopAttackingIPs = append(stats.TopAttackingIPs, s)
	}
	if err := ipRows.Err(); err != nil {
		return nil, err
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	_ = db.QueryRow(`SELECT COUNT(*) FROM logins WHERE timestamp >= ?`, today).Scan(&stats.TodayCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM logins WHERE timestamp >= ? AND timestamp < ?`,
		yesterday, today).Scan(&stats.YesterdayCount)

	return stats, nil
}

// CountRecentLoginEvents returns the count of logins since the given time.
func CountRecentLoginEvents(db *sql.DB, since time.Time) (int, error) {
	var c int
	err := db.QueryRow(`SELECT COUNT(*) FROM logins WHERE timestamp >= ?`, since.UTC()).Scan(&c)
	return c, err
}

// ---------------------------------------------------------------------------
// Alert queries (alerts table).
// ---------------------------------------------------------------------------

// AlertRow represents a row from the alerts table for API responses.
type AlertRow struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	AlertKey  string    `json:"alert_key"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	HTTPCode  int       `json:"http_code"`
	Details   string    `json:"details"`
}

// ListAlertRows returns paginated alerts.
func ListAlertRows(db *sql.DB, alertType, status string, page, perPage int) ([]AlertRow, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}

	where := " WHERE 1=1"
	args := []interface{}{}
	if alertType != "" {
		where += " AND type = ?"
		args = append(args, alertType)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	q := `SELECT id, timestamp, type, alert_key, message, status, http_code, details
	      FROM alerts` + where + ` ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	rowArgs := make([]interface{}, len(args))
	copy(rowArgs, args)
	rowArgs = append(rowArgs, perPage, offset)

	rows, err := db.Query(q, rowArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []AlertRow
	for rows.Next() {
		var a AlertRow
		if err := rows.Scan(&a.ID, &a.Timestamp, &a.Type, &a.AlertKey, &a.Message,
			&a.Status, &a.HTTPCode, &a.Details); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// AlertRowStats holds aggregate statistics for the alerts table.
type AlertRowStats struct {
	CountByType   map[string]int `json:"count_by_type"`
	CountByStatus map[string]int `json:"count_by_status"`
	Last24h       int            `json:"last_24h"`
	Last7d        int            `json:"last_7d"`
	Last30d       int            `json:"last_30d"`
}

// GetAlertRowStats returns aggregate stats from the alerts table.
func GetAlertRowStats(db *sql.DB) (*AlertRowStats, error) {
	stats := &AlertRowStats{
		CountByType:   make(map[string]int),
		CountByStatus: make(map[string]int),
	}

	typeRows, err := db.Query(`SELECT type, COUNT(*) FROM alerts GROUP BY type`)
	if err != nil {
		return nil, err
	}
	defer typeRows.Close()
	for typeRows.Next() {
		var t string
		var c int
		if err := typeRows.Scan(&t, &c); err != nil {
			return nil, err
		}
		stats.CountByType[t] = c
	}

	statusRows, err := db.Query(`SELECT status, COUNT(*) FROM alerts WHERE status != '' GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var s string
		var c int
		if err := statusRows.Scan(&s, &c); err != nil {
			return nil, err
		}
		stats.CountByStatus[s] = c
	}

	now := time.Now().UTC()
	_ = db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE timestamp >= ?`, now.Add(-24*time.Hour)).Scan(&stats.Last24h)
	_ = db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE timestamp >= ?`, now.Add(-7*24*time.Hour)).Scan(&stats.Last7d)
	_ = db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE timestamp >= ?`, now.Add(-30*24*time.Hour)).Scan(&stats.Last30d)

	return stats, nil
}

// CountRecentAlertRows returns the count of alerts since the given time.
func CountRecentAlertRows(db *sql.DB, since time.Time) (int, error) {
	var c int
	err := db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE timestamp >= ?`, since.UTC()).Scan(&c)
	return c, err
}

// ---------------------------------------------------------------------------
// Banned IPs.
// ---------------------------------------------------------------------------

// BannedIPSimple is a simplified banned IP for API responses.
type BannedIPSimple struct {
	IP       string    `json:"ip"`
	Jail     string    `json:"jail"`
	Country  string    `json:"country"`
	City     string    `json:"city"`
	BannedAt time.Time `json:"banned_at"`
}

// ListBannedIPsSimple returns all banned IPs.
func ListBannedIPsSimple(db *sql.DB) ([]BannedIPSimple, error) {
	rows, err := db.Query(`SELECT ip, jail, country, city, banned_at FROM banned_ips ORDER BY banned_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BannedIPSimple
	for rows.Next() {
		var b BannedIPSimple
		if err := rows.Scan(&b.IP, &b.Jail, &b.Country, &b.City, &b.BannedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Settings queries.
// ---------------------------------------------------------------------------

// GetSettingsMap returns all settings as a key->value map.
func GetSettingsMap(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// UpsertSettingKV inserts or updates a single setting.
func UpsertSettingKV(db *sql.DB, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value)
	return err
}
