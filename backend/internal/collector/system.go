package collector

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// SystemCollector periodically reads system metrics from /proc and syscall and
// pushes them to the database and WebSocket hub.
type SystemCollector struct {
	db       *sql.DB
	hub      *ws.Hub
	interval time.Duration

	// Previous CPU sample for delta calculation.
	prevIdle  uint64
	prevTotal uint64

	// Previous network counters for delta calculation.
	prevNetIn  uint64
	prevNetOut uint64
}

// NewSystemCollector returns a configured SystemCollector.
func NewSystemCollector(db *sql.DB, hub *ws.Hub, interval time.Duration) *SystemCollector {
	return &SystemCollector{
		db:       db,
		hub:      hub,
		interval: interval,
	}
}

// Start begins the periodic collection loop. It blocks until the context is
// cancelled.
func (sc *SystemCollector) Start(ctx context.Context) {
	log.Printf("[system] collector starting (interval=%s)", sc.interval)

	// Take an initial CPU sample so the first Collect() can compute a delta.
	idle, total, err := readCPURaw()
	if err == nil {
		sc.prevIdle = idle
		sc.prevTotal = total
	}

	// Take initial network counters.
	netIn, netOut, err := readNetDev()
	if err == nil {
		sc.prevNetIn = netIn
		sc.prevNetOut = netOut
	}

	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[system] collector stopped")
			return
		case <-ticker.C:
			m, err := sc.Collect()
			if err != nil {
				log.Printf("[system] collect error: %v", err)
				continue
			}
			if _, err := models.InsertMetric(sc.db, &m); err != nil {
				log.Printf("[system] db insert error: %v", err)
			}
			sc.hub.BroadcastMessage(&ws.Message{
				Type: "system_metrics",
				Data: m,
			})
		}
	}
}

// Collect gathers a single Metric snapshot from the local system.
func (sc *SystemCollector) Collect() (models.Metric, error) {
	cpuPct, err := sc.ReadCPU()
	if err != nil {
		return models.Metric{}, fmt.Errorf("cpu: %w", err)
	}

	mem, err := ReadMemory()
	if err != nil {
		return models.Metric{}, fmt.Errorf("memory: %w", err)
	}

	disk, err := ReadDisk("/")
	if err != nil {
		return models.Metric{}, fmt.Errorf("disk: %w", err)
	}

	load, err := ReadLoadAvg()
	if err != nil {
		return models.Metric{}, fmt.Errorf("loadavg: %w", err)
	}

	totalMB := int(mem.totalKB / 1024)
	usedKB := mem.totalKB - mem.availableKB
	usedMB := int(usedKB / 1024)
	ramPct := 0.0
	if mem.totalKB > 0 {
		ramPct = float64(usedKB) / float64(mem.totalKB) * 100
	}

	swapPct := 0.0
	if mem.swapTotalKB > 0 {
		swapUsed := mem.swapTotalKB - mem.swapFreeKB
		swapPct = float64(swapUsed) / float64(mem.swapTotalKB) * 100
	}

	m := models.Metric{
		Timestamp:   time.Now().UTC(),
		CPUPercent:  round2(cpuPct),
		RAMPercent:  round2(ramPct),
		RAMUsedMB:   usedMB,
		RAMTotalMB:  totalMB,
		DiskPercent: round2(disk.percent),
		DiskUsedGB:  round2(disk.usedGB),
		DiskTotalGB: round2(disk.totalGB),
		SwapPercent: round2(swapPct),
		Load1m:      load.load1,
		Load5m:      load.load5,
		Load15m:     load.load15,
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// CPU
// ---------------------------------------------------------------------------

// ReadCPU returns overall CPU usage as a percentage (0-100) since the last
// invocation, computed from delta values read from /proc/stat.
func (sc *SystemCollector) ReadCPU() (float64, error) {
	idle, total, err := readCPURaw()
	if err != nil {
		return 0, err
	}

	deltaIdle := idle - sc.prevIdle
	deltaTotal := total - sc.prevTotal

	sc.prevIdle = idle
	sc.prevTotal = total

	if deltaTotal == 0 {
		return 0, nil
	}

	cpuPct := (1.0 - float64(deltaIdle)/float64(deltaTotal)) * 100
	if cpuPct < 0 {
		cpuPct = 0
	}
	return cpuPct, nil
}

// readCPURaw parses the first "cpu" line of /proc/stat and returns aggregate
// idle and total jiffies.
func readCPURaw() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("/proc/stat cpu line too short: %q", line)
		}
		// fields: cpu user nice system idle iowait irq softirq steal guest guest_nice
		var vals [10]uint64
		for i := 1; i < len(fields) && i <= 10; i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			vals[i-1] = v
		}
		// idle = idle + iowait (indices 3 and 4)
		idleVal := vals[3]
		if len(fields) > 5 {
			idleVal += vals[4]
		}
		var totalVal uint64
		for i := 0; i < len(fields)-1 && i < 10; i++ {
			totalVal += vals[i]
		}
		return idleVal, totalVal, nil
	}
	return 0, 0, fmt.Errorf("no cpu line found in /proc/stat")
}

// ---------------------------------------------------------------------------
// Memory
// ---------------------------------------------------------------------------

type memInfo struct {
	totalKB     uint64
	freeKB      uint64
	availableKB uint64
	buffersKB   uint64
	cachedKB    uint64
	swapTotalKB uint64
	swapFreeKB  uint64
}

// ReadMemory returns parsed memory information from /proc/meminfo.
func ReadMemory() (memInfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return memInfo{}, fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	m := memInfo{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			m.totalKB = val
		case "MemFree":
			m.freeKB = val
		case "MemAvailable":
			m.availableKB = val
		case "Buffers":
			m.buffersKB = val
		case "Cached":
			m.cachedKB = val
		case "SwapTotal":
			m.swapTotalKB = val
		case "SwapFree":
			m.swapFreeKB = val
		}
	}

	// Fallback: if MemAvailable is not present (very old kernels), estimate it.
	if m.availableKB == 0 && m.totalKB > 0 {
		m.availableKB = m.freeKB + m.buffersKB + m.cachedKB
	}

	return m, scanner.Err()
}

// ---------------------------------------------------------------------------
// Disk
// ---------------------------------------------------------------------------

type diskInfo struct {
	totalGB float64
	usedGB  float64
	percent float64
}

// ReadDisk returns filesystem usage for the given mount path.
func ReadDisk(path string) (diskInfo, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return diskInfo{}, fmt.Errorf("statfs %s: %w", path, err)
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB := float64(totalBytes) / (1 << 30)
	usedGB := float64(usedBytes) / (1 << 30)

	pct := 0.0
	// Use the same formula as df: used / (used + avail) * 100.
	usedPlusAvail := usedBytes + availBytes
	if usedPlusAvail > 0 {
		pct = float64(usedBytes) / float64(usedPlusAvail) * 100
	}

	return diskInfo{totalGB: totalGB, usedGB: usedGB, percent: pct}, nil
}

// ---------------------------------------------------------------------------
// Load Average
// ---------------------------------------------------------------------------

type loadAvg struct {
	load1  float64
	load5  float64
	load15 float64
}

// ReadLoadAvg returns the 1, 5, and 15-minute load averages.
func ReadLoadAvg() (loadAvg, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return loadAvg{}, fmt.Errorf("read /proc/loadavg: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return loadAvg{}, fmt.Errorf("/proc/loadavg too short")
	}

	l := loadAvg{}
	l.load1, _ = strconv.ParseFloat(fields[0], 64)
	l.load5, _ = strconv.ParseFloat(fields[1], 64)
	l.load15, _ = strconv.ParseFloat(fields[2], 64)
	return l, nil
}

// ---------------------------------------------------------------------------
// Network I/O
// ---------------------------------------------------------------------------

// readNetDev sums rx_bytes (col 1) and tx_bytes (col 9) for all non-lo
// interfaces from /proc/net/dev.
func readNetDev() (rxTotal, txTotal uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// Skip the two header lines.
		if lineNum <= 2 {
			continue
		}
		line := scanner.Text()
		// Format: "  iface: rx_bytes rx_packets ... tx_bytes tx_packets ..."
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		rxTotal += rx
		txTotal += tx
	}
	return rxTotal, txTotal, scanner.Err()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// round2 rounds a float to two decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
