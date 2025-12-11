package pubsub

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// MessageBroadcaster interface for broadcasting messages to clients
type MessageBroadcaster interface {
	Broadcast(channel string, message []byte)
}

// Consumer subscribes to Redis Pub/Sub channels and forwards messages
type Consumer struct {
	redis       *redis.Client
	broadcaster MessageBroadcaster
	log         zerolog.Logger
	channels    []string
}

// LogMessage represents a log entry from build/deployment
type LogMessage struct {
	Type      string `json:"type"` // "build_log", "deployment_log", "event"
	ProjectID string `json:"project_id"`
	BuildID   string `json:"build_id,omitempty"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level,omitempty"` // "info", "error", "warn"
}

// NewConsumer creates a new Redis Pub/Sub consumer
func NewConsumer(client *redis.Client, broadcaster MessageBroadcaster, log zerolog.Logger) *Consumer {
	return &Consumer{
		redis:       client,
		broadcaster: broadcaster,
		log:         log,
		channels: []string{
			"build_logs:*",      // Build logs (pattern subscription)
			"deployment_logs:*", // Deployment logs (pattern subscription)
			"events:*",          // General events (pattern subscription)
		},
	}
}

// Start begins listening to Redis Pub/Sub channels
func (c *Consumer) Start(ctx context.Context) {
	c.log.Info().Strs("patterns", c.channels).Msg("Starting Redis Pub/Sub consumer")

	// Use pattern subscription for wildcard channels
	pubsub := c.redis.PSubscribe(ctx, c.channels...)
	defer pubsub.Close()

	// Wait for subscription confirmation
	_, err := pubsub.Receive(ctx)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to subscribe to channels")
		return
	}

	c.log.Info().Msg("Successfully subscribed to Redis channels")

	// Listen for messages
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			c.log.Info().Msg("Stopping Redis Pub/Sub consumer")
			return
		case msg := <-ch:
			c.handleMessage(msg)
		}
	}
}

func (c *Consumer) handleMessage(msg *redis.Message) {
	if msg == nil {
		return
	}

	c.log.Debug().
		Str("channel", msg.Channel).
		Str("pattern", msg.Pattern).
		Msg("Received message from Redis")

	// Parse the channel to extract project/build ID
	// Format: build_logs:<project_id>:<build_id> or deployment_logs:<project_id>
	channel := msg.Channel

	// Try to parse as LogMessage
	var logMsg LogMessage
	var rawPayload map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Payload), &rawPayload); err != nil {
		// If not JSON, wrap as raw message
		logMsg = LogMessage{
			Type:    getMessageType(channel),
			Message: msg.Payload,
		}
	} else {
		// Map runner service format to notification service format
		// Runner service uses "line" field, notification service uses "message"
		if line, ok := rawPayload["line"].(string); ok && line != "" {
			logMsg.Message = line
		} else if message, ok := rawPayload["message"].(string); ok {
			logMsg.Message = message
		} else {
			logMsg.Message = msg.Payload
		}

		// Copy other fields
		if buildID, ok := rawPayload["build_id"].(string); ok {
			logMsg.BuildID = buildID
		}
		if timestamp, ok := rawPayload["timestamp"].(string); ok {
			logMsg.Timestamp = timestamp
		} else if ts, ok := rawPayload["timestamp"]; ok {
			// Handle time.Time format
			if tsBytes, err := json.Marshal(ts); err == nil {
				logMsg.Timestamp = string(tsBytes)
			}
		}
		if level, ok := rawPayload["level"].(string); ok {
			logMsg.Level = level
		}
	}

	// Extract IDs from channel if not in payload
	if logMsg.ProjectID == "" {
		logMsg.ProjectID = extractProjectID(channel)
	}
	if logMsg.BuildID == "" {
		logMsg.BuildID = extractBuildID(channel)
	}
	if logMsg.Type == "" {
		logMsg.Type = getMessageType(channel)
	}

	// Marshal the enriched message
	enrichedPayload, err := json.Marshal(logMsg)
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to marshal message")
		return
	}

	// Broadcast to WebSocket clients subscribed to this channel
	c.broadcaster.Broadcast(channel, enrichedPayload)

	// Also broadcast to project-specific channel for clients watching all project events
	if logMsg.ProjectID != "" {
		projectChannel := "project:" + logMsg.ProjectID
		c.broadcaster.Broadcast(projectChannel, enrichedPayload)
	}
}

func getMessageType(channel string) string {
	if strings.HasPrefix(channel, "build_logs:") {
		return "build_log"
	}
	if strings.HasPrefix(channel, "deployment_logs:") {
		return "deployment_log"
	}
	if strings.HasPrefix(channel, "events:") {
		return "event"
	}
	return "unknown"
}

func extractProjectID(channel string) string {
	parts := strings.Split(channel, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func extractBuildID(channel string) string {
	parts := strings.Split(channel, ":")
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
