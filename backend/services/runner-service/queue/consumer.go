package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

const (
	// TaskTypeBuildJob matches the task type from Build Service
	TaskTypeBuildJob = "build:execute"

	// QueueBuilds is the queue name
	QueueBuilds = "builds"
)

// BuildJobPayload represents the job payload from Build Service
// Must match build-service/queue/producer.go
type BuildJobPayload struct {
	BuildID      string            `json:"build_id"`
	ProjectID    string            `json:"project_id"`
	RepoURL      string            `json:"repo_url"`
	Branch       string            `json:"branch"`
	CommitSHA    string            `json:"commit_sha"`
	BuildCommand string            `json:"build_command"`
	StartCommand string            `json:"start_command"`
	Preset       string            `json:"preset"`
	Port         int               `json:"port"`
	Secrets      map[string]string `json:"secrets"`
}

// ParseBuildJobPayload deserializes a build job payload
func ParseBuildJobPayload(data []byte) (*BuildJobPayload, error) {
	var payload BuildJobPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &payload, nil
}

// BuildJobHandler is the interface for handling build jobs
type BuildJobHandler interface {
	HandleBuildJob(ctx context.Context, payload *BuildJobPayload) error
}

// Consumer wraps the Asynq server for consuming build jobs
type Consumer struct {
	server  *asynq.Server
	handler BuildJobHandler
	log     zerolog.Logger
}

// ConsumerConfig holds configuration for the consumer
type ConsumerConfig struct {
	RedisAddr   string
	Concurrency int // Number of concurrent workers
}

// NewConsumer creates a new queue consumer
func NewConsumer(cfg ConsumerConfig, handler BuildJobHandler, log zerolog.Logger) *Consumer {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				QueueBuilds: 10, // Priority weight
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				taskID := "unknown"
				if task != nil {
					if rw := task.ResultWriter(); rw != nil {
						taskID = rw.TaskID()
					}
				}
				log.Error().
					Err(err).
					Str("task_type", task.Type()).
					Str("task_id", taskID).
					Msg("Task processing failed")
			}),
		},
	)

	return &Consumer{
		server:  srv,
		handler: handler,
		log:     log,
	}
}

// Start starts the consumer server
func (c *Consumer) Start() error {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TaskTypeBuildJob, c.handleBuildTask)

	c.log.Info().Msg("Starting Asynq consumer for build jobs")
	return c.server.Start(mux)
}

// Shutdown gracefully shuts down the consumer
func (c *Consumer) Shutdown() {
	c.log.Info().Msg("Shutting down Asynq consumer")
	c.server.Shutdown()
}

// handleBuildTask is the Asynq handler for build tasks
func (c *Consumer) handleBuildTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseBuildJobPayload(task.Payload())
	if err != nil {
		c.log.Error().Err(err).Msg("Failed to parse build job payload")
		return fmt.Errorf("parse payload: %w", err)
	}

	c.log.Info().
		Str("build_id", payload.BuildID).
		Str("project_id", payload.ProjectID).
		Str("repo_url", payload.RepoURL).
		Str("branch", payload.Branch).
		Msg("Received build job")

	if err := c.handler.HandleBuildJob(ctx, payload); err != nil {
		c.log.Error().
			Err(err).
			Str("build_id", payload.BuildID).
			Msg("Build job failed")
		return err
	}

	c.log.Info().
		Str("build_id", payload.BuildID).
		Msg("Build job completed successfully")

	return nil
}

