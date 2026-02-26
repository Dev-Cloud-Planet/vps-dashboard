package models

import (
	"database/sql"
	"fmt"
	"time"
)

// BannedIP represents a fail2ban ban/unban event for a single IP address.
type BannedIP struct {
	ID         int64      `json:"id,omitempty"`
	IP         string     `json:"ip"`
	Jail       string     `json:"jail"`
	BannedAt   time.Time  `json:"banned_at"`
	UnbannedAt *time.Time `json:"unbanned_at,omitempty"`
	Country    string     `json:"country"`
	City       string     `json:"city"`
	ISP        string     `json:"isp"`
	Lat        float64    `json:"lat"`
	Lon        float64    `json:"lon"`
	IsProxy    bool       `json:"is_proxy"`
	IsActive   bool       `json:"is_active"`
}

// InsertBannedIP stores a new ban record and returns the row id.
func InsertBannedIP(db *sql.DB, b *BannedIP) (int64, error) {
	proxy := 0
	if b.IsProxy {
		proxy = 1
	}
	active := 0
	if b.IsActive {
		active = 1
	}

	res, err := db.Exec(`
		INSERT INTO banned_ips (
			ip, jail, banned_at, unbanned_at,
			country, city, isp, lat, lon,
			is_proxy, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.IP, b.Jail, b.BannedAt.UTC(), nil,
		b.Country, b.City, b.ISP, b.Lat, b.Lon,
		proxy, active,
	)
	if err != nil {
		return 0, fmt.Errorf("insert banned ip: %w", err)
	}
	return res.LastInsertId()
}

// UpdateBannedIP marks a ban as lifted by setting the unbanned_at timestamp
// and clearing the is_active flag.
func UpdateBannedIP(db *sql.DB, ip, jail string, unbannedAt time.Time) error {
	_, err := db.Exec(`
		UPDATE banned_ips
		SET unbanned_at = ?, is_active = 0
		WHERE ip = ? AND jail = ? AND is_active = 1`,
		unbannedAt.UTC(), ip, jail,
	)
	if err != nil {
		return fmt.Errorf("update banned ip: %w", err)
	}
	return nil
}

// ListBannedIPs returns all ban records ordered by most recent first.
func ListBannedIPs(db *sql.DB) ([]BannedIP, error) {
	rows, err := db.Query(`
		SELECT id, ip, jail, banned_at, unbanned_at,
		       country, city, isp, lat, lon,
		       is_proxy, is_active
		FROM banned_ips
		ORDER BY banned_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list banned ips: %w", err)
	}
	defer rows.Close()

	return scanBannedIPs(rows)
}

// GetActiveBans returns only currently-active bans.
func GetActiveBans(db *sql.DB) ([]BannedIP, error) {
	rows, err := db.Query(`
		SELECT id, ip, jail, banned_at, unbanned_at,
		       country, city, isp, lat, lon,
		       is_proxy, is_active
		FROM banned_ips
		WHERE is_active = 1
		ORDER BY banned_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("get active bans: %w", err)
	}
	defer rows.Close()

	return scanBannedIPs(rows)
}

func scanBannedIPs(rows *sql.Rows) ([]BannedIP, error) {
	var out []BannedIP
	for rows.Next() {
		var b BannedIP
		var proxy, active int
		var unbannedAt sql.NullTime
		if err := rows.Scan(
			&b.ID, &b.IP, &b.Jail, &b.BannedAt, &unbannedAt,
			&b.Country, &b.City, &b.ISP, &b.Lat, &b.Lon,
			&proxy, &active,
		); err != nil {
			return nil, fmt.Errorf("scan banned ip: %w", err)
		}
		if unbannedAt.Valid {
			b.UnbannedAt = &unbannedAt.Time
		}
		b.IsProxy = proxy != 0
		b.IsActive = active != 0
		out = append(out, b)
	}
	return out, rows.Err()
}
