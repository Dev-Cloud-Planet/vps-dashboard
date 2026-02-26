package ws

import (
	"context"
	"log"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const (
	// sendBufferSize is the capacity of the per-client outgoing message
	// channel. If the channel fills up the client is considered slow and
	// will be disconnected.
	sendBufferSize = 256

	// writeTimeout is the maximum time allowed for writing a single
	// message to the underlying connection.
	writeTimeout = 10 * time.Second

	// pongWait is how long we wait for a pong before considering the
	// connection dead.
	pongWait = 60 * time.Second

	// pingInterval must be less than pongWait so that pings are sent
	// before the read deadline expires.
	pingInterval = 30 * time.Second
)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewClient creates a Client attached to the given Hub.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
	}
}

// ReadPump reads incoming messages from the WebSocket. It runs in its own
// goroutine per client. When the connection closes, the client is
// unregistered from the hub.
func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "read pump done")
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			// Any read error (including normal close) ends the pump.
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Printf("ws: read error: %v", err)
			}
			return
		}

		msg, err := DecodeMessage(data)
		if err != nil {
			log.Printf("ws: decode error: %v", err)
			continue
		}

		// For now we only log inbound messages. Specific message handling
		// (e.g. subscribe/unsubscribe topics) can be added here.
		_ = msg
	}
}

// WritePump sends queued messages to the WebSocket. It runs in its own
// goroutine per client and also handles periodic pings to keep the
// connection alive.
func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "write pump done")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel.
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			if err := wsjson.Write(writeCtx, c.conn, jsonRawHelper(msg)); err != nil {
				cancel()
				log.Printf("ws: write error: %v", err)
				return
			}
			cancel()

		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			if err := c.conn.Ping(pingCtx); err != nil {
				cancel()
				log.Printf("ws: ping error: %v", err)
				return
			}
			cancel()

		case <-ctx.Done():
			return
		}
	}
}

// jsonRawHelper wraps raw JSON bytes so that wsjson.Write does not
// double-encode the payload. It implements json.Marshaler.
type jsonRawHelper []byte

func (j jsonRawHelper) MarshalJSON() ([]byte, error) {
	return j, nil
}
