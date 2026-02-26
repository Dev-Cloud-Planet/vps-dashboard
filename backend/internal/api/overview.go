package api

import (
	"bufio"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// OverviewHandler holds dependencies for the overview endpoint.
type OverviewHandler struct {
	DB *sql.DB
}

// GetOverview handles GET /api/overview.
// It returns aggregated dashboard data including the latest system metrics,
// container counts, recent alert and login counts, system uptime, and active
// SSH sessions.
func (h *OverviewHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	ov := models.Overview{}

	// Latest system metrics.
	metric, err := models.GetLatestSystemMetric(h.DB)
	if err == nil && metric != nil {
		ov.CPUPercent = metric.CPUPercent
		ov.MemoryPercent = metric.MemoryPercent
		ov.DiskPercent = metric.DiskPercent
		ov.LoadAvg1 = metric.LoadAvg1
		ov.LoadAvg5 = metric.LoadAvg5
		ov.LoadAvg15 = metric.LoadAvg15
	}

	// Container counts.
	total, running, stopped, unhealthy, err := models.CountContainersByState(h.DB)
	if err == nil {
		ov.ContainersTotal = total
		ov.ContainersRunning = running
		ov.ContainersStopped = stopped
		ov.ContainersUnhealthy = unhealthy
	}

	// Recent alerts (last 24h).
	since := time.Now().Add(-24 * time.Hour)
	if count, err := models.CountRecentAlertRows(h.DB, since); err == nil {
		ov.RecentAlerts = count
	}

	// Recent logins (last 24h).
	if count, err := models.CountRecentLoginEvents(h.DB, since); err == nil {
		ov.RecentLogins = count
	}

	// System uptime.
	ov.UptimeSeconds = readUptime()

	// Active SSH sessions.
	ov.ActiveSSHSessions = countSSHSessions()

	respondJSON(w, http.StatusOK, ov)
}

// readUptime reads the system uptime from /proc/uptime.
func readUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}
	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uptime
}

// countSSHSessions counts active SSH sessions by parsing the output-style of
// /var/run/utmp or, as a more portable fallback, reading /proc for sshd
// child processes. We use a simple approach: count lines in `who` output
// by reading /var/run/utmp entries (binary), or fall back to /proc scanning.
func countSSHSessions() int {
	// Try reading /var/run/utmp lines. A simpler heuristic: count pts/ entries
	// in /var/run/utmp. Since utmp is binary, we'll use /proc instead.
	return countSSHFromProc()
}

// countSSHFromProc counts unique pts sessions by scanning /proc/[pid]/stat
// for sshd child processes. A more reliable method: read
// /var/run/utmp or parse `who` output style from /proc/net.
// Simplest approach: count pseudo-terminal slave devices in /dev/pts.
func countSSHFromProc() int {
	// Count entries in /dev/pts/ (each represents a terminal session).
	// Subtract 1 for ptmx if present.
	entries, err := os.ReadDir("/dev/pts")
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if e.Name() == "ptmx" {
			continue
		}
		count++
	}

	// As a better heuristic, try to find sshd processes in /proc.
	// Look for processes whose cmdline contains "sshd:" (session processes).
	sshCount := countSSHProcesses()
	if sshCount > 0 {
		return sshCount
	}

	return count
}

// countSSHProcesses scans /proc for sshd session processes.
func countSSHProcesses() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only check numeric (PID) directories.
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		cmdlinePath := "/proc/" + e.Name() + "/cmdline"
		f, err := os.Open(cmdlinePath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Scan()
		line := scanner.Text()
		f.Close()

		// sshd session processes have cmdlines like "sshd: user@pts/0"
		if strings.Contains(line, "sshd:") && strings.Contains(line, "@") {
			count++
		}
	}
	return count
}
