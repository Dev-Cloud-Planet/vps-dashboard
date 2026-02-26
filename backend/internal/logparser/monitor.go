package logparser

import (
	"regexp"
	"time"
)

// MonitorEvent represents a parsed line from monitor.log.
type MonitorEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	RawLine   string                 `json:"raw_line,omitempty"`
}

// Monitor log line patterns.
var (
	// HEARTBEAT: containers=5 cpu=12% ram=45% disk=68%
	reHeartbeat = regexp.MustCompile(
		`HEARTBEAT:\s*containers=(\d+)\s+cpu=(\d+(?:\.\d+)?)%\s+ram=(\d+(?:\.\d+)?)%\s+disk=(\d+(?:\.\d+)?)%`)

	// HIGH CPU: 92%  /  HIGH RAM: 87%  /  HIGH DISK: 95%
	reHighResource = regexp.MustCompile(
		`HIGH\s+(CPU|RAM|DISK):\s*(\d+(?:\.\d+)?)%`)

	// CONTAINER DOWN: myapp (exited)
	reContainerDown = regexp.MustCompile(
		`CONTAINER DOWN:\s*(\S+)\s*\(([^)]+)\)`)

	// CONTAINER RECOVERED: myapp
	reContainerRecovered = regexp.MustCompile(
		`CONTAINER RECOVERED:\s*(\S+)`)

	// PORT DOWN: HTTP (80)
	rePortDown = regexp.MustCompile(
		`PORT DOWN:\s*(\S+)\s*\((\d+)\)`)

	// PORT RECOVERED: HTTP (80)
	rePortRecovered = regexp.MustCompile(
		`PORT RECOVERED:\s*(\S+)\s*\((\d+)\)`)
)

// ParseMonitorLine parses a single line from monitor.log and returns a
// MonitorEvent. If the line does not match any known pattern, ok is false.
func ParseMonitorLine(line string) (MonitorEvent, bool) {
	ts, rest, hasTS := ParseTimestamp(line)
	if !hasTS {
		ts = time.Now().UTC()
		rest = line
	}

	evt := MonitorEvent{
		Timestamp: ts,
		RawLine:   line,
		Data:      make(map[string]interface{}),
	}

	// Try each pattern in order of frequency.

	if m := reHeartbeat.FindStringSubmatch(rest); m != nil {
		evt.Type = "HEARTBEAT"
		evt.Data["containers"] = parseInt(m[1])
		evt.Data["cpu_percent"] = parseFloat(m[2])
		evt.Data["ram_percent"] = parseFloat(m[3])
		evt.Data["disk_percent"] = parseFloat(m[4])
		return evt, true
	}

	if m := reHighResource.FindStringSubmatch(rest); m != nil {
		evt.Type = "HIGH_" + m[1]
		evt.Data["resource"] = m[1]
		evt.Data["percent"] = parseFloat(m[2])
		return evt, true
	}

	if m := reContainerDown.FindStringSubmatch(rest); m != nil {
		evt.Type = "CONTAINER_DOWN"
		evt.Data["name"] = m[1]
		evt.Data["status"] = m[2]
		return evt, true
	}

	if m := reContainerRecovered.FindStringSubmatch(rest); m != nil {
		evt.Type = "CONTAINER_RECOVERED"
		evt.Data["name"] = m[1]
		return evt, true
	}

	if m := rePortDown.FindStringSubmatch(rest); m != nil {
		evt.Type = "PORT_DOWN"
		evt.Data["service"] = m[1]
		evt.Data["port"] = parseInt(m[2])
		return evt, true
	}

	if m := rePortRecovered.FindStringSubmatch(rest); m != nil {
		evt.Type = "PORT_RECOVERED"
		evt.Data["service"] = m[1]
		evt.Data["port"] = parseInt(m[2])
		return evt, true
	}

	return MonitorEvent{}, false
}
