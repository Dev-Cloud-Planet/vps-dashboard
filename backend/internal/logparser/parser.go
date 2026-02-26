package logparser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Common timestamp formats found in the monitored log files.
const (
	// loginTimestampLayout matches the format used in logins.log:
	//   [2026-02-26T17:27:06.661347+00:00 ubuntu sshd[3824249]:]
	loginTimestampLayout = "2006-01-02T15:04:05.999999-07:00"

	// isoLayout is the standard ISO 8601 format used in many logs.
	isoLayout = "2006-01-02T15:04:05-07:00"

	// simpleLayout is a fallback without fractional seconds.
	simpleLayout = "2006-01-02 15:04:05"
)

// reLoginTimestamp captures the bracketed timestamp from logins.log.
// Example: [2026-02-26T17:27:06.661347+00:00 ubuntu sshd[3824249]:]
var reLoginTimestamp = regexp.MustCompile(
	`^\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+[+-]\d{2}:\d{2})\s`,
)

// reGenericTimestamp matches a leading ISO timestamp.
var reGenericTimestamp = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:\d{2}|Z)?)`,
)

// ParseLoginTimestamp extracts and parses the timestamp from a logins.log line.
func ParseLoginTimestamp(line string) (time.Time, string, bool) {
	m := reLoginTimestamp.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, line, false
	}
	t, err := time.Parse(loginTimestampLayout, m[1])
	if err != nil {
		return time.Time{}, line, false
	}
	// Return the remainder after the closing bracket.
	rest := line[len(m[0]):]
	if idx := strings.Index(rest, "]"); idx >= 0 {
		rest = rest[idx+1:]
	}
	rest = strings.TrimLeft(rest, ": ")
	return t.UTC(), rest, true
}

// ParseTimestamp tries to parse a leading ISO timestamp from a line.
func ParseTimestamp(line string) (time.Time, string, bool) {
	m := reGenericTimestamp.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, line, false
	}
	raw := m[1]
	rest := strings.TrimLeft(line[len(raw):], " -:")

	for _, layout := range []string{loginTimestampLayout, isoLayout, time.RFC3339Nano, time.RFC3339, simpleLayout} {
		t, err := time.Parse(layout, raw)
		if err == nil {
			return t.UTC(), rest, true
		}
	}
	return time.Time{}, line, false
}

// parseInt is a safe helper that returns 0 on failure.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// parseFloat is a safe helper that returns 0 on failure.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// trimQuotes removes leading/trailing single and double quotes.
func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	return s
}
