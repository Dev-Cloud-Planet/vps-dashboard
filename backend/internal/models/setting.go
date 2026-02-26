package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Setting represents a single key-value pair stored in the settings table.
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetSetting retrieves the value for the given key. Returns an empty string
// and no error when the key does not exist.
func GetSetting(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, nil
}

// SetSetting upserts a setting value. If the key already exists it is updated
// along with the updated_at timestamp.
func SetSetting(db *sql.DB, key, value string) error {
	now := time.Now().UTC()
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now,
	)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}

// GetAllSettings returns every stored setting as a slice.
func GetAllSettings(db *sql.DB) ([]Setting, error) {
	rows, err := db.Query(`SELECT key, value, updated_at FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()

	var out []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
