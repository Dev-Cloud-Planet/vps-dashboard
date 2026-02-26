package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Container holds the current state and resource usage of a Docker container.
type Container struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Image       string    `json:"image"`
	Status      string    `json:"status"`
	Health      string    `json:"health"`
	StartedAt   time.Time `json:"started_at"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	MemUsageMB  float64   `json:"mem_usage_mb"`
	MemLimitMB  float64   `json:"mem_limit_mb"`
	IsCritical  bool      `json:"is_critical"`
	LastUpdated time.Time `json:"last_updated"`
}

// ContainerMetric is a point-in-time resource sample for a single container.
type ContainerMetric struct {
	ID            int64     `json:"id,omitempty"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	Timestamp     time.Time `json:"timestamp"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemPercent    float64   `json:"mem_percent"`
	MemUsageMB    float64   `json:"mem_usage_mb"`
}

// UpsertContainer inserts a container record or updates it if the id already
// exists.
func UpsertContainer(db *sql.DB, c *Container) error {
	critical := 0
	if c.IsCritical {
		critical = 1
	}
	_, err := db.Exec(`
		INSERT INTO containers (
			id, name, image, status, health, started_at,
			cpu_percent, mem_percent, mem_usage_mb, mem_limit_mb,
			is_critical, last_updated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name         = excluded.name,
			image        = excluded.image,
			status       = excluded.status,
			health       = excluded.health,
			started_at   = excluded.started_at,
			cpu_percent  = excluded.cpu_percent,
			mem_percent  = excluded.mem_percent,
			mem_usage_mb = excluded.mem_usage_mb,
			mem_limit_mb = excluded.mem_limit_mb,
			is_critical  = excluded.is_critical,
			last_updated = excluded.last_updated`,
		c.ID, c.Name, c.Image, c.Status, c.Health, c.StartedAt.UTC(),
		c.CPUPercent, c.MemPercent, c.MemUsageMB, c.MemLimitMB,
		critical, c.LastUpdated.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert container: %w", err)
	}
	return nil
}

// ListContainers returns all containers, optionally filtered by a
// case-insensitive substring match on name, image, or status.
func ListContainers(db *sql.DB, filter string) ([]Container, error) {
	query := `
		SELECT id, name, image, status, health, started_at,
		       cpu_percent, mem_percent, mem_usage_mb, mem_limit_mb,
		       is_critical, last_updated
		FROM containers`
	var args []interface{}

	if filter != "" {
		query += ` WHERE LOWER(name) LIKE ? OR LOWER(image) LIKE ? OR LOWER(status) LIKE ?`
		like := "%" + strings.ToLower(filter) + "%"
		args = append(args, like, like, like)
	}
	query += ` ORDER BY name ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer rows.Close()

	var out []Container
	for rows.Next() {
		var c Container
		var critical int
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Image, &c.Status, &c.Health, &c.StartedAt,
			&c.CPUPercent, &c.MemPercent, &c.MemUsageMB, &c.MemLimitMB,
			&critical, &c.LastUpdated,
		); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		c.IsCritical = critical != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetContainer returns a single container by its Docker id.
func GetContainer(db *sql.DB, id string) (*Container, error) {
	var c Container
	var critical int
	err := db.QueryRow(`
		SELECT id, name, image, status, health, started_at,
		       cpu_percent, mem_percent, mem_usage_mb, mem_limit_mb,
		       is_critical, last_updated
		FROM containers WHERE id = ?`, id,
	).Scan(
		&c.ID, &c.Name, &c.Image, &c.Status, &c.Health, &c.StartedAt,
		&c.CPUPercent, &c.MemPercent, &c.MemUsageMB, &c.MemLimitMB,
		&critical, &c.LastUpdated,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get container: %w", err)
	}
	c.IsCritical = critical != 0
	return &c, nil
}

// InsertContainerMetric stores a time-series sample for a container.
func InsertContainerMetric(db *sql.DB, cm *ContainerMetric) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO container_metrics (
			container_id, container_name, timestamp, cpu_percent, mem_percent, mem_usage_mb
		) VALUES (?, ?, ?, ?, ?, ?)`,
		cm.ContainerID, cm.ContainerName, cm.Timestamp.UTC(),
		cm.CPUPercent, cm.MemPercent, cm.MemUsageMB,
	)
	if err != nil {
		return 0, fmt.Errorf("insert container metric: %w", err)
	}
	return res.LastInsertId()
}
