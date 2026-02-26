package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/geo"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/models"
	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// ActionsHandler handles container management and IP blocking endpoints.
type ActionsHandler struct {
	DB        *sql.DB
	Hub       *ws.Hub
	DockerCli *client.Client
	Geo       *geo.Locator

	failedMu     sync.Mutex
	failedCounts map[string]*failedAttempt
}

type failedAttempt struct {
	Count    int
	LastSeen time.Time
}

// NewActionsHandler creates an ActionsHandler with a Docker client.
func NewActionsHandler(db *sql.DB, hub *ws.Hub, geoLocator *geo.Locator) (*ActionsHandler, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client for actions: %w", err)
	}
	return &ActionsHandler{
		DB:           db,
		Hub:          hub,
		DockerCli:    cli,
		Geo:          geoLocator,
		failedCounts: make(map[string]*failedAttempt),
	}, nil
}

// ---------------------------------------------------------------------------
// Container actions
// ---------------------------------------------------------------------------

// ContainerStart handles POST /api/containers/{id}/start
func (h *ActionsHandler) ContainerStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, err := h.resolveContainerID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "container not found")
		return
	}
	if err := h.DockerCli.ContainerStart(r.Context(), fullID, container.StartOptions{}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to start container: "+err.Error())
		return
	}
	log.Printf("[actions] container started: %s", id)
	respondJSON(w, http.StatusOK, map[string]string{"status": "started", "container_id": id})
}

// ContainerStop handles POST /api/containers/{id}/stop
func (h *ActionsHandler) ContainerStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, err := h.resolveContainerID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "container not found")
		return
	}
	timeout := 30
	if err := h.DockerCli.ContainerStop(r.Context(), fullID, container.StopOptions{Timeout: &timeout}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to stop container: "+err.Error())
		return
	}
	log.Printf("[actions] container stopped: %s", id)
	respondJSON(w, http.StatusOK, map[string]string{"status": "stopped", "container_id": id})
}

// ContainerRestart handles POST /api/containers/{id}/restart
func (h *ActionsHandler) ContainerRestart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	fullID, err := h.resolveContainerID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "container not found")
		return
	}
	timeout := 30
	if err := h.DockerCli.ContainerRestart(r.Context(), fullID, container.StopOptions{Timeout: &timeout}); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to restart container: "+err.Error())
		return
	}
	log.Printf("[actions] container restarted: %s", id)
	respondJSON(w, http.StatusOK, map[string]string{"status": "restarted", "container_id": id})
}

func (h *ActionsHandler) resolveContainerID(ctx context.Context, shortID string) (string, error) {
	containers, err := h.DockerCli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	for _, c := range containers {
		if strings.HasPrefix(c.ID, shortID) {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("container %s not found", shortID)
}

// ---------------------------------------------------------------------------
// IP blocking
// ---------------------------------------------------------------------------

type blockIPRequest struct {
	IP     string `json:"ip"`
	Reason string `json:"reason"`
}

// BlockIP handles POST /api/banned-ips
func (h *ActionsHandler) BlockIP(w http.ResponseWriter, r *http.Request) {
	var req blockIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if net.ParseIP(req.IP) == nil {
		respondError(w, http.StatusBadRequest, "invalid IP address")
		return
	}

	if err := h.iptablesBlock(r.Context(), req.IP); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to block IP: "+err.Error())
		return
	}

	ban := &models.BannedIP{
		IP:       req.IP,
		Jail:     "dashboard",
		BannedAt: time.Now().UTC(),
		IsActive: true,
	}

	if h.Geo != nil {
		if result, err := h.Geo.Lookup(req.IP); err == nil && result != nil {
			ban.Country = result.Country
			ban.City = result.City
			ban.ISP = result.ISP
			ban.Lat = result.Lat
			ban.Lon = result.Lon
			ban.IsProxy = result.Proxy
		}
	}

	id, err := models.InsertBannedIP(h.DB, ban)
	if err != nil {
		log.Printf("[actions] ban in iptables but DB insert failed: %v", err)
	}
	ban.ID = id

	h.Hub.BroadcastMessage(&ws.Message{
		Type: "banned_ip_update",
		Data: map[string]interface{}{"action": "block", "ip": ban},
	})

	log.Printf("[actions] IP blocked: %s (reason: %s)", req.IP, req.Reason)
	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "blocked", "ip": ban})
}

// UnblockIP handles DELETE /api/banned-ips/{ip}
func (h *ActionsHandler) UnblockIP(w http.ResponseWriter, r *http.Request) {
	ip := chi.URLParam(r, "ip")
	if net.ParseIP(ip) == nil {
		respondError(w, http.StatusBadRequest, "invalid IP address")
		return
	}

	if err := h.iptablesUnblock(r.Context(), ip); err != nil {
		log.Printf("[actions] iptables unblock failed (may not exist): %v", err)
	}

	// Mark ALL active bans for this IP as unbanned (across all jails).
	for _, jail := range []string{"dashboard", "auto-block", "sshd"} {
		_ = models.UpdateBannedIP(h.DB, ip, jail, time.Now().UTC())
	}

	h.Hub.BroadcastMessage(&ws.Message{
		Type: "banned_ip_update",
		Data: map[string]interface{}{"action": "unblock", "ip": ip},
	})

	log.Printf("[actions] IP unblocked: %s", ip)
	respondJSON(w, http.StatusOK, map[string]string{"status": "unblocked", "ip": ip})
}

// runIptables runs an iptables command via nsenter into the host network
// namespace. Falls back to direct iptables if nsenter fails.
func (h *ActionsHandler) runIptables(ctx context.Context, args ...string) error {
	// Try nsenter first (requires pid:host + SYS_ADMIN)
	nsArgs := append([]string{"-t", "1", "-n", "--", "iptables"}, args...)
	cmd := exec.CommandContext(ctx, "nsenter", nsArgs...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	log.Printf("[actions] nsenter iptables failed (%v: %s), trying direct iptables", err, strings.TrimSpace(string(output)))

	// Fallback: direct iptables (works if network_mode=host or NET_ADMIN in host netns)
	cmd2 := exec.CommandContext(ctx, "iptables", args...)
	output2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf("iptables %v: nsenter: %s; direct: %s: %w", args, string(output), string(output2), err2)
	}
	return nil
}

// iptablesBlock blocks an IP using iptables via nsenter into host namespace.
func (h *ActionsHandler) iptablesBlock(ctx context.Context, ip string) error {
	return h.runIptables(ctx, "-I", "INPUT", "-s", ip, "-j", "DROP")
}

// iptablesUnblock removes an IP block.
func (h *ActionsHandler) iptablesUnblock(ctx context.Context, ip string) error {
	return h.runIptables(ctx, "-D", "INPUT", "-s", ip, "-j", "DROP")
}

// ---------------------------------------------------------------------------
// Auto-ban: track failed logins and block after 5 attempts
// ---------------------------------------------------------------------------

// TrackFailedLogin is called by the log watcher when a LOGIN_FAIL event is
// parsed. After 5 failures from the same IP within 1 hour, it auto-blocks.
func (h *ActionsHandler) TrackFailedLogin(ip string) {
	if net.ParseIP(ip) == nil || ip == "" {
		return
	}
	// Skip private IPs (internal Docker traffic, etc.)
	if isPrivateIP(ip) {
		return
	}

	h.failedMu.Lock()
	defer h.failedMu.Unlock()

	entry, exists := h.failedCounts[ip]
	if !exists {
		entry = &failedAttempt{}
		h.failedCounts[ip] = entry
	}

	if time.Since(entry.LastSeen) > time.Hour {
		entry.Count = 0
	}

	entry.Count++
	entry.LastSeen = time.Now()

	if entry.Count >= 5 {
		log.Printf("[actions] auto-blocking IP %s after %d failed attempts", ip, entry.Count)
		entry.Count = 0

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := h.iptablesBlock(ctx, ip); err != nil {
				log.Printf("[actions] auto-block iptables failed for %s: %v", ip, err)
				return
			}

			ban := &models.BannedIP{
				IP:       ip,
				Jail:     "auto-block",
				BannedAt: time.Now().UTC(),
				IsActive: true,
			}
			if h.Geo != nil {
				if result, err := h.Geo.Lookup(ip); err == nil && result != nil {
					ban.Country = result.Country
					ban.City = result.City
					ban.ISP = result.ISP
					ban.Lat = result.Lat
					ban.Lon = result.Lon
					ban.IsProxy = result.Proxy
				}
			}
			if _, err := models.InsertBannedIP(h.DB, ban); err != nil {
				log.Printf("[actions] auto-block DB insert failed for %s: %v", ip, err)
			}
			h.Hub.BroadcastMessage(&ws.Message{
				Type: "banned_ip_update",
				Data: map[string]interface{}{"action": "auto-block", "ip": ban},
			})
		}()
	}
}

// CleanupOldAttempts removes stale entries from the failed login tracker.
func (h *ActionsHandler) CleanupOldAttempts() {
	h.failedMu.Lock()
	defer h.failedMu.Unlock()
	cutoff := time.Now().Add(-2 * time.Hour)
	for ip, entry := range h.failedCounts {
		if entry.LastSeen.Before(cutoff) {
			delete(h.failedCounts, ip)
		}
	}
}

// isPrivateIP returns true for loopback, link-local, and RFC-1918 addresses.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return true
	}
	privateRanges := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
