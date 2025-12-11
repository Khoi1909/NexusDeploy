package handlers

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// In production, restrict to allowed origins
		return true
	},
	// Enable compression
	EnableCompression: true,
}

// WebSocketProxy proxies WebSocket connections to notification service
type WebSocketProxy struct {
	notificationServiceURL string
	log                    zerolog.Logger
}

// NewWebSocketProxy creates a new WebSocket proxy
func NewWebSocketProxy(log zerolog.Logger) *WebSocketProxy {
	notificationURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationURL == "" {
		notificationURL = "http://notification-service:8080"
	}

	return &WebSocketProxy{
		notificationServiceURL: notificationURL,
		log:                    log,
	}
}

// HandleWebSocket proxies WebSocket connection to notification service
func (p *WebSocketProxy) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	p.log.Info().
		Str("remote", r.RemoteAddr).
		Str("path", r.URL.Path).
		Str("query", r.URL.RawQuery).
		Msg("WebSocket upgrade request received")

	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		p.log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}
	defer clientConn.Close()

	// Set pong handler for client connection to keep it alive
	clientConn.SetPongHandler(func(string) error {
		// Reset read deadline on pong
		return nil
	})

	p.log.Info().Msg("Client WebSocket connection upgraded successfully")

	// Parse notification service URL
	backendURL, err := url.Parse(p.notificationServiceURL)
	if err != nil {
		p.log.Error().Err(err).Msg("Invalid notification service URL")
		clientConn.WriteMessage(websocket.CloseMessage, []byte("Invalid backend URL"))
		return
	}

	// Build WebSocket URL for notification service
	// Convert http:// to ws:// and https:// to wss://
	wsScheme := "ws"
	if backendURL.Scheme == "https" {
		wsScheme = "wss"
	} else if backendURL.Scheme == "http" {
		wsScheme = "ws"
	}

	// If host doesn't include port and it's http, add default port 8080
	host := backendURL.Host
	if wsScheme == "ws" && !containsPort(host) {
		host = host + ":8080"
	}

	wsURL := url.URL{
		Scheme:   wsScheme,
		Host:     host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// Filter out WebSocket-specific headers before forwarding
	// gorilla/websocket will add these automatically
	backendHeaders := make(http.Header)
	for key, values := range r.Header {
		// Skip WebSocket upgrade headers - gorilla/websocket adds these automatically
		if strings.HasPrefix(strings.ToLower(key), "sec-websocket") ||
			strings.ToLower(key) == "connection" ||
			strings.ToLower(key) == "upgrade" {
			continue
		}
		backendHeaders[key] = values
	}

	// Connect to notification service
	p.log.Info().Str("url", wsURL.String()).Msg("Connecting to notification service")
	backendConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), backendHeaders)
	if err != nil {
		p.log.Error().Err(err).Str("url", wsURL.String()).Msg("Failed to connect to notification service")
		clientConn.WriteMessage(websocket.CloseMessage, []byte("Failed to connect to backend"))
		return
	}
	defer backendConn.Close()

	// Set pong handler for backend connection
	backendConn.SetPongHandler(func(string) error {
		return nil
	})

	p.log.Info().
		Str("remote", r.RemoteAddr).
		Str("path", r.URL.Path).
		Msg("WebSocket proxy connection established - starting message proxy")

	// Proxy messages in both directions
	errChan := make(chan error, 2)

	// Client -> Backend
	go func() {
		defer func() {
			p.log.Debug().Msg("Client->Backend proxy goroutine exiting")
		}()
		// Set read deadline to prevent hanging
		clientConn.SetReadDeadline(time.Now().Add(60 * time.Second))
		clientConn.SetPongHandler(func(string) error {
			clientConn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					p.log.Warn().Err(err).Msg("Client WebSocket read error")
				} else {
					p.log.Debug().Err(err).Msg("Client WebSocket closed normally")
				}
				errChan <- err
				return
			}
			// Reset read deadline after successful read
			clientConn.SetReadDeadline(time.Now().Add(60 * time.Second))
			// Handle ping/pong/close messages specially
			if messageType == websocket.PingMessage {
				if err := backendConn.WriteMessage(websocket.PingMessage, message); err != nil {
					p.log.Warn().Err(err).Msg("Failed to forward ping to backend")
					errChan <- err
					return
				}
				continue
			}
			if messageType == websocket.PongMessage {
				if err := backendConn.WriteMessage(websocket.PongMessage, message); err != nil {
					p.log.Warn().Err(err).Msg("Failed to forward pong to backend")
					errChan <- err
					return
				}
				continue
			}
			if messageType == websocket.CloseMessage {
				backendConn.WriteMessage(websocket.CloseMessage, message)
				errChan <- websocket.ErrCloseSent
				return
			}
			if err := backendConn.WriteMessage(messageType, message); err != nil {
				p.log.Warn().Err(err).Msg("Failed to write message to backend")
				errChan <- err
				return
			}
		}
	}()

	// Backend -> Client
	go func() {
		defer func() {
			p.log.Debug().Msg("Backend->Client proxy goroutine exiting")
		}()
		for {
			messageType, message, err := backendConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					p.log.Warn().Err(err).Msg("Backend WebSocket read error")
				} else {
					p.log.Debug().Err(err).Msg("Backend WebSocket closed normally")
				}
				errChan <- err
				return
			}
			// Handle ping/pong/close messages specially
			if messageType == websocket.PingMessage {
				if err := clientConn.WriteMessage(websocket.PingMessage, message); err != nil {
					p.log.Warn().Err(err).Msg("Failed to forward ping to client")
					errChan <- err
					return
				}
				continue
			}
			if messageType == websocket.PongMessage {
				if err := clientConn.WriteMessage(websocket.PongMessage, message); err != nil {
					p.log.Warn().Err(err).Msg("Failed to forward pong to client")
					errChan <- err
					return
				}
				continue
			}
			if messageType == websocket.CloseMessage {
				clientConn.WriteMessage(websocket.CloseMessage, message)
				errChan <- websocket.ErrCloseSent
				return
			}
			// Log text messages for debugging (ACK messages)
			if messageType == websocket.TextMessage {
				p.log.Debug().Str("message", string(message)).Msg("Forwarding message to client")
			}
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				p.log.Warn().Err(err).Msg("Failed to write message to client")
				errChan <- err
				return
			}
		}
	}()

	// Wait for error from either direction
	err = <-errChan
	if err != nil {
		// Log which direction failed
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			p.log.Info().Err(err).Msg("WebSocket proxy connection closed unexpectedly")
		} else if err == websocket.ErrCloseSent {
			p.log.Debug().Msg("WebSocket proxy connection closed normally")
		} else {
			p.log.Debug().Err(err).Msg("WebSocket proxy connection closed")
		}
	} else {
		p.log.Debug().Msg("WebSocket proxy connection ended without error")
	}
}

// containsPort checks if host string contains a port number
func containsPort(host string) bool {
	return strings.Contains(host, ":")
}
