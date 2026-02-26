package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	Port string

	// Database
	DBPath string

	// Auth
	JWTSecret     string
	AdminUser     string
	AdminPassword string

	// Log file paths
	MonitorLogPath  string
	LoginLogPath    string
	AlertLogPath    string
	Fail2banLogPath string

	// Polling intervals
	MetricsInterval   time.Duration
	DockerInterval    time.Duration
	PortCheckInterval time.Duration

	// Startup behaviour
	BackfillOnStart bool

	// CORS
	CORSOrigins []string

	// Docker monitoring
	CriticalContainers []string
}

// Load reads configuration from environment variables, applying defaults for
// any value that is not explicitly set.
func Load() *Config {
	return &Config{
		Port:               envOrDefault("PORT", "8080"),
		DBPath:             envOrDefault("DB_PATH", "/data/dashboard.db"),
		JWTSecret:          envOrDefault("JWT_SECRET", ""),
		AdminUser:          envOrDefault("ADMIN_USER", "admin"),
		AdminPassword:      envOrDefault("ADMIN_PASSWORD", "changeme"),
		MonitorLogPath:     envOrDefault("MONITOR_LOG_PATH", ""),
		LoginLogPath:       envOrDefault("LOGIN_LOG_PATH", ""),
		AlertLogPath:       envOrDefault("ALERT_LOG_PATH", ""),
		Fail2banLogPath:    envOrDefault("FAIL2BAN_LOG_PATH", ""),
		MetricsInterval:    envOrDuration("METRICS_INTERVAL", 10*time.Second),
		DockerInterval:     envOrDuration("DOCKER_INTERVAL", 15*time.Second),
		PortCheckInterval:  envOrDuration("PORT_CHECK_INTERVAL", 60*time.Second),
		BackfillOnStart:    envOrBool("BACKFILL_ON_START", true),
		CORSOrigins:        envOrStringSlice("CORS_ORIGINS", []string{"*"}),
		CriticalContainers: envOrStringSlice("CRITICAL_CONTAINERS", nil),
	}
}

// envOrDefault returns the environment variable value or the provided fallback.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrDuration parses a Go duration string (e.g. "10s", "1m30s") from an
// environment variable. Returns the fallback on missing or invalid values.
func envOrDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

// envOrBool parses a boolean from an environment variable. Accepts "true",
// "1", "yes" as truthy and "false", "0", "no" as falsy. Returns the fallback
// when the variable is unset or cannot be parsed.
func envOrBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		switch strings.ToLower(v) {
		case "yes":
			return true
		case "no":
			return false
		default:
			return fallback
		}
	}
	return b
}

// envOrStringSlice splits a comma-separated environment variable into a slice
// of trimmed, non-empty strings. Returns the fallback when the variable is
// unset or empty.
func envOrStringSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
