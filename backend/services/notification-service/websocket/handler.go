package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// In production, this should be restricted to allowed origins
		return true
	},
}

// Handler handles WebSocket connections
type Handler struct {
	hub *Hub
	log zerolog.Logger
}

// SubscribeMessage represents a subscribe/unsubscribe request from client
type SubscribeMessage struct {
	Action  string `json:"action"`  // "subscribe" or "unsubscribe"
	Channel string `json:"channel"` // channel to subscribe/unsubscribe
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, log zerolog.Logger) *Handler {
	return &Handler{
		hub: hub,
		log: log,
	}
}

// HandleWebSocket handles WebSocket connection upgrade and management
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get optional correlation ID from request header
	correlationID := r.Header.Get("X-Correlation-ID")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error().Err(err).Str("correlation_id", correlationID).Msg("Failed to upgrade WebSocket")
		return
	}

	// Create new client
	client := &Client{
		hub:      h.hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}

	// Register the client
	h.hub.Register(client)

	h.log.Info().Str("correlation_id", correlationID).Str("remote", r.RemoteAddr).Msg("WebSocket client connected")

	// Check for initial subscriptions from query params
	// e.g., /ws?subscribe=build_logs:proj-123:build-456,project:proj-123
	if subs := r.URL.Query().Get("subscribe"); subs != "" {
		for _, channel := range splitChannels(subs) {
			h.hub.Subscribe(client, channel)
			// Send ACK for query param subscriptions too
			h.sendAck(client, "subscribed", channel)
		}
	}

	// Start goroutines for reading and writing
	go h.writePump(client, correlationID)
	go h.readPump(client, correlationID)
}

// readPump reads messages from the WebSocket connection
func (h *Handler) readPump(client *Client, correlationID string) {
	defer func() {
		h.hub.Unregister(client)
		client.conn.Close()
		h.log.Info().Str("correlation_id", correlationID).Msg("WebSocket client disconnected")
	}()

	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.Error().Err(err).Str("correlation_id", correlationID).Msg("WebSocket read error")
			}
			break
		}

		// Parse subscription message
		var subMsg SubscribeMessage
		if err := json.Unmarshal(message, &subMsg); err != nil {
			h.log.Warn().Err(err).Str("correlation_id", correlationID).Msg("Invalid WebSocket message format")
			continue
		}

		// Handle subscribe/unsubscribe
		switch subMsg.Action {
		case "subscribe":
			h.hub.Subscribe(client, subMsg.Channel)
			h.sendAck(client, "subscribed", subMsg.Channel)
		case "unsubscribe":
			h.hub.Unsubscribe(client, subMsg.Channel)
			h.sendAck(client, "unsubscribed", subMsg.Channel)
		default:
			h.log.Warn().Str("action", subMsg.Action).Str("correlation_id", correlationID).Msg("Unknown WebSocket action")
		}
	}
}

// writePump writes messages to the WebSocket connection
func (h *Handler) writePump(client *Client, correlationID string) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current WebSocket message
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendAck sends an acknowledgment message to the client
func (h *Handler) sendAck(client *Client, status, channel string) {
	ack := map[string]string{
		"type":    "ack",
		"status":  status,
		"channel": channel,
	}
	data, err := json.Marshal(ack)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	default:
		// Buffer full, skip ack
	}
}

// splitChannels splits comma-separated channel list
func splitChannels(s string) []string {
	if s == "" {
		return nil
	}
	var channels []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if start < i {
				channels = append(channels, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		channels = append(channels, s[start:])
	}
	return channels
}

