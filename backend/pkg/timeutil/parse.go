package timeutil

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Parse attempts to parse a timestamp string using multiple formats commonly
// found in the monitoring log files. It returns the first successful parse or
// an error if no format matches.
//
// Supported formats:
//   - "Thu Feb 26 17:00:00 UTC 2026"                                (monitor.log)
//   - "2026-02-26T17:00:00.000000+00:00 ubuntu sshd[1234]:"        (logins.log with systemd prefix)
//   - "2026-02-26T17:00:00.000000+00:00"                            (ISO-8601 with micro)
//   - "2026-02-26T17:00:00Z"                                        (RFC-3339)
//   - "2026-02-26 17:00:00"                                         (simple datetime)
//   - "Feb 26 17:00:00"                                             (short syslog; assumes current year)
func Parse(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("timeutil: empty string")
	}

	// Try each parser in order of specificity.
	for _, fn := range parsers {
		if t, ok := fn(raw); ok {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("timeutil: unrecognised timestamp format: %q", raw)
}

// parsers is the ordered list of parser functions tried by Parse.
var parsers = []func(string) (time.Time, bool){
	parseSystemdPrefix,
	parseLayout("Mon Jan _2 15:04:05 MST 2006"),        // monitor.log
	parseLayout("2006-01-02T15:04:05.000000-07:00"),     // ISO-8601 microseconds
	parseLayout("2006-01-02T15:04:05.000000+00:00"),     // same, explicit +00:00
	parseLayout(time.RFC3339Nano),                       // RFC-3339 nano
	parseLayout(time.RFC3339),                           // RFC-3339
	parseLayout("2006-01-02T15:04:05Z"),                 // RFC-3339 without offset
	parseLayout("2006-01-02 15:04:05"),                  // simple datetime
	parseShortSyslog,                                    // "Feb 26 17:00:00"
}

// parseLayout returns a parser function that tries time.Parse with the given
// Go reference layout.
func parseLayout(layout string) func(string) (time.Time, bool) {
	return func(raw string) (time.Time, bool) {
		t, err := time.Parse(layout, raw)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
}

// systemdPrefixRe matches an ISO timestamp at the start of a systemd journal
// line, e.g. "2026-02-26T17:00:00.000000+00:00 ubuntu sshd[1234]:".
var systemdPrefixRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}[+-]\d{2}:\d{2})\s`,
)

// parseSystemdPrefix extracts the leading ISO timestamp from a full systemd
// log line.
func parseSystemdPrefix(raw string) (time.Time, bool) {
	m := systemdPrefixRe.FindStringSubmatch(raw)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02T15:04:05.000000-07:00", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// shortSyslogRe matches "Mon DD HH:MM:SS" at the beginning of a line.
var shortSyslogRe = regexp.MustCompile(
	`^([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{2}:\d{2}:\d{2})`,
)

// parseShortSyslog handles the abbreviated syslog format "Feb 26 17:00:00".
// Since this format has no year, the current year is assumed.
func parseShortSyslog(raw string) (time.Time, bool) {
	m := shortSyslogRe.FindStringSubmatch(raw)
	if m == nil {
		return time.Time{}, false
	}
	// Reconstruct with current year.
	year := time.Now().UTC().Year()
	composed := fmt.Sprintf("%s %s %s %d", m[1], m[2], m[3], year)
	t, err := time.Parse("Jan 2 15:04:05 2006", composed)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
