package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	// LogChannelPrefix is the prefix for build log channels
	LogChannelPrefix = "build_logs:"

	// EventChannelPrefix is the prefix for build event channels
	EventChannelPrefix = "build_events:"
)

// LogMessage represents a log message published to Redis
type LogMessage struct {
	BuildID   string    `json:"build_id"`
	Timestamp time.Time `json:"timestamp"`
	Line      string    `json:"line"`
	Level     string    `json:"level"` // info, warn, error
}

// EventMessage represents a build event published to Redis
type EventMessage struct {
	BuildID   string    `json:"build_id"`
	Event     string    `json:"event"` // started, step_complete, completed, failed
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Publisher handles publishing logs and events to Redis Pub/Sub
type Publisher struct {
	client *redis.Client
	log    zerolog.Logger
}

// PublisherConfig holds configuration for the publisher
type PublisherConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
}

// NewPublisher creates a new Redis Pub/Sub publisher
func NewPublisher(cfg PublisherConfig, log zerolog.Logger) (*Publisher, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	log.Info().Str("redis", cfg.RedisAddr).Msg("Redis Pub/Sub publisher initialized")

	return &Publisher{
		client: client,
		log:    log,
	}, nil
}

// Close closes the Redis connection
func (p *Publisher) Close() error {
	return p.client.Close()
}

// PublishLog publishes a log line for a build
func (p *Publisher) PublishLog(ctx context.Context, buildID, line string) error {
	return p.PublishLogWithLevel(ctx, buildID, line, "info")
}

// PublishLogWithLevel publishes a log line with a specific level
func (p *Publisher) PublishLogWithLevel(ctx context.Context, buildID, line, level string) error {
	return p.PublishLogWithProject(ctx, "", buildID, line, level)
}

// PublishLogWithProject publishes a log line with project ID (format: build_logs:projectId:buildId)
func (p *Publisher) PublishLogWithProject(ctx context.Context, projectID, buildID, line, level string) error {
	msg := LogMessage{
		BuildID:   buildID,
		Timestamp: time.Now(),
		Line:      line,
		Level:     level,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal log message: %w", err)
	}

	// Use format: build_logs:projectId:buildId if projectID provided, otherwise build_logs:buildId
	var channel string
	if projectID != "" {
		channel = LogChannelPrefix + projectID + ":" + buildID
	} else {
		channel = LogChannelPrefix + buildID
	}

	if err := p.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("publish log: %w", err)
	}

	return nil
}

// PublishEvent publishes a build event
func (p *Publisher) PublishEvent(ctx context.Context, event EventMessage) error {
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event message: %w", err)
	}

	channel := EventChannelPrefix + event.BuildID
	if err := p.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	p.log.Debug().
		Str("build_id", event.BuildID).
		Str("event", event.Event).
		Msg("Published build event")

	return nil
}

// PublishBuildStarted publishes a build started event
func (p *Publisher) PublishBuildStarted(ctx context.Context, buildID string) error {
	return p.PublishEvent(ctx, EventMessage{
		BuildID: buildID,
		Event:   "started",
		Message: "Build started",
	})
}

// PublishStepComplete publishes a step completion event
func (p *Publisher) PublishStepComplete(ctx context.Context, buildID, stepName, status string) error {
	return p.PublishEvent(ctx, EventMessage{
		BuildID: buildID,
		Event:   "step_complete",
		Status:  status,
		Message: fmt.Sprintf("Step '%s' completed with status: %s", stepName, status),
	})
}

// PublishBuildCompleted publishes a build completed event
func (p *Publisher) PublishBuildCompleted(ctx context.Context, buildID, status, message string) error {
	return p.PublishEvent(ctx, EventMessage{
		BuildID: buildID,
		Event:   "completed",
		Status:  status,
		Message: message,
	})
}

// LogCollector collects logs and publishes them in batches
type LogCollector struct {
	publisher  *Publisher
	projectID  string
	buildID    string
	logs       []string
	batchSize  int
	appendFunc func(ctx context.Context, buildID string, logs []string) error // Callback to save logs to database
}

// NewLogCollector creates a log collector for a build
func NewLogCollector(publisher *Publisher, buildID string, batchSize int) *LogCollector {
	return NewLogCollectorWithProject(publisher, "", buildID, batchSize, nil)
}

// NewLogCollectorWithProject creates a log collector for a build with project ID
func NewLogCollectorWithProject(publisher *Publisher, projectID, buildID string, batchSize int, appendFunc func(ctx context.Context, buildID string, logs []string) error) *LogCollector {
	if batchSize <= 0 {
		batchSize = 10
	}
	return &LogCollector{
		publisher:  publisher,
		projectID:  projectID,
		buildID:    buildID,
		logs:       make([]string, 0, batchSize),
		batchSize:  batchSize,
		appendFunc: appendFunc,
	}
}

// sanitizeUTF8 removes invalid UTF-8 sequences from a string
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Replace invalid UTF-8 with replacement character
	var b strings.Builder
	for i, r := range s {
		if r == utf8.RuneError {
			_, size := utf8.DecodeRuneInString(s[i:])
			if size == 1 {
				b.WriteRune('?')
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Add adds a log line and publishes if batch is full
func (c *LogCollector) Add(ctx context.Context, line string) error {
	// Sanitize UTF-8 before storing
	line = sanitizeUTF8(line)
	// Add to logs array first (for database storage later)
	c.logs = append(c.logs, line)

	// Publish immediately for real-time streaming with project ID
	if err := c.publisher.PublishLogWithProject(ctx, c.projectID, c.buildID, line, "info"); err != nil {
		return err
	}

	// Save to database in batches to ensure logs are persisted even if build fails
	if c.appendFunc != nil && len(c.logs) >= c.batchSize {
		batch := make([]string, len(c.logs))
		for i, log := range c.logs {
			batch[i] = sanitizeUTF8(log)
		}
		if err := c.appendFunc(ctx, c.buildID, batch); err != nil {
			// Log error but don't fail - continue collecting logs
			c.publisher.log.Warn().Err(err).Msg("Failed to save log batch to database")
		} else {
			// Clear logs after successful save
			c.logs = c.logs[:0]
		}
	}

	return nil
}

// Flush publishes any remaining logs
func (c *LogCollector) Flush(ctx context.Context) error {
	// Save any remaining logs to database
	if c.appendFunc != nil && len(c.logs) > 0 {
		batch := make([]string, len(c.logs))
		for i, log := range c.logs {
			batch[i] = sanitizeUTF8(log)
		}
		if err := c.appendFunc(ctx, c.buildID, batch); err != nil {
			return fmt.Errorf("flush logs: %w", err)
		}
		c.logs = c.logs[:0]
	}
	return nil
}

// GetLogs returns all collected logs
func (c *LogCollector) GetLogs() []string {
	return c.logs
}
