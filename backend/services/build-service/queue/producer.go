package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	// TaskTypeBuildJob is the type name for build jobs
	TaskTypeBuildJob = "build:execute"

	// Default queue name
	QueueBuilds = "builds"

	// Task retention period
	TaskRetention = 24 * time.Hour
)

// BuildJobPayload represents the job payload sent to Runner Service
// Matches SRS C.2 Message Queue Format
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

// Producer handles pushing jobs to the Redis queue via Asynq
type Producer struct {
	client *asynq.Client
}

// NewProducer creates a new queue producer
func NewProducer(redisAddr string) (*Producer, error) {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisAddr,
	})

	return &Producer{
		client: client,
	}, nil
}

// Close closes the producer connection
func (p *Producer) Close() error {
	return p.client.Close()
}

// EnqueueBuildJob pushes a build job to the queue
func (p *Producer) EnqueueBuildJob(ctx context.Context, payload *BuildJobPayload) (*asynq.TaskInfo, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskTypeBuildJob, data,
		asynq.Queue(QueueBuilds),
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
		asynq.Retention(TaskRetention),
		asynq.TaskID(payload.BuildID), // Ensure idempotency
	)

	info, err := p.client.Enqueue(task)
	if err != nil {
		return nil, fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("build_id", payload.BuildID).
		Str("project_id", payload.ProjectID).
		Str("task_id", info.ID).
		Str("queue", info.Queue).
		Msg("Build job enqueued")

	return info, nil
}

// ParseBuildJobPayload deserializes a build job payload (used by Runner Service)
func ParseBuildJobPayload(data []byte) (*BuildJobPayload, error) {
	var payload BuildJobPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return &payload, nil
}

