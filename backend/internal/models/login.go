package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Login represents a parsed authentication event from the logins log.
type Login struct {
	ID         int64     `json:"id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	EventType  string    `json:"event_type"`
	Username   string    `json:"username"`
	IP         string    `json:"ip"`
	Method     string    `json:"method"`
	Attempts   int       `json:"attempts"`
	Command    string    `json:"command"`
	ByUser     string    `json:"by_user"`
	GeoCountry string    `json:"geo_country"`
	GeoCity    string    `json:"geo_city"`
	GeoISP     string    `json:"geo_isp"`
	GeoLat     float64   `json:"geo_lat"`
	GeoLon     float64   `json:"geo_lon"`
	RawLine    string    `json:"raw_line,omitempty"`
}

// LoginStats contains aggregate counters for the login summary endpoint.
type LoginStats struct {
	TotalLogins      int            `json:"total_logins"`
	SuccessfulLogins int            `json:"successful_logins"`
	FailedLogins     int            `json:"failed_logins"`
	UniqueIPs        int            `json:"unique_ips"`
	TopIPs           []IPCount      `json:"top_ips"`
	TopUsers         []UserCount    `json:"top_users"`
	ByType           map[string]int `json:"by_type"`
}

// IPCount pairs an IP address with its occurrence count.
type IPCount struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// UserCount pairs a username with its occurrence count.
type UserCount struct {
	Username string `json:"username"`
	Count    int    `json:"count"`
}

// InsertLogin stores a login event and returns the row id.
func InsertLogin(db *sql.DB, l *Login) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO logins (
			timestamp, event_type, username, ip, method, attempts,
			command, by_user, geo_country, geo_city, geo_isp,
			geo_lat, geo_lon, raw_line
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Timestamp.UTC(), l.EventType, l.Username, l.IP, l.Method, l.Attempts,
		l.Command, l.ByUser, l.GeoCountry, l.GeoCity, l.GeoISP,
		l.GeoLat, l.GeoLon, l.RawLine,
	)
	if err != nil {
		return 0, fmt.Errorf("insert login: %w", err)
	}
	return res.LastInsertId()
}

// ListLogins returns a paginated list of login events with optional filters
// on event type and IP address.
func ListLogins(db *sql.DB, eventType, ip string, page, perPage int) ([]Login, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}

	where, args := buildLoginWhere(eventType, ip)

	// Total count for pagination.
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM logins"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count logins: %w", err)
	}

	offset := (page - 1) * perPage
	query := `
		SELECT id, timestamp, event_type, username, ip, method, attempts,
		       command, by_user, geo_country, geo_city, geo_isp,
		       geo_lat, geo_lon, raw_line
		FROM logins` + where + `
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list logins: %w", err)
	}
	defer rows.Close()

	var out []Login
	for rows.Next() {
		var l Login
		if err := rows.Scan(
			&l.ID, &l.Timestamp, &l.EventType, &l.Username, &l.IP, &l.Method,
			&l.Attempts, &l.Command, &l.ByUser, &l.GeoCountry, &l.GeoCity,
			&l.GeoISP, &l.GeoLat, &l.GeoLon, &l.RawLine,
		); err != nil {
			return nil, 0, fmt.Errorf("scan login: %w", err)
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// GetLoginStats returns aggregate statistics about login events.
func GetLoginStats(db *sql.DB) (*LoginStats, error) {
	stats := &LoginStats{
		ByType: make(map[string]int),
	}

	// Total
	db.QueryRow(`SELECT COUNT(*) FROM logins`).Scan(&stats.TotalLogins)

	// Successful / failed
	db.QueryRow(`SELECT COUNT(*) FROM logins WHERE event_type = 'LOGIN_OK'`).Scan(&stats.SuccessfulLogins)
	db.QueryRow(`SELECT COUNT(*) FROM logins WHERE event_type = 'LOGIN_FAIL'`).Scan(&stats.FailedLogins)

	// Unique IPs
	db.QueryRow(`SELECT COUNT(DISTINCT ip) FROM logins WHERE ip != ''`).Scan(&stats.UniqueIPs)

	// By type
	typeRows, err := db.Query(`SELECT event_type, COUNT(*) FROM logins GROUP BY event_type`)
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

	// Top IPs
	ipRows, err := db.Query(`
		SELECT ip, COUNT(*) AS cnt
		FROM logins WHERE ip != ''
		GROUP BY ip ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer ipRows.Close()
		for ipRows.Next() {
			var ic IPCount
			if ipRows.Scan(&ic.IP, &ic.Count) == nil {
				stats.TopIPs = append(stats.TopIPs, ic)
			}
		}
	}

	// Top users
	userRows, err := db.Query(`
		SELECT username, COUNT(*) AS cnt
		FROM logins WHERE username != ''
		GROUP BY username ORDER BY cnt DESC LIMIT 10`)
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var uc UserCount
			if userRows.Scan(&uc.Username, &uc.Count) == nil {
				stats.TopUsers = append(stats.TopUsers, uc)
			}
		}
	}

	return stats, nil
}

func buildLoginWhere(eventType, ip string) (string, []interface{}) {
	var clauses []string
	var args []interface{}

	if eventType != "" {
		clauses = append(clauses, "event_type = ?")
		args = append(args, eventType)
	}
	if ip != "" {
		clauses = append(clauses, "ip = ?")
		args = append(args, ip)
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
