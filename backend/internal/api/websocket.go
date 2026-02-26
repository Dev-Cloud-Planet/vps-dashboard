package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"nhooyr.io/websocket"

	"github.com/Dev-Cloud-Planet/vps-dashboard/backend/internal/ws"
)

// WebSocketHandler holds dependencies for the WebSocket upgrade endpoint.
type WebSocketHandler struct {
	Hub       *ws.Hub
	JWTSecret string
}

// HandleWS handles GET /api/ws.
// It validates the JWT from the "token" query parameter, upgrades the
// connection to WebSocket, registers the client with the hub, and starts
// the read/write pumps.
func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	// Validate JWT from query param.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		respondError(w, http.StatusUnauthorized, "missing token query parameter")
		return
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(h.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		respondError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	// Accept the WebSocket connection.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // CORS is handled by our middleware.
	})
	if err != nil {
		log.Printf("[ws] accept error: %v", err)
		return
	}

	// Create a client using the ws package's NewClient helper. This wires up
	// the hub, the connection, and the internal send channel.
	client := ws.NewClient(h.Hub, conn)

	// Register the client with the hub.
	h.Hub.Register(client)

	log.Printf("[ws] client connected (user=%s)", claims.Username)

	// Start the read and write pumps. ReadPump will handle unregistration
	// when the connection closes. Both pumps block, so we run one in a
	// goroutine and block on the other.
	ctx := r.Context()
	go client.WritePump(ctx)
	client.ReadPump(ctx) // blocks until disconnect
}

// HandleWSWithContext is identical to HandleWS but accepts an explicit context.
// Useful for testing.
func (h *WebSocketHandler) HandleWSWithContext(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	h.HandleWS(w, r.WithContext(ctx))
}
