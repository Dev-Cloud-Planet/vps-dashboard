package logparser

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nxadm/tail"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// LogPaths holds the filesystem paths to each monitored log file.
type LogPaths struct {
	MonitorLog string
	LoginsLog  string
	AlertsLog  string
}

// DefaultLogPaths returns the standard log file locations.
func DefaultLogPaths() LogPaths {
	return LogPaths{
		MonitorLog: "/var/log/vps-monitor/monitor.log",
		LoginsLog:  "/var/log/vps-monitor/logins.log",
		AlertsLog:  "/var/log/vps-monitor/alerts.log",
	}
}

// Watcher tails log files, parses each line and stores the resulting events
// in the database while broadcasting them over WebSocket.
type Watcher struct {
	db            *sql.DB
	hub           *ws.Hub
	paths         LogPaths
	OnFailedLogin func(ip string) // called on LOGIN_FAIL events for auto-ban
}

// NewWatcher creates a new Watcher.
func NewWatcher(db *sql.DB, hub *ws.Hub, paths LogPaths) *Watcher {
	return &Watcher{
		db:    db,
		hub:   hub,
		paths: paths,
	}
}

// WatchAll starts a goroutine for each log file and blocks until ctx is
// cancelled. All errors are logged rather than returned so that one failing
// watcher does not bring down the others.
func (w *Watcher) WatchAll(ctx context.Context) {
	go w.watchFile(ctx, w.paths.MonitorLog, "monitor")
	go w.watchFile(ctx, w.paths.LoginsLog, "logins")
	go w.watchFile(ctx, w.paths.AlertsLog, "alerts")

	log.Println("[watcher] all log watchers started")
	<-ctx.Done()
	log.Println("[watcher] all log watchers stopping")
}

// watchFile tails a single log file and dispatches each new line to the
// appropriate parser.
func (w *Watcher) watchFile(ctx context.Context, path, kind string) {
	if path == "" {
		log.Printf("[watcher:%s] no path configured, skipping", kind)
		return
	}

	// Wait for the file to appear if it doesn't exist yet.
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		log.Printf("[watcher:%s] waiting for %s to appear...", kind, path)
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}

	cfg := tail.Config{
		Follow:    true,
		ReOpen:    true, // Handle log rotation.
		MustExist: false,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // Start at end of file.
		Logger:    tail.DiscardingLogger,
	}

	t, err := tail.TailFile(path, cfg)
	if err != nil {
		log.Printf("[watcher:%s] failed to tail %s: %v", kind, path, err)
		return
	}

	log.Printf("[watcher:%s] tailing %s", kind, path)

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			t.Cleanup()
			return
		case line, ok := <-t.Lines:
			if !ok {
				log.Printf("[watcher:%s] tail channel closed", kind)
				return
			}
			if line.Err != nil {
				log.Printf("[watcher:%s] tail error: %v", kind, line.Err)
				continue
			}
			if line.Text == "" {
				continue
			}
			w.processLine(kind, line.Text)
		}
	}
}

// processLine routes a log line to the correct parser and persists the result.
func (w *Watcher) processLine(kind, line string) {
	switch kind {
	case "monitor":
		evt, ok := ParseMonitorLine(line)
		if !ok {
			return
		}
		w.handleMonitorEvent(evt)

	case "logins":
		evt, ok := ParseLoginLine(line)
		if !ok {
			return
		}
		w.handleLoginEvent(evt)

	case "alerts":
		evt, ok := ParseAlertLine(line)
		if !ok {
			return
		}
		w.handleAlertEvent(evt)
	}
}

// handleMonitorEvent broadcasts a monitor event. Noteworthy events (HIGH_*,
// *_DOWN) are also stored as alerts.
func (w *Watcher) handleMonitorEvent(evt MonitorEvent) {
	w.hub.BroadcastMessage(&ws.Message{
		Type: "monitor_event",
		Data: evt,
	})

	// Convert noteworthy monitor events into alerts.
	switch evt.Type {
	case "HIGH_CPU", "HIGH_RAM", "HIGH_DISK":
		pct, _ := evt.Data["percent"].(float64)
		resource, _ := evt.Data["resource"].(string)
		alert := &models.Alert{
			Timestamp: evt.Timestamp,
			Type:      evt.Type,
			AlertKey:  fmt.Sprintf("high_%s", resource),
			Message:   fmt.Sprintf("%s at %.0f%%", resource, pct),
			Status:    "active",
		}
		if _, err := models.InsertAlert(w.db, alert); err != nil {
			log.Printf("[watcher:monitor] insert alert: %v", err)
		}
		w.hub.BroadcastMessage(&ws.Message{Type: "alert", Data: alert})

	case "CONTAINER_DOWN":
		name, _ := evt.Data["name"].(string)
		alert := &models.Alert{
			Timestamp: evt.Timestamp,
			Type:      "CONTAINER_DOWN",
			AlertKey:  "container_down_" + name,
			Message:   "Container down: " + name,
			Status:    "active",
		}
		if _, err := models.InsertAlert(w.db, alert); err != nil {
			log.Printf("[watcher:monitor] insert alert: %v", err)
		}
		w.hub.BroadcastMessage(&ws.Message{Type: "alert", Data: alert})

	case "PORT_DOWN":
		svc, _ := evt.Data["service"].(string)
		alert := &models.Alert{
			Timestamp: evt.Timestamp,
			Type:      "PORT_DOWN",
			AlertKey:  "port_down_" + svc,
			Message:   "Port down: " + svc,
			Status:    "active",
		}
		if _, err := models.InsertAlert(w.db, alert); err != nil {
			log.Printf("[watcher:monitor] insert alert: %v", err)
		}
		w.hub.BroadcastMessage(&ws.Message{Type: "alert", Data: alert})
	}
}

// handleLoginEvent persists a login event and broadcasts it.
func (w *Watcher) handleLoginEvent(evt models.Login) {
	if _, err := models.InsertLogin(w.db, &evt); err != nil {
		log.Printf("[watcher:logins] insert: %v", err)
	}
	w.hub.BroadcastMessage(&ws.Message{
		Type: "login_event",
		Data: evt,
	})

	// Track failed logins for auto-ban.
	if evt.EventType == "LOGIN_FAIL" && evt.IP != "" && w.OnFailedLogin != nil {
		w.OnFailedLogin(evt.IP)
	}
}

// handleAlertEvent persists an alert event and broadcasts it.
func (w *Watcher) handleAlertEvent(evt AlertEvent) {
	alert := evt.ToAlert()
	if _, err := models.InsertAlert(w.db, &alert); err != nil {
		log.Printf("[watcher:alerts] insert: %v", err)
	}
	w.hub.BroadcastMessage(&ws.Message{
		Type: "alert_event",
		Data: evt,
	})
}

// ---------------------------------------------------------------------------
// Backfill: read existing log files from the beginning on startup.
// ---------------------------------------------------------------------------

// BackfillAll reads each log file from the beginning and inserts all parsed
// events into the database. This is intended to be called once at startup to
// populate the DB with historical data.
func (w *Watcher) BackfillAll() {
	w.backfillFile(w.paths.MonitorLog, "monitor")
	w.backfillFile(w.paths.LoginsLog, "logins")
	w.backfillFile(w.paths.AlertsLog, "alerts")
}

func (w *Watcher) backfillFile(path, kind string) {
	if path == "" {
		log.Printf("[backfill:%s] no path configured, skipping", kind)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf("[backfill:%s] cannot open %s: %v", kind, path, err)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		switch kind {
		case "monitor":
			if evt, ok := ParseMonitorLine(line); ok {
				w.handleMonitorEvent(evt)
				count++
			}
		case "logins":
			if evt, ok := ParseLoginLine(line); ok {
				w.handleLoginEvent(evt)
				count++
			}
		case "alerts":
			if evt, ok := ParseAlertLine(line); ok {
				w.handleAlertEvent(evt)
				count++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[backfill:%s] scan error: %v", kind, err)
	}
	log.Printf("[backfill:%s] imported %d events from %s", kind, count, path)
}
