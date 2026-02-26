package database

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Migrate runs all schema migrations, creates indexes, seeds default settings,
// and ensures the default admin user exists.
func Migrate(db *sql.DB, adminUser, adminPassword string) error {
	if err := createTables(db); err != nil {
		return err
	}
	if err := createIndexes(db); err != nil {
		return err
	}
	if err := seedSettings(db); err != nil {
		return err
	}
	if err := seedAdminUser(db, adminUser, adminPassword); err != nil {
		return err
	}
	return nil
}

func createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS metrics (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp     DATETIME NOT NULL,
			cpu_percent   REAL     NOT NULL DEFAULT 0,
			ram_percent   REAL     NOT NULL DEFAULT 0,
			ram_used_mb   INTEGER  NOT NULL DEFAULT 0,
			ram_total_mb  INTEGER  NOT NULL DEFAULT 0,
			disk_percent  REAL     NOT NULL DEFAULT 0,
			disk_used_gb  REAL     NOT NULL DEFAULT 0,
			disk_total_gb REAL     NOT NULL DEFAULT 0,
			swap_percent  REAL     NOT NULL DEFAULT 0,
			load_1m       REAL     NOT NULL DEFAULT 0,
			load_5m       REAL     NOT NULL DEFAULT 0,
			load_15m      REAL     NOT NULL DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS containers (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			image        TEXT NOT NULL DEFAULT '',
			status       TEXT NOT NULL DEFAULT '',
			health       TEXT NOT NULL DEFAULT '',
			started_at   DATETIME,
			cpu_percent  REAL    NOT NULL DEFAULT 0,
			mem_percent  REAL    NOT NULL DEFAULT 0,
			mem_usage_mb REAL    NOT NULL DEFAULT 0,
			mem_limit_mb REAL    NOT NULL DEFAULT 0,
			is_critical  INTEGER NOT NULL DEFAULT 0,
			last_updated DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS container_metrics (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			container_id   TEXT    NOT NULL,
			container_name TEXT    NOT NULL DEFAULT '',
			timestamp      DATETIME NOT NULL,
			cpu_percent    REAL    NOT NULL DEFAULT 0,
			mem_percent    REAL    NOT NULL DEFAULT 0,
			mem_usage_mb   REAL    NOT NULL DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS logins (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp   DATETIME NOT NULL,
			event_type  TEXT    NOT NULL DEFAULT '',
			username    TEXT    NOT NULL DEFAULT '',
			ip          TEXT    NOT NULL DEFAULT '',
			method      TEXT    NOT NULL DEFAULT '',
			attempts    INTEGER NOT NULL DEFAULT 0,
			command     TEXT    NOT NULL DEFAULT '',
			by_user     TEXT    NOT NULL DEFAULT '',
			geo_country TEXT    NOT NULL DEFAULT '',
			geo_city    TEXT    NOT NULL DEFAULT '',
			geo_isp     TEXT    NOT NULL DEFAULT '',
			geo_lat     REAL    NOT NULL DEFAULT 0,
			geo_lon     REAL    NOT NULL DEFAULT 0,
			raw_line    TEXT    NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS alerts (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME NOT NULL,
			type      TEXT    NOT NULL DEFAULT '',
			alert_key TEXT    NOT NULL DEFAULT '',
			message   TEXT    NOT NULL DEFAULT '',
			status    TEXT    NOT NULL DEFAULT '',
			http_code INTEGER NOT NULL DEFAULT 0,
			details   TEXT    NOT NULL DEFAULT '',
			raw_line  TEXT    NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS banned_ips (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			ip          TEXT    NOT NULL DEFAULT '',
			jail        TEXT    NOT NULL DEFAULT '',
			banned_at   DATETIME NOT NULL,
			unbanned_at DATETIME,
			country     TEXT    NOT NULL DEFAULT '',
			city        TEXT    NOT NULL DEFAULT '',
			isp         TEXT    NOT NULL DEFAULT '',
			lat         REAL    NOT NULL DEFAULT 0,
			lon         REAL    NOT NULL DEFAULT 0,
			is_proxy    INTEGER NOT NULL DEFAULT 0,
			is_active   INTEGER NOT NULL DEFAULT 1
		)`,

		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL,
			created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
			last_login    DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
	}

	for _, ddl := range tables {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("database: migrate table: %w", err)
		}
	}
	return nil
}

func createIndexes(db *sql.DB) error {
	indexes := []string{
		// metrics
		`CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp)`,

		// container_metrics
		`CREATE INDEX IF NOT EXISTS idx_container_metrics_container_id ON container_metrics(container_id)`,
		`CREATE INDEX IF NOT EXISTS idx_container_metrics_timestamp ON container_metrics(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_container_metrics_cid_ts ON container_metrics(container_id, timestamp)`,

		// logins
		`CREATE INDEX IF NOT EXISTS idx_logins_timestamp ON logins(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_logins_event_type ON logins(event_type)`,
		`CREATE INDEX IF NOT EXISTS idx_logins_ip ON logins(ip)`,
		`CREATE INDEX IF NOT EXISTS idx_logins_username ON logins(username)`,

		// alerts
		`CREATE INDEX IF NOT EXISTS idx_alerts_timestamp ON alerts(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_alert_key ON alerts(alert_key)`,

		// banned_ips
		`CREATE INDEX IF NOT EXISTS idx_banned_ips_ip ON banned_ips(ip)`,
		`CREATE INDEX IF NOT EXISTS idx_banned_ips_is_active ON banned_ips(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_banned_ips_banned_at ON banned_ips(banned_at)`,

		// users
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
	}

	for _, ddl := range indexes {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("database: create index: %w", err)
		}
	}
	return nil
}

func seedSettings(db *sql.DB) error {
	defaults := map[string]string{
		"metrics_retention_days": "30",
		"logins_retention_days":  "90",
		"alerts_retention_days":  "90",
		"theme":                  "dark",
		"dashboard_refresh_ms":   "5000",
		"alerts_enabled":         "true",
	}

	stmt, err := db.Prepare(`INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("database: prepare seed settings: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for k, v := range defaults {
		if _, err := stmt.Exec(k, v, now); err != nil {
			return fmt.Errorf("database: seed setting %s: %w", k, err)
		}
	}
	return nil
}

func seedAdminUser(db *sql.DB, username, password string) error {
	if username == "" || password == "" {
		return nil
	}

	// Check whether the user already exists.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count); err != nil {
		return fmt.Errorf("database: check admin user: %w", err)
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("database: hash admin password: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(hash), time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("database: insert admin user: %w", err)
	}
	return nil
}
