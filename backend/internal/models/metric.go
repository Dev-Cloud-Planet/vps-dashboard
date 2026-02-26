package models

import (
	"database/sql"
	"fmt"
	"time"
)

// Metric holds a point-in-time snapshot of system resource usage.
type Metric struct {
	ID          int64     `json:"id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	RAMPercent  float64   `json:"ram_percent"`
	RAMUsedMB   int       `json:"ram_used_mb"`
	RAMTotalMB  int       `json:"ram_total_mb"`
	DiskPercent float64   `json:"disk_percent"`
	DiskUsedGB  float64   `json:"disk_used_gb"`
	DiskTotalGB float64   `json:"disk_total_gb"`
	SwapPercent float64   `json:"swap_percent"`
	Load1m      float64   `json:"load_1m"`
	Load5m      float64   `json:"load_5m"`
	Load15m     float64   `json:"load_15m"`
}

// InsertMetric stores a new system metrics snapshot and returns the row id.
func InsertMetric(db *sql.DB, m *Metric) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO metrics (
			timestamp, cpu_percent, ram_percent, ram_used_mb, ram_total_mb,
			disk_percent, disk_used_gb, disk_total_gb, swap_percent,
			load_1m, load_5m, load_15m
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Timestamp.UTC(), m.CPUPercent, m.RAMPercent, m.RAMUsedMB, m.RAMTotalMB,
		m.DiskPercent, m.DiskUsedGB, m.DiskTotalGB, m.SwapPercent,
		m.Load1m, m.Load5m, m.Load15m,
	)
	if err != nil {
		return 0, fmt.Errorf("insert metric: %w", err)
	}
	return res.LastInsertId()
}

// GetLatestMetric returns the most recent system metrics row.
func GetLatestMetric(db *sql.DB) (*Metric, error) {
	m := &Metric{}
	err := db.QueryRow(`
		SELECT id, timestamp, cpu_percent, ram_percent, ram_used_mb, ram_total_mb,
		       disk_percent, disk_used_gb, disk_total_gb, swap_percent,
		       load_1m, load_5m, load_15m
		FROM metrics ORDER BY timestamp DESC LIMIT 1`,
	).Scan(
		&m.ID, &m.Timestamp, &m.CPUPercent, &m.RAMPercent, &m.RAMUsedMB, &m.RAMTotalMB,
		&m.DiskPercent, &m.DiskUsedGB, &m.DiskTotalGB, &m.SwapPercent,
		&m.Load1m, &m.Load5m, &m.Load15m,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest metric: %w", err)
	}
	return m, nil
}

// GetMetricsRange returns metrics between from and to. When the raw row count
// exceeds maxPoints the data is automatically downsampled by bucketing rows
// into equal time windows and averaging the values within each bucket.
func GetMetricsRange(db *sql.DB, from, to time.Time, maxPoints int) ([]Metric, error) {
	if maxPoints <= 0 {
		maxPoints = 500
	}

	// Determine the total number of rows in the range so we know whether
	// downsampling is necessary.
	var total int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM metrics WHERE timestamp BETWEEN ? AND ?`,
		from.UTC(), to.UTC(),
	).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count metrics: %w", err)
	}

	if total <= maxPoints {
		return queryMetricsRaw(db, from, to)
	}

	return queryMetricsDownsampled(db, from, to, maxPoints)
}

func queryMetricsRaw(db *sql.DB, from, to time.Time) ([]Metric, error) {
	rows, err := db.Query(`
		SELECT id, timestamp, cpu_percent, ram_percent, ram_used_mb, ram_total_mb,
		       disk_percent, disk_used_gb, disk_total_gb, swap_percent,
		       load_1m, load_5m, load_15m
		FROM metrics
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC`,
		from.UTC(), to.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query metrics raw: %w", err)
	}
	defer rows.Close()

	return scanMetrics(rows)
}

func queryMetricsDownsampled(db *sql.DB, from, to time.Time, maxPoints int) ([]Metric, error) {
	// Calculate the bucket size in seconds so that we end up with at most
	// maxPoints buckets.
	rangeSec := to.Sub(from).Seconds()
	bucketSec := int(rangeSec) / maxPoints
	if bucketSec < 1 {
		bucketSec = 1
	}

	query := fmt.Sprintf(`
		SELECT 0 AS id,
		       datetime(
		           (strftime('%%s', timestamp) / %d) * %d, 'unixepoch'
		       ) AS bucket,
		       AVG(cpu_percent),
		       AVG(ram_percent),
		       CAST(AVG(ram_used_mb) AS INTEGER),
		       CAST(AVG(ram_total_mb) AS INTEGER),
		       AVG(disk_percent),
		       AVG(disk_used_gb),
		       AVG(disk_total_gb),
		       AVG(swap_percent),
		       AVG(load_1m),
		       AVG(load_5m),
		       AVG(load_15m)
		FROM metrics
		WHERE timestamp BETWEEN ? AND ?
		GROUP BY strftime('%%s', timestamp) / %d
		ORDER BY bucket ASC`,
		bucketSec, bucketSec, bucketSec,
	)

	rows, err := db.Query(query, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("query metrics downsampled: %w", err)
	}
	defer rows.Close()

	return scanMetrics(rows)
}

func scanMetrics(rows *sql.Rows) ([]Metric, error) {
	var out []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(
			&m.ID, &m.Timestamp, &m.CPUPercent, &m.RAMPercent, &m.RAMUsedMB, &m.RAMTotalMB,
			&m.DiskPercent, &m.DiskUsedGB, &m.DiskTotalGB, &m.SwapPercent,
			&m.Load1m, &m.Load5m, &m.Load15m,
		); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
