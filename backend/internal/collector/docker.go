package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// DockerCollector periodically queries the Docker daemon for container state
// and resource usage, writes results to the database and broadcasts them via
// WebSocket.
type DockerCollector struct {
	db                 *sql.DB
	hub                *ws.Hub
	interval           time.Duration
	cli                *client.Client
	criticalContainers map[string]bool
}

// NewDockerCollector creates a DockerCollector. It will attempt to connect to
// the Docker daemon via the default environment settings (typically the Unix
// socket at /var/run/docker.sock).
func NewDockerCollector(db *sql.DB, hub *ws.Hub, interval time.Duration, criticalNames []string) (*DockerCollector, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	crit := make(map[string]bool, len(criticalNames))
	for _, n := range criticalNames {
		crit[strings.ToLower(n)] = true
	}
	return &DockerCollector{
		db:                 db,
		hub:                hub,
		interval:           interval,
		cli:                cli,
		criticalContainers: crit,
	}, nil
}

// Start begins the periodic collection loop. It blocks until ctx is cancelled.
func (dc *DockerCollector) Start(ctx context.Context) {
	log.Printf("[docker] collector starting (interval=%s)", dc.interval)

	ticker := time.NewTicker(dc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[docker] collector stopped")
			dc.cli.Close()
			return
		case <-ticker.C:
			if err := dc.Collect(ctx); err != nil {
				log.Printf("[docker] collect error: %v", err)
			}
		}
	}
}

// Collect performs one collection cycle: lists all containers, fetches stats
// for running ones, persists the data and broadcasts it.
func (dc *DockerCollector) Collect(ctx context.Context) error {
	containers, err := dc.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	now := time.Now().UTC()
	results := make([]models.Container, 0, len(containers))
	cmetrics := make([]models.ContainerMetric, 0, len(containers))

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Determine health status from container status string.
		health := extractHealth(c.Status)

		startedAt := time.Unix(c.Created, 0).UTC()

		ctr := models.Container{
			ID:          c.ID[:12],
			Name:        name,
			Image:       c.Image,
			Status:      c.Status,
			Health:      health,
			StartedAt:   startedAt,
			IsCritical:  dc.criticalContainers[strings.ToLower(name)],
			LastUpdated: now,
		}

		// Only fetch live stats for running containers.
		if c.State == "running" {
			cpuPct, memPct, memUsageMB, memLimitMB, err := dc.fetchStats(ctx, c.ID)
			if err != nil {
				log.Printf("[docker] stats %s: %v", name, err)
			} else {
				ctr.CPUPercent = cpuPct
				ctr.MemPercent = memPct
				ctr.MemUsageMB = memUsageMB
				ctr.MemLimitMB = memLimitMB

				cm := models.ContainerMetric{
					ContainerID:   c.ID[:12],
					ContainerName: name,
					Timestamp:     now,
					CPUPercent:    cpuPct,
					MemPercent:    memPct,
					MemUsageMB:    memUsageMB,
				}
				if _, err := models.InsertContainerMetric(dc.db, &cm); err != nil {
					log.Printf("[docker] insert metric %s: %v", name, err)
				}
				cmetrics = append(cmetrics, cm)
			}
		}

		// Persist the container record.
		if err := models.UpsertContainer(dc.db, &ctr); err != nil {
			log.Printf("[docker] upsert container %s: %v", name, err)
		}

		results = append(results, ctr)
	}

	// Broadcast aggregated results.
	dc.hub.BroadcastMessage(&ws.Message{
		Type: "containers",
		Data: map[string]interface{}{
			"containers": results,
			"metrics":    cmetrics,
		},
	})

	return nil
}

// fetchStats retrieves a one-shot stats snapshot for a single container and
// computes CPU and memory percentages.
func (dc *DockerCollector) fetchStats(ctx context.Context, fullID string) (cpuPct, memPct, memUsageMB, memLimitMB float64, err error) {
	statsResp, err := dc.cli.ContainerStatsOneShot(ctx, fullID)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("container stats: %w", err)
	}
	defer statsResp.Body.Close()

	body, err := io.ReadAll(statsResp.Body)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("read stats body: %w", err)
	}

	var stats container.StatsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("unmarshal stats: %w", err)
	}

	cpuPct = calculateCPUPercent(&stats)

	if stats.MemoryStats.Limit > 0 {
		// Subtract cache from usage for a more accurate "real" memory figure.
		usedBytes := stats.MemoryStats.Usage
		if v, ok := stats.MemoryStats.Stats["cache"]; ok {
			usedBytes -= v
		}
		memPct = float64(usedBytes) / float64(stats.MemoryStats.Limit) * 100
		memUsageMB = float64(usedBytes) / (1024 * 1024)
		memLimitMB = float64(stats.MemoryStats.Limit) / (1024 * 1024)
	}

	return round2(cpuPct), round2(memPct), round2(memUsageMB), round2(memLimitMB), nil
}

// calculateCPUPercent computes the CPU usage percentage from the Docker stats
// response by comparing the delta of container CPU usage against the delta of
// system CPU usage.
func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) -
		float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) -
		float64(stats.PreCPUStats.SystemUsage)

	if systemDelta <= 0 || cpuDelta < 0 {
		return 0
	}

	numCPUs := float64(stats.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		numCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if numCPUs == 0 {
		numCPUs = 1
	}

	return (cpuDelta / systemDelta) * numCPUs * 100.0
}

// extractHealth returns a normalised health string from a Docker container
// status line, e.g. "Up 5 minutes (healthy)" -> "healthy".
func extractHealth(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "(healthy)"):
		return "healthy"
	case strings.Contains(lower, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(lower, "(health: starting)"):
		return "starting"
	default:
		return ""
	}
}
