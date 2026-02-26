package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// RouterConfig holds the dependencies needed to build the HTTP router.
type RouterConfig struct {
	DB             *sql.DB
	Hub            *ws.Hub
	JWTSecret      string
	AllowedOrigins []string
}

// NewRouter creates and returns a fully configured chi router with all
// middleware and API routes.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger)
	r.Use(CORSMiddleware(cfg.AllowedOrigins))

	// Instantiate handlers.
	authH := &AuthHandler{DB: cfg.DB, JWTSecret: cfg.JWTSecret}
	overviewH := &OverviewHandler{DB: cfg.DB}
	metricsH := &MetricsHandler{DB: cfg.DB}
	containersH := &ContainersHandler{DB: cfg.DB}
	loginsH := &LoginsHandler{DB: cfg.DB}
	alertsH := &AlertsHandler{DB: cfg.DB}
	settingsH := &SettingsHandler{DB: cfg.DB}
	wsH := &WebSocketHandler{Hub: cfg.Hub, JWTSecret: cfg.JWTSecret}

	r.Route("/api", func(r chi.Router) {
		// --- Public routes ---

		// Health check.
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{
				"status": "ok",
			})
		})

		// Auth.
		r.Post("/auth/login", authH.Login)

		// WebSocket (JWT validated inside handler from query param).
		r.Get("/ws", wsH.HandleWS)

		// --- Protected routes (require valid JWT) ---
		r.Group(func(r chi.Router) {
			r.Use(JWTAuth(cfg.JWTSecret))

			// Auth - password change.
			r.Put("/auth/password", authH.ChangePassword)

			// Overview.
			r.Get("/overview", overviewH.GetOverview)

			// System metrics.
			r.Get("/metrics/current", metricsH.GetCurrent)
			r.Get("/metrics/history", metricsH.GetHistory)

			// Containers.
			r.Get("/containers", containersH.List)
			r.Get("/containers/{id}", containersH.Get)
			r.Get("/containers/{id}/metrics", containersH.GetMetrics)

			// Login events.
			r.Get("/logins", loginsH.List)
			r.Get("/logins/stats", loginsH.GetStats)
			r.Get("/banned-ips", loginsH.GetBannedIPs)

			// Alerts.
			r.Get("/alerts", alertsH.List)
			r.Get("/alerts/stats", alertsH.GetStats)

			// Settings.
			r.Get("/settings", settingsH.Get)
			r.Put("/settings", settingsH.Update)
		})
	})

	return r
}
