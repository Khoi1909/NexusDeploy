package handler

import (
	"context"
	"fmt"
	"time"

	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	"github.com/nexusdeploy/backend/services/runner-service/clients"
	"github.com/nexusdeploy/backend/services/runner-service/executor"
	"github.com/nexusdeploy/backend/services/runner-service/pubsub"
	"github.com/nexusdeploy/backend/services/runner-service/queue"
	"github.com/rs/zerolog"
)

// BuildHandler handles build jobs from the queue
type BuildHandler struct {
	clients   *clients.Clients
	executor  *executor.DockerExecutor
	publisher *pubsub.Publisher
	log       zerolog.Logger
}

// NewBuildHandler creates a new build handler
func NewBuildHandler(
	clients *clients.Clients,
	executor *executor.DockerExecutor,
	publisher *pubsub.Publisher,
	log zerolog.Logger,
) *BuildHandler {
	return &BuildHandler{
		clients:   clients,
		executor:  executor,
		publisher: publisher,
		log:       log,
	}
}

// HandleBuildJob implements the queue.BuildJobHandler interface
func (h *BuildHandler) HandleBuildJob(ctx context.Context, payload *queue.BuildJobPayload) error {
	startTime := time.Now()
	buildID := payload.BuildID

	h.log.Info().
		Str("build_id", buildID).
		Str("project_id", payload.ProjectID).
		Str("repo_url", payload.RepoURL).
		Msg("Starting build job")

	// Create log collector with project ID for proper channel naming
	// Pass appendFunc to save logs to database in batches
	appendFunc := func(ctx context.Context, buildID string, logs []string) error {
		return h.clients.AppendBuildLogs(ctx, buildID, logs)
	}
	logCollector := pubsub.NewLogCollectorWithProject(h.publisher, payload.ProjectID, buildID, 10, appendFunc)

	// Helper function to log and publish
	logLine := func(line string) {
		logCollector.Add(ctx, line)
		h.log.Debug().Str("build_id", buildID).Msg(line)
	}

	// Notify build started
	h.publisher.PublishBuildStarted(ctx, buildID)

	// Update status to Running
	if err := h.clients.UpdateBuildStatus(ctx, buildID, buildpb.BuildStatus_BUILD_STATUS_RUNNING, nil); err != nil {
		h.log.Error().Err(err).Msg("Failed to update build status to Running")
		// Continue anyway
	}

	// Build context for executor
	bc := &executor.BuildContext{
		BuildID:      buildID,
		ProjectID:    payload.ProjectID,
		RepoURL:      payload.RepoURL,
		Branch:       payload.Branch,
		CommitSHA:    payload.CommitSHA,
		BuildCommand: payload.BuildCommand,
		StartCommand: payload.StartCommand,
		Preset:       payload.Preset,
		Port:         payload.Port,
		Secrets:      payload.Secrets,
	}

	// Fetch project info from Project Service if missing
	if bc.RepoURL == "" || bc.Branch == "" || bc.Preset == "" || bc.Port == 0 {
		logLine("[setup] Fetching project configuration from Project Service...")
		// Note: userID is not available in payload, use empty string (Project Service should handle this)
		project, err := h.clients.GetProject(ctx, payload.ProjectID, "")
		if err != nil {
			logLine(fmt.Sprintf("[setup] Warning: Failed to fetch project: %v", err))
		} else {
			if bc.RepoURL == "" && project.RepoUrl != "" {
				bc.RepoURL = project.RepoUrl
			}
			if bc.Branch == "" && project.Branch != "" {
				bc.Branch = project.Branch
			}
			if bc.Preset == "" && project.Preset != "" {
				bc.Preset = project.Preset
			}
			if bc.Port == 0 && project.Port != 0 {
				bc.Port = int(project.Port)
			}
			if bc.BuildCommand == "" && project.BuildCommand != "" {
				bc.BuildCommand = project.BuildCommand
			}
			if bc.StartCommand == "" && project.StartCommand != "" {
				bc.StartCommand = project.StartCommand
			}
		}
	}

	// Fetch secrets from Project Service if not provided
	if len(bc.Secrets) == 0 {
		logLine("[setup] Fetching secrets from Project Service...")
		secrets, err := h.clients.GetProjectSecrets(ctx, payload.ProjectID)
		if err != nil {
			logLine(fmt.Sprintf("[setup] Warning: Failed to fetch secrets: %v", err))
			// Non-fatal, continue without secrets
		} else {
			bc.Secrets = secrets
			logLine(fmt.Sprintf("[setup] Loaded %d secrets", len(secrets)))
		}
	}

	// Execute build pipeline
	result := h.executePipeline(ctx, bc, logLine)

	// Calculate duration
	duration := time.Since(startTime)
	logLine(fmt.Sprintf("[done] Build completed in %s", duration.Round(time.Second)))

	// Flush any remaining logs to database
	if err := logCollector.Flush(ctx); err != nil {
		h.log.Error().Err(err).Msg("Failed to flush remaining build logs")
	}

	// Update final status
	var finalStatus buildpb.BuildStatus
	var statusMessage string

	if result.Success {
		finalStatus = buildpb.BuildStatus_BUILD_STATUS_SUCCESS
		statusMessage = fmt.Sprintf("Build successful, image: %s", result.ImageTag)
		h.publisher.PublishBuildCompleted(ctx, buildID, "success", statusMessage)
	} else {
		finalStatus = buildpb.BuildStatus_BUILD_STATUS_FAILED
		statusMessage = fmt.Sprintf("Build failed: %v", result.Error)
		h.publisher.PublishBuildCompleted(ctx, buildID, "failed", statusMessage)
	}

	if err := h.clients.UpdateBuildStatus(ctx, buildID, finalStatus, []string{statusMessage}); err != nil {
		h.log.Error().Err(err).Msg("Failed to update final build status")
	}

	// Cleanup workspace
	if result.WorkDir != "" {
		if err := h.executor.Cleanup(result.WorkDir); err != nil {
			h.log.Warn().Err(err).Str("workspace", result.WorkDir).Msg("Failed to cleanup workspace")
		}
	}

	if result.Success {
		return nil
	}
	return result.Error
}

// executePipeline runs the full build pipeline
func (h *BuildHandler) executePipeline(ctx context.Context, bc *executor.BuildContext, logLine func(string)) *executor.BuildResult {
	result := &executor.BuildResult{
		Success: false,
	}

	// Step 1: Clone repository
	logLine("[step 1/4] Cloning repository...")
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "clone", "running")

	workspace, err := h.executor.CloneRepository(ctx, bc, logLine)
	if err != nil {
		result.Error = fmt.Errorf("clone repository: %w", err)
		h.publisher.PublishStepComplete(ctx, bc.BuildID, "clone", "failed")
		return result
	}
	result.WorkDir = workspace
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "clone", "success")

	// Step 2: Run build command
	logLine("[step 2/4] Running build command...")
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "build", "running")

	if err := h.executor.RunBuildCommand(ctx, bc, workspace, logLine); err != nil {
		result.Error = fmt.Errorf("build command: %w", err)
		h.publisher.PublishStepComplete(ctx, bc.BuildID, "build", "failed")
		return result
	}
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "build", "success")

	// Update status to BuildingImage
	h.clients.UpdateBuildStatus(ctx, bc.BuildID, buildpb.BuildStatus_BUILD_STATUS_BUILDING_IMAGE, nil)

	// Step 3: Build Docker image
	logLine("[step 3/4] Building Docker image...")
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_build", "running")

	imageTag, err := h.executor.BuildDockerImage(ctx, bc, workspace, logLine)
	if err != nil {
		result.Error = fmt.Errorf("build docker image: %w", err)
		h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_build", "failed")
		return result
	}
	result.ImageTag = imageTag
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_build", "success")

	// Update status to PushingImage
	h.clients.UpdateBuildStatus(ctx, bc.BuildID, buildpb.BuildStatus_BUILD_STATUS_PUSHING_IMAGE, nil)

	// Step 4: Push image to registry
	logLine("[step 4/4] Pushing image to registry...")
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_push", "running")

	if err := h.executor.PushImage(ctx, imageTag, logLine); err != nil {
		result.Error = fmt.Errorf("push image: %w", err)
		h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_push", "failed")
		return result
	}
	h.publisher.PublishStepComplete(ctx, bc.BuildID, "docker_push", "success")

	result.Success = true
	return result
}
