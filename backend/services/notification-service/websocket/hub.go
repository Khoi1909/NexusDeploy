package websocket

import (
	"context"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
)

var (
	activeConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "notification_websocket_connections_active",
		Help: "Number of active WebSocket connections",
	})
	messagesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "notification_redis_messages_received_total",
		Help: "Total number of messages received from Redis",
	})
	messagesBroadcast = promauto.NewCounter(prometheus.CounterOpts{
		Name: "notification_websocket_messages_broadcast_total",
		Help: "Total number of messages broadcast to WebSocket clients",
	})
)

// Client represents a WebSocket connection
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	channels map[string]bool // channels this client is subscribed to
	mu       sync.RWMutex
}

// Hub manages WebSocket clients and message broadcasting
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Channel subscriptions: channel -> set of clients
	subscriptions map[string]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Subscribe client to channel
	subscribe chan *subscribeRequest

	// Unsubscribe client from channel
	unsubscribe chan *subscribeRequest

	// Broadcast message to channel
	broadcast chan *broadcastMessage

	log zerolog.Logger
	mu  sync.RWMutex
}

type subscribeRequest struct {
	client  *Client
	channel string
}

type broadcastMessage struct {
	channel string
	message []byte
}

// NewHub creates a new WebSocket hub
func NewHub(log zerolog.Logger) *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		subscriptions: make(map[string]map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		subscribe:     make(chan *subscribeRequest),
		unsubscribe:   make(chan *subscribeRequest),
		broadcast:     make(chan *broadcastMessage, 256),
		log:           log,
	}
}

// Run starts the hub event loop
func (h *Hub) Run(ctx context.Context) {
	h.log.Info().Msg("Starting WebSocket hub")

	for {
		select {
		case <-ctx.Done():
			h.log.Info().Msg("Stopping WebSocket hub")
			h.closeAllClients()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case req := <-h.subscribe:
			h.subscribeClient(req.client, req.channel)

		case req := <-h.unsubscribe:
			h.unsubscribeClient(req.client, req.channel)

		case msg := <-h.broadcast:
			h.broadcastMessage(msg.channel, msg.message)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	activeConnections.Inc()
	h.log.Debug().Msg("Client registered")
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)

		// Remove from all subscriptions
		client.mu.RLock()
		for channel := range client.channels {
			if subs, ok := h.subscriptions[channel]; ok {
				delete(subs, client)
				if len(subs) == 0 {
					delete(h.subscriptions, channel)
				}
			}
		}
		client.mu.RUnlock()

		close(client.send)
		activeConnections.Dec()
		h.log.Debug().Msg("Client unregistered")
	}
}

func (h *Hub) subscribeClient(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add client to channel subscription
	if _, ok := h.subscriptions[channel]; !ok {
		h.subscriptions[channel] = make(map[*Client]bool)
	}
	h.subscriptions[channel][client] = true

	// Track subscription on client side
	client.mu.Lock()
	client.channels[channel] = true
	client.mu.Unlock()

	h.log.Debug().Str("channel", channel).Msg("Client subscribed to channel")
}

func (h *Hub) unsubscribeClient(client *Client, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscriptions[channel]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.subscriptions, channel)
		}
	}

	client.mu.Lock()
	delete(client.channels, channel)
	client.mu.Unlock()

	h.log.Debug().Str("channel", channel).Msg("Client unsubscribed from channel")
}

func (h *Hub) broadcastMessage(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	messagesReceived.Inc()

	subs, ok := h.subscriptions[channel]
	if !ok {
		return
	}

	for client := range subs {
		select {
		case client.send <- message:
			messagesBroadcast.Inc()
		default:
			// Client send buffer full, skip
			h.log.Warn().Msg("Client send buffer full, skipping message")
		}
	}
}

func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}
	h.subscriptions = make(map[string]map[*Client]bool)
}

// Broadcast implements MessageBroadcaster interface
func (h *Hub) Broadcast(channel string, message []byte) {
	h.broadcast <- &broadcastMessage{
		channel: channel,
		message: message,
	}
}

// Register adds a new client
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe subscribes a client to a channel
func (h *Hub) Subscribe(client *Client, channel string) {
	h.subscribe <- &subscribeRequest{
		client:  client,
		channel: channel,
	}
}

// Unsubscribe removes a client from a channel
func (h *Hub) Unsubscribe(client *Client, channel string) {
	h.unsubscribe <- &subscribeRequest{
		client:  client,
		channel: channel,
	}
}

// GetActiveConnections returns the number of active connections
func (h *Hub) GetActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetChannelSubscribers returns the number of subscribers for a channel
func (h *Hub) GetChannelSubscribers(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if subs, ok := h.subscriptions[channel]; ok {
		return len(subs)
	}
	return 0
}

