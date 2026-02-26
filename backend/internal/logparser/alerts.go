package logparser

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// Alert log line patterns.
var (
	// Alert sent: high_cpu
	reAlertSent = regexp.MustCompile(
		`Alert sent:\s*(\S+)`)

	// Alert FAILED (500): high_cpu - connection timeout
	reAlertFailed = regexp.MustCompile(
		`Alert FAILED\s*\((\d+)\):\s*(\S+)\s*-\s*(.+)$`)

	// Rate limited: high_cpu (120s ago)
	reRateLimited = regexp.MustCompile(
		`Rate limited:\s*(\S+)\s*\((\d+(?:\.\d+)?)s\s+ago\)`)
)

// AlertEvent represents a parsed line from alerts.log. This intermediate type
// is converted to models.Alert before database storage.
type AlertEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"` // ALERT_SENT, ALERT_FAILED, RATE_LIMITED
	Key        string    `json:"key"`
	Code       int       `json:"code,omitempty"`
	Details    string    `json:"details,omitempty"`
	SecondsAgo float64   `json:"seconds_ago,omitempty"`
	RawLine    string    `json:"raw_line,omitempty"`
}

// ParseAlertLine parses a single line from alerts.log and returns an
// AlertEvent. If the line does not match any known pattern, ok is false.
func ParseAlertLine(line string) (AlertEvent, bool) {
	ts, rest, hasTS := ParseTimestamp(line)
	if !hasTS {
		ts = time.Now().UTC()
		rest = line
	}

	evt := AlertEvent{
		Timestamp: ts,
		RawLine:   line,
	}

	if m := reAlertFailed.FindStringSubmatch(rest); m != nil {
		evt.Type = "ALERT_FAILED"
		evt.Code = parseInt(m[1])
		evt.Key = m[2]
		evt.Details = m[3]
		return evt, true
	}

	if m := reRateLimited.FindStringSubmatch(rest); m != nil {
		evt.Type = "RATE_LIMITED"
		evt.Key = m[1]
		evt.SecondsAgo = parseFloat(m[2])
		return evt, true
	}

	if m := reAlertSent.FindStringSubmatch(rest); m != nil {
		evt.Type = "ALERT_SENT"
		evt.Key = m[1]
		return evt, true
	}

	return AlertEvent{}, false
}

// ToAlert converts an AlertEvent to a models.Alert suitable for DB insertion.
func (ae AlertEvent) ToAlert() models.Alert {
	a := models.Alert{
		Timestamp: ae.Timestamp,
		Type:      ae.Type,
		AlertKey:  ae.Key,
		RawLine:   ae.RawLine,
	}

	switch ae.Type {
	case "ALERT_SENT":
		a.Status = "sent"
		a.Message = fmt.Sprintf("Alert sent for key: %s", ae.Key)
	case "ALERT_FAILED":
		a.Status = "failed"
		a.HTTPCode = ae.Code
		a.Details = ae.Details
		a.Message = fmt.Sprintf("Alert delivery failed (%d): %s - %s", ae.Code, ae.Key, ae.Details)
	case "RATE_LIMITED":
		a.Status = "rate_limited"
		a.Message = fmt.Sprintf("Alert rate limited: %s (%.0fs ago)", ae.Key, ae.SecondsAgo)
	}

	return a
}
