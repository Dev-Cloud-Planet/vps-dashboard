package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/api"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/collector"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/config"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/database"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/geo"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/logparser"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[main] VPS Dashboard backend starting...")

	// -----------------------------------------------------------------------
	// 1. Load configuration from environment variables.
	// -----------------------------------------------------------------------
	cfg := config.Load()
	if cfg.JWTSecret == "" {
		log.Println("[main] WARNING: JWT_SECRET is not set; using insecure default")
		cfg.JWTSecret = "insecure-default-change-me"
	}

	// -----------------------------------------------------------------------
	// 2. Open database and run migrations.
	// -----------------------------------------------------------------------
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("[main] failed to open database: %v", err)
	}
	defer db.Close()

	// Run schema migrations (creates tables, indexes, seeds settings, creates
	// the default admin user).
	if err := database.Migrate(db, cfg.AdminUser, cfg.AdminPassword); err != nil {
		log.Fatalf("[main] migration failed: %v", err)
	}

	log.Println("[main] database ready")

	// -----------------------------------------------------------------------
	// 3. Ensure default admin user exists (idempotent).
	// -----------------------------------------------------------------------
	ensureAdminUser(db, cfg.AdminUser, cfg.AdminPassword)

	// -----------------------------------------------------------------------
	// 4. Initialise WebSocket hub and start its event loop.
	// -----------------------------------------------------------------------
	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)
	log.Println("[main] websocket hub started")

	// -----------------------------------------------------------------------
	// 5. Start system metrics collector.
	// -----------------------------------------------------------------------
	sysColl := collector.NewSystemCollector(db, hub, cfg.MetricsInterval)
	go sysColl.Start(ctx)

	// -----------------------------------------------------------------------
	// 6. Start Docker container collector.
	// -----------------------------------------------------------------------
	dockerColl, err := collector.NewDockerCollector(db, hub, cfg.DockerInterval, cfg.CriticalContainers)
	if err != nil {
		log.Printf("[main] WARNING: docker collector not available: %v", err)
	} else {
		go dockerColl.Start(ctx)
	}

	// -----------------------------------------------------------------------
	// 7. Start port checker.
	// -----------------------------------------------------------------------
	portChecker := collector.NewPortChecker(db, hub, cfg.PortCheckInterval)
	go portChecker.Start(ctx)

	// -----------------------------------------------------------------------
	// 8. Start log file watchers.
	// -----------------------------------------------------------------------
	logPaths := logparser.LogPaths{
		MonitorLog: cfg.MonitorLogPath,
		LoginsLog:  cfg.LoginLogPath,
		AlertsLog:  cfg.AlertLogPath,
	}

	// Use defaults if paths are not configured.
	if logPaths.MonitorLog == "" && logPaths.LoginsLog == "" && logPaths.AlertsLog == "" {
		logPaths = logparser.DefaultLogPaths()
	}

	watcher := logparser.NewWatcher(db, hub, logPaths)

	// Optionally backfill existing log data on first run.
	if cfg.BackfillOnStart {
		go func() {
			log.Println("[main] starting backfill...")
			watcher.BackfillAll()
			log.Println("[main] backfill complete")
		}()
	}

	go watcher.WatchAll(ctx)

	// -----------------------------------------------------------------------
	// 9. Start data retention cleanup ticker (runs daily).
	// -----------------------------------------------------------------------
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Run once on startup after a short delay.
		time.Sleep(30 * time.Second)
		runDataCleanup(db)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runDataCleanup(db)
			}
		}
	}()

	// -----------------------------------------------------------------------
	// 10. Create actions handler (container mgmt + IP blocking + auto-ban).
	// -----------------------------------------------------------------------
	geoLocator := geo.NewLocator()
	var actionsH *api.ActionsHandler
	actionsH, err = api.NewActionsHandler(db, hub, geoLocator)
	if err != nil {
		log.Printf("[main] WARNING: actions handler not available: %v", err)
	} else {
		watcher.OnFailedLogin = actionsH.TrackFailedLogin

		// Periodic cleanup of stale failed-attempt entries.
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					actionsH.CleanupOldAttempts()
				}
			}
		}()
		log.Println("[main] actions handler ready (container mgmt + IP blocking)")
	}

	// -----------------------------------------------------------------------
	// 11. Build and start HTTP server.
	// -----------------------------------------------------------------------
	router := api.NewRouter(api.RouterConfig{
		DB:             db,
		Hub:            hub,
		JWTSecret:      cfg.JWTSecret,
		AllowedOrigins: cfg.CORSOrigins,
		Actions:        actionsH,
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine.
	go func() {
		log.Printf("[main] HTTP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	// -----------------------------------------------------------------------
	// 11. Graceful shutdown on SIGINT / SIGTERM.
	// -----------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[main] received signal %s, shutting down...", sig)

	// Cancel all background goroutines.
	cancel()

	// Give the HTTP server a deadline to finish active requests.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[main] server shutdown error: %v", err)
	}

	log.Println("[main] server stopped gracefully")
}

// ensureAdminUser creates the default admin user if it does not already exist.
func ensureAdminUser(db *sql.DB, username, password string) {
	if username == "" || password == "" {
		return
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count); err != nil {
		log.Printf("[main] check admin user: %v", err)
		return
	}
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[main] hash admin password: %v", err)
		return
	}

	_, err = db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, string(hash), time.Now().UTC(),
	)
	if err != nil {
		log.Printf("[main] create admin user: %v", err)
		return
	}
	log.Printf("[main] default admin user '%s' created", username)
}

// runDataCleanup removes old metrics, logins, and alerts beyond the retention
// period. Retention is read from the settings table or defaults to 30/90/90
// days.
func runDataCleanup(db *sql.DB) {
	metricsDays := getSettingInt(db, "metrics_retention_days", 30)
	loginsDays := getSettingInt(db, "logins_retention_days", 90)
	alertsDays := getSettingInt(db, "alerts_retention_days", 90)

	log.Printf("[cleanup] pruning data older than metrics=%dd logins=%dd alerts=%dd",
		metricsDays, loginsDays, alertsDays)

	pruneTable(db, "metrics", "timestamp", metricsDays)
	pruneTable(db, "container_metrics", "timestamp", metricsDays)
	pruneTable(db, "logins", "timestamp", loginsDays)
	pruneTable(db, "alerts", "timestamp", alertsDays)

	log.Println("[cleanup] data cleanup complete")
}

// pruneTable deletes rows older than the given number of days from the
// specified table, using the given timestamp column.
func pruneTable(db *sql.DB, table, tsCol string, days int) {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s < datetime('now', '-%d days')", table, tsCol, days)
	res, err := db.Exec(q)
	if err != nil {
		log.Printf("[cleanup] prune %s: %v", table, err)
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		log.Printf("[cleanup] pruned %d rows from %s", n, table)
	}
}

// getSettingInt reads an integer setting from the database, returning the
// default value if the key is missing or not a valid integer.
func getSettingInt(db *sql.DB, key string, defaultVal int) int {
	var val string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	n := 0
	for _, ch := range val {
		if ch < '0' || ch > '9' {
			return defaultVal
		}
		n = n*10 + int(ch-'0')
	}
	if n <= 0 {
		return defaultVal
	}
	return n
}
