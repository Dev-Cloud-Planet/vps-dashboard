package logparser

import (
	"regexp"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
)

// Login log line patterns.
// Timestamp format: [2026-02-26T17:27:06.661347+00:00 ubuntu sshd[3824249]:]
var (
	// LOGIN OK: user=root ip=1.2.3.4 method=publickey
	reLoginOK = regexp.MustCompile(
		`LOGIN OK:\s*user=(\S+)\s+ip=(\S+)\s+method=(\S+)`)

	// LOGIN FAIL: user=admin ip=5.6.7.8 attempts=3
	reLoginFail = regexp.MustCompile(
		`LOGIN FAIL:\s*user=(\S+)\s+ip=(\S+)\s+attempts=(\d+)`)

	// SESSION: user=root by=pam_unix  (covers opened/closed)
	reSession = regexp.MustCompile(
		`SESSION:\s*user=(\S+)\s+by=(\S+)`)

	// NEW USER: username
	reNewUser = regexp.MustCompile(
		`NEW USER:\s*(\S+)`)

	// USER DELETED: username
	reUserDeleted = regexp.MustCompile(
		`USER DELETED:\s*(\S+)`)

	// SUDO DANGER: user=deploy cmd=/bin/rm -rf /
	reSudoDanger = regexp.MustCompile(
		`SUDO DANGER:\s*user=(\S+)\s+cmd=(.+)$`)

	// Accepted publickey for root from 1.2.3.4 port 54321 ssh2
	reAccepted = regexp.MustCompile(
		`Accepted\s+(\S+)\s+for\s+(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)

	// Failed password for root from 1.2.3.4 port 54321 ssh2
	reFailed = regexp.MustCompile(
		`Failed\s+(\S+)\s+for\s+(?:invalid user\s+)?(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)

	// Ban 1.2.3.4  (fail2ban)
	reBan = regexp.MustCompile(
		`Ban\s+(\S+)`)

	// Unban 1.2.3.4  (fail2ban)
	reUnban = regexp.MustCompile(
		`Unban\s+(\S+)`)
)

// ParseLoginLine parses a single line from logins.log and returns a
// models.Login. If the line does not match any known pattern, ok is false.
func ParseLoginLine(line string) (models.Login, bool) {
	ts, rest, hasTS := ParseLoginTimestamp(line)
	if !hasTS {
		// Try generic timestamp as fallback.
		ts, rest, hasTS = ParseTimestamp(line)
		if !hasTS {
			ts = time.Now().UTC()
			rest = line
		}
	}

	evt := models.Login{
		Timestamp: ts,
		RawLine:   line,
	}

	// Our custom log markers first (these are written by the monitoring scripts).

	if m := reLoginOK.FindStringSubmatch(rest); m != nil {
		evt.EventType = "LOGIN_OK"
		evt.Username = m[1]
		evt.IP = m[2]
		evt.Method = m[3]
		return evt, true
	}

	if m := reLoginFail.FindStringSubmatch(rest); m != nil {
		evt.EventType = "LOGIN_FAIL"
		evt.Username = m[1]
		evt.IP = m[2]
		evt.Attempts = parseInt(m[3])
		return evt, true
	}

	if m := reSession.FindStringSubmatch(rest); m != nil {
		evt.EventType = "SESSION"
		evt.Username = m[1]
		evt.ByUser = m[2]
		return evt, true
	}

	if m := reNewUser.FindStringSubmatch(rest); m != nil {
		evt.EventType = "NEW_USER"
		evt.Username = m[1]
		return evt, true
	}

	if m := reUserDeleted.FindStringSubmatch(rest); m != nil {
		evt.EventType = "USER_DELETED"
		evt.Username = m[1]
		return evt, true
	}

	if m := reSudoDanger.FindStringSubmatch(rest); m != nil {
		evt.EventType = "SUDO_DANGER"
		evt.Username = m[1]
		evt.Command = trimQuotes(m[2])
		return evt, true
	}

	// Raw sshd log patterns (in case the line is an unprocessed auth.log line).

	if m := reAccepted.FindStringSubmatch(rest); m != nil {
		evt.EventType = "LOGIN_OK"
		evt.Method = m[1]
		evt.Username = m[2]
		evt.IP = m[3]
		return evt, true
	}

	if m := reFailed.FindStringSubmatch(rest); m != nil {
		evt.EventType = "LOGIN_FAIL"
		evt.Method = m[1]
		evt.Username = m[2]
		evt.IP = m[3]
		return evt, true
	}

	// fail2ban patterns.

	if m := reBan.FindStringSubmatch(rest); m != nil {
		evt.EventType = "BAN"
		evt.IP = m[1]
		return evt, true
	}

	if m := reUnban.FindStringSubmatch(rest); m != nil {
		evt.EventType = "UNBAN"
		evt.IP = m[1]
		return evt, true
	}

	return models.Login{}, false
}
