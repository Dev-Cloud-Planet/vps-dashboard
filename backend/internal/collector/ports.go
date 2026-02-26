package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// portDef describes a well-known service port to monitor.
type portDef struct {
	Name string
	Port int
}

// PortStatus represents the TCP reachability of a single service port.
type PortStatus struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	Open bool   `json:"open"`
}

// defaultPorts is the list of ports checked on every cycle.
var defaultPorts = []portDef{
	{Name: "SSH", Port: 22},
	{Name: "HTTP", Port: 80},
	{Name: "HTTPS", Port: 443},
	{Name: "n8n", Port: 5678},
	{Name: "PostgreSQL", Port: 5432},
	{Name: "Coolify", Port: 8000},
	{Name: "Traefik", Port: 8080},
}

// PortChecker periodically TCP-dials a set of service ports and broadcasts
// their availability via WebSocket.
type PortChecker struct {
	db       *sql.DB
	hub      *ws.Hub
	interval time.Duration
	timeout  time.Duration
	ports    []portDef
}

// NewPortChecker creates a PortChecker with the default port list.
func NewPortChecker(db *sql.DB, hub *ws.Hub, interval time.Duration) *PortChecker {
	return &PortChecker{
		db:       db,
		hub:      hub,
		interval: interval,
		timeout:  2 * time.Second,
		ports:    defaultPorts,
	}
}

// Start begins the periodic port-checking loop. It blocks until ctx is
// cancelled.
func (pc *PortChecker) Start(ctx context.Context) {
	log.Printf("[ports] checker starting (interval=%s, ports=%d)", pc.interval, len(pc.ports))

	// Run immediately on start, then on every tick.
	pc.check()

	ticker := time.NewTicker(pc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[ports] checker stopped")
			return
		case <-ticker.C:
			pc.check()
		}
	}
}

// check performs a single round of TCP port checks.
func (pc *PortChecker) check() {
	results := make([]PortStatus, 0, len(pc.ports))

	for _, p := range pc.ports {
		open := tcpDial(p.Port, pc.timeout)
		results = append(results, PortStatus{
			Name: p.Name,
			Port: p.Port,
			Open: open,
		})
	}

	pc.hub.BroadcastMessage(&ws.Message{
		Type: "port_status",
		Data: results,
	})
}

// tcpDial attempts to open a TCP connection to localhost on the given port.
func tcpDial(port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// CheckPort performs a single ad-hoc check on a specific port.
// Useful for the API layer.
func CheckPort(port int) bool {
	return tcpDial(port, 2*time.Second)
}
