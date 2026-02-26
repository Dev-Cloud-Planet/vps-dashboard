package ws

import (
	"context"
	"log"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to all of
// them using a fan-out pattern.
type Hub struct {
	// clients is the set of currently-registered clients.
	clients map[*Client]struct{}

	// broadcast is a channel for messages that should be sent to every
	// connected client.
	broadcast chan []byte

	// register is a channel for new client connections.
	register chan *Client

	// unregister is a channel for client disconnections.
	unregister chan *Client

	mu sync.RWMutex
}

// NewHub creates and returns a new Hub ready to be started with Run.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop. It blocks until ctx is cancelled, so it
// should be called in a dedicated goroutine.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
			log.Printf("ws: client connected (total=%d)", h.ClientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("ws: client disconnected (total=%d)", h.ClientCount())

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Slow client -- drop the connection.
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()

		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Broadcast sends a pre-encoded JSON message to every connected client.
func (h *Hub) Broadcast(data []byte) {
	h.broadcast <- data
}

// BroadcastMessage encodes a Message and broadcasts it. This is a convenience
// wrapper around Broadcast.
func (h *Hub) BroadcastMessage(msg *Message) {
	data, err := msg.Encode()
	if err != nil {
		log.Printf("ws: encode broadcast message: %v", err)
		return
	}
	h.Broadcast(data)
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
