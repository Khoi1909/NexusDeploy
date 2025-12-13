package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/nexusdeploy/backend/services/build-service/models"
	pb "github.com/nexusdeploy/backend/services/build-service/proto"
	"github.com/nexusdeploy/backend/services/build-service/queue"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

var log zerolog.Logger

func init() {
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "build-service").
		Logger()
}

// BuildServiceServer implements the BuildService gRPC server
type BuildServiceServer struct {
	pb.UnimplementedBuildServiceServer
	db         *gorm.DB
	cfg        *cfgpkg.Config
	producer   *queue.Producer
	authClient authpb.AuthServiceClient
	authConn   *grpc.ClientConn
}

// NewBuildServiceServer creates a new BuildService server
func NewBuildServiceServer(db *gorm.DB, cfg *cfgpkg.Config, producer *queue.Producer, authClient authpb.AuthServiceClient, authConn *grpc.ClientConn) *BuildServiceServer {
	return &BuildServiceServer{
		db:         db,
		cfg:        cfg,
		producer:   producer,
		authClient: authClient,
		authConn:   authConn,
	}
}

// getCorrelationID extracts correlation_id from gRPC metadata for logging
func getCorrelationID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return "unknown"
}

// ==================== TriggerBuild ====================

// TriggerBuild creates a new build and enqueues it for processing
func (s *BuildServiceServer) TriggerBuild(ctx context.Context, req *pb.TriggerBuildRequest) (*pb.TriggerBuildResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("project_id", req.ProjectId).
		Str("commit_sha", req.CommitSha).
		Msg("TriggerBuild called")

	// Validate request
	if req.ProjectId == "" {
		return &pb.TriggerBuildResponse{Error: "project_id is required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.TriggerBuildResponse{Error: "invalid project_id format"}, nil
	}

	// Check permission: enforce max_builds_per_month and concurrent builds (FR7.4)
	if s.authClient != nil && req.UserId != "" {
		planResp, err := s.authClient.GetUserPlan(ctx, &authpb.GetUserPlanRequest{
			UserId: req.UserId,
		})
		if err != nil {
			log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("Failed to get user plan for permission check")
			return &pb.TriggerBuildResponse{Error: "failed to check user plan"}, nil
		}
		if planResp.Error != "" {
			log.Warn().Str("error", planResp.Error).Str("user_id", req.UserId).Msg("GetUserPlan returned error")
			return &pb.TriggerBuildResponse{Error: "failed to check user plan: " + planResp.Error}, nil
		}

		// Note: Ownership verification should be done by API Gateway before calling Build Service
		// Build Service trusts that API Gateway has verified the user owns the project

		// Check concurrent builds limit - count active builds across all user's projects
		// Note: Build Service doesn't have access to project_db, so we trust API Gateway
		// has verified ownership. We count builds for this project only (concurrent builds
		// are limited per project in practice, but plan limit applies globally).
		// For proper per-user counting, this would require Project Service client injection.
		// Active builds: pending, running, building_image, pushing_image, deploying
		var activeBuildCount int64
		activeStatuses := []string{
			string(models.BuildStatusPending),
			string(models.BuildStatusRunning),
			string(models.BuildStatusBuildingImage),
			string(models.BuildStatusPushingImage),
			string(models.BuildStatusDeploying),
		}
		// Count active builds for this project (API Gateway ensures user owns the project)
		if err := s.db.Model(&models.Build{}).
			Where("project_id = ? AND status IN ?", projectID, activeStatuses).
			Count(&activeBuildCount).Error; err != nil {
			log.Error().Err(err).Str("correlation_id", corrID).Msg("Failed to count active builds")
			return &pb.TriggerBuildResponse{Error: "failed to check build limits"}, nil
		}

		// Standard plan: max 1 concurrent build, Premium: max 5 concurrent builds (per SRS 4.6)
		maxConcurrentBuilds := int32(1) // Standard default
		if planResp.Plan == "premium" {
			maxConcurrentBuilds = 5
		}

		// Note: This counts concurrent builds for current project only.
		// To properly enforce per-user limit, we would need Project Service client
		// to get all user's projects. For MVP, we trust API Gateway has verified ownership.
		if activeBuildCount >= int64(maxConcurrentBuilds) {
			log.Warn().
				Str("correlation_id", corrID).
				Str("user_id", req.UserId).
				Int64("active_builds", activeBuildCount).
				Int32("max_concurrent", maxConcurrentBuilds).
				Str("plan", planResp.Plan).
				Msg("User reached concurrent builds limit")
			return &pb.TriggerBuildResponse{
				Error: fmt.Sprintf("You have reached the concurrent builds limit for the %s plan (%d builds). Please wait for current builds to complete or upgrade your plan.", planResp.Plan, maxConcurrentBuilds),
			}, nil
		}
	}

	// Create build record
	build := &models.Build{
		ProjectID: projectID,
		CommitSHA: req.CommitSha,
		Status:    models.BuildStatusPending,
	}

	if err := s.db.Create(build).Error; err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("Failed to create build")
		return &pb.TriggerBuildResponse{Error: "failed to create build"}, nil
	}

	// Create initial build steps
	steps := []models.BuildStep{
		{BuildID: build.ID, StepName: models.StepClone, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepInstall, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepBuild, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepTest, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepDockerBuild, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepDockerPush, Status: models.StepStatusPending},
		{BuildID: build.ID, StepName: models.StepDeploy, Status: models.StepStatusPending},
	}
	if err := s.db.Create(&steps).Error; err != nil {
		log.Warn().Err(err).Str("correlation_id", corrID).Msg("Failed to create build steps")
	}

	// Enqueue job for Runner Service
	// Note: In production, we'd fetch project config from Project Service
	payload := &queue.BuildJobPayload{
		BuildID:   build.ID.String(),
		ProjectID: req.ProjectId,
		RepoURL:   req.RepoUrl,
		Branch:    req.Branch,
		CommitSHA: req.CommitSha,
		Secrets:   make(map[string]string), // Will be populated by Runner Service
	}

	if _, err := s.producer.EnqueueBuildJob(ctx, payload); err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("Failed to enqueue build job")
		// Update build status to failed
		s.db.Model(build).Update("status", models.BuildStatusFailed)
		return &pb.TriggerBuildResponse{Error: "failed to enqueue build job"}, nil
	}

	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", build.ID.String()).
		Msg("Build triggered successfully")

	return &pb.TriggerBuildResponse{
		Build: buildToProto(build),
	}, nil
}

// ==================== UpdateBuildStatus ====================

// UpdateBuildStatus updates the status of a build (called by Runner Service)
func (s *BuildServiceServer) UpdateBuildStatus(ctx context.Context, req *pb.UpdateBuildStatusRequest) (*pb.UpdateBuildStatusResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Int32("status", int32(req.Status)).
		Msg("UpdateBuildStatus called")

	if req.BuildId == "" {
		return &pb.UpdateBuildStatusResponse{Error: "build_id is required"}, nil
	}

	buildID, err := uuid.Parse(req.BuildId)
	if err != nil {
		return &pb.UpdateBuildStatusResponse{Error: "invalid build_id format"}, nil
	}

	// Find build
	var build models.Build
	if err := s.db.First(&build, "id = ?", buildID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.UpdateBuildStatusResponse{Error: "build not found"}, nil
		}
		return &pb.UpdateBuildStatusResponse{Error: "failed to get build"}, nil
	}

	// Convert proto status to model status
	newStatus := protoStatusToModel(req.Status)

	// Validate state transition
	if !build.CanTransitionTo(newStatus) {
		log.Warn().
			Str("correlation_id", corrID).
			Str("current", string(build.Status)).
			Str("requested", string(newStatus)).
			Msg("Invalid status transition")
		return &pb.UpdateBuildStatusResponse{Error: "invalid status transition"}, nil
	}

	// Update build
	updates := map[string]interface{}{
		"status": newStatus,
	}

	now := time.Now()
	if newStatus == models.BuildStatusRunning && build.StartedAt == nil {
		updates["started_at"] = now
	}
	if build.IsTerminal() || newStatus == models.BuildStatusSuccess ||
		newStatus == models.BuildStatusFailed || newStatus == models.BuildStatusDeployFailed {
		updates["finished_at"] = now
	}

	// Lưu image_tag nếu có (từ Runner Service khi build image xong)
	if req.ImageTag != "" {
		updates["image_tag"] = req.ImageTag
	}

	if err := s.db.Model(&build).Updates(updates).Error; err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("Failed to update build")
		return &pb.UpdateBuildStatusResponse{Error: "failed to update build"}, nil
	}

	// Append logs if provided
	if len(req.LogLines) > 0 {
		if err := s.appendLogs(ctx, buildID, req.LogLines); err != nil {
			log.Warn().Err(err).Str("correlation_id", corrID).Msg("Failed to append logs")
		}
	}

	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Str("new_status", string(newStatus)).
		Msg("Build status updated")

	return &pb.UpdateBuildStatusResponse{Acknowledged: true}, nil
}

// ==================== ListBuilds ====================

// ListBuilds returns a paginated list of builds for a project
func (s *BuildServiceServer) ListBuilds(ctx context.Context, req *pb.ListBuildsRequest) (*pb.ListBuildsResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("project_id", req.ProjectId).
		Int32("page", req.Page).
		Int32("page_size", req.PageSize).
		Msg("ListBuilds called")

	if req.ProjectId == "" {
		return &pb.ListBuildsResponse{Error: "project_id is required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.ListBuildsResponse{Error: "invalid project_id format"}, nil
	}

	// Pagination defaults
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Query builds
	var builds []models.Build
	var total int64

	s.db.Model(&models.Build{}).Where("project_id = ?", projectID).Count(&total)

	log.Debug().
		Str("correlation_id", corrID).
		Str("project_id", projectID.String()).
		Int64("total", total).
		Int32("page", page).
		Int32("page_size", pageSize).
		Int32("offset", offset).
		Msg("Querying builds from database")

	if err := s.db.Where("project_id = ?", projectID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(pageSize)).
		Find(&builds).Error; err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("Failed to list builds")
		return &pb.ListBuildsResponse{Error: "failed to list builds"}, nil
	}

	log.Debug().
		Str("correlation_id", corrID).
		Int("builds_found", len(builds)).
		Msg("ListBuilds query results")

	protoBuilds := make([]*pb.Build, len(builds))
	for i, b := range builds {
		protoBuilds[i] = buildToProto(&b)
	}

	return &pb.ListBuildsResponse{
		Builds: protoBuilds,
		Total:  int32(total),
	}, nil
}

// ==================== GetBuild ====================

// GetBuild returns a build with its steps
func (s *BuildServiceServer) GetBuild(ctx context.Context, req *pb.GetBuildRequest) (*pb.GetBuildResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Msg("GetBuild called")

	if req.BuildId == "" {
		return &pb.GetBuildResponse{Error: "build_id is required"}, nil
	}

	buildID, err := uuid.Parse(req.BuildId)
	if err != nil {
		return &pb.GetBuildResponse{Error: "invalid build_id format"}, nil
	}

	var build models.Build
	if err := s.db.Preload("Steps").First(&build, "id = ?", buildID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.GetBuildResponse{Error: "build not found"}, nil
		}
		return &pb.GetBuildResponse{Error: "failed to get build"}, nil
	}

	protoSteps := make([]*pb.BuildStep, len(build.Steps))
	for i, step := range build.Steps {
		protoSteps[i] = stepToProto(&step)
	}

	return &pb.GetBuildResponse{
		Build: buildToProto(&build),
		Steps: protoSteps,
	}, nil
}

// ==================== GetBuildLogs ====================

// GetBuildLogs returns logs for a build (used by AI Service for analysis)
func (s *BuildServiceServer) GetBuildLogs(ctx context.Context, req *pb.GetBuildLogsRequest) (*pb.GetBuildLogsResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Int32("limit", req.Limit).
		Msg("GetBuildLogs called")

	if req.BuildId == "" {
		return &pb.GetBuildLogsResponse{Error: "build_id is required"}, nil
	}

	buildID, err := uuid.Parse(req.BuildId)
	if err != nil {
		return &pb.GetBuildLogsResponse{Error: "invalid build_id format"}, nil
	}

	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	query := s.db.Where("build_id = ?", buildID).Order("id ASC")
	if req.AfterId > 0 {
		query = query.Where("id > ?", req.AfterId)
	}

	var logs []models.BuildLog
	// Fetch one extra to check if there are more
	if err := query.Limit(int(limit) + 1).Find(&logs).Error; err != nil {
		return &pb.GetBuildLogsResponse{Error: "failed to get logs"}, nil
	}

	hasMore := len(logs) > int(limit)
	if hasMore {
		logs = logs[:limit]
	}

	protoLogs := make([]*pb.BuildLog, len(logs))
	for i, l := range logs {
		protoLogs[i] = logToProto(&l)
	}

	return &pb.GetBuildLogsResponse{
		Logs:    protoLogs,
		HasMore: hasMore,
	}, nil
}

// ==================== AppendBuildLogs ====================

// AppendBuildLogs appends logs to a build (called by Runner Service)
func (s *BuildServiceServer) AppendBuildLogs(ctx context.Context, req *pb.AppendBuildLogsRequest) (*pb.AppendBuildLogsResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("build_id", req.BuildId).
		Int("log_count", len(req.LogLines)).
		Msg("AppendBuildLogs called")

	if req.BuildId == "" {
		return &pb.AppendBuildLogsResponse{Error: "build_id is required"}, nil
	}

	buildID, err := uuid.Parse(req.BuildId)
	if err != nil {
		return &pb.AppendBuildLogsResponse{Error: "invalid build_id format"}, nil
	}

	if err := s.appendLogs(ctx, buildID, req.LogLines); err != nil {
		return &pb.AppendBuildLogsResponse{Error: "failed to append logs"}, nil
	}

	return &pb.AppendBuildLogsResponse{Acknowledged: true}, nil
}

// ==================== DeleteBuildLogs ====================

// DeleteBuildLogs deletes logs for builds in a project
func (s *BuildServiceServer) DeleteBuildLogs(ctx context.Context, req *pb.DeleteBuildLogsRequest) (*pb.DeleteBuildLogsResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("project_id", req.ProjectId).
		Str("user_id", req.UserId).
		Int("build_count", len(req.BuildIds)).
		Msg("DeleteBuildLogs called")

	if req.ProjectId == "" || req.UserId == "" {
		return &pb.DeleteBuildLogsResponse{Error: "project_id and user_id are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.DeleteBuildLogsResponse{Error: "invalid project_id format"}, nil
	}

	_, err = uuid.Parse(req.UserId)
	if err != nil {
		return &pb.DeleteBuildLogsResponse{Error: "invalid user_id format"}, nil
	}

	// Get builds for this project (to verify ownership)
	var buildModels []models.Build
	query := s.db.Where("project_id = ?", projectID)

	// If specific build IDs provided, filter by them
	if len(req.BuildIds) > 0 {
		buildUUIDs := make([]uuid.UUID, 0, len(req.BuildIds))
		for _, bid := range req.BuildIds {
			if buildUUID, err := uuid.Parse(bid); err == nil {
				buildUUIDs = append(buildUUIDs, buildUUID)
			}
		}
		if len(buildUUIDs) > 0 {
			query = query.Where("id IN ?", buildUUIDs)
		}
	}

	if err := query.Find(&buildModels).Error; err != nil {
		return &pb.DeleteBuildLogsResponse{Error: "failed to get builds"}, nil
	}

	if len(buildModels) == 0 {
		return &pb.DeleteBuildLogsResponse{
			BuildsAffected: 0,
			LogsDeleted:    0,
		}, nil
	}

	// Extract build IDs
	buildIDs := make([]uuid.UUID, len(buildModels))
	for i, b := range buildModels {
		buildIDs[i] = b.ID
	}

	// Count logs before deletion
	var logsCount int64
	s.db.Model(&models.BuildLog{}).Where("build_id IN ?", buildIDs).Count(&logsCount)

	// Delete logs for these builds (CASCADE will handle build_steps)
	if err := s.db.Where("build_id IN ?", buildIDs).Delete(&models.BuildLog{}).Error; err != nil {
		log.Error().Err(err).Msg("Failed to delete build logs")
		return &pb.DeleteBuildLogsResponse{Error: "failed to delete logs"}, nil
	}

	// Delete build steps (CASCADE should handle this, but explicit delete for safety)
	if err := s.db.Where("build_id IN ?", buildIDs).Delete(&models.BuildStep{}).Error; err != nil {
		log.Warn().Err(err).Msg("Failed to delete build steps (may not exist)")
		// Continue anyway
	}

	// Delete the builds themselves
	if err := s.db.Where("id IN ?", buildIDs).Delete(&models.Build{}).Error; err != nil {
		log.Error().Err(err).Msg("Failed to delete builds")
		return &pb.DeleteBuildLogsResponse{Error: "failed to delete builds"}, nil
	}

	// Cleanup workspaces in runner-service
	buildIDStrings := make([]string, len(buildIDs))
	for i, id := range buildIDs {
		buildIDStrings[i] = id.String()
	}

	// Call runner-service to cleanup workspaces
	if err := cleanupRunnerWorkspaces(ctx, buildIDStrings, s.cfg); err != nil {
		log.Warn().
			Err(err).
			Str("correlation_id", corrID).
			Msg("Failed to cleanup runner workspaces (non-fatal)")
		// Continue anyway - database cleanup succeeded
	}

	log.Info().
		Str("correlation_id", corrID).
		Int("builds_affected", len(buildIDs)).
		Int64("logs_deleted", logsCount).
		Msg("Build history deleted successfully")

	return &pb.DeleteBuildLogsResponse{
		BuildsAffected: int32(len(buildIDs)),
		LogsDeleted:    logsCount,
	}, nil
}

// ==================== Helper Functions ====================

func (s *BuildServiceServer) appendLogs(ctx context.Context, buildID uuid.UUID, lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	now := time.Now()
	logs := make([]models.BuildLog, len(lines))
	for i, line := range lines {
		logs[i] = models.BuildLog{
			BuildID:   buildID,
			Timestamp: now,
			LogLine:   line,
		}
	}

	return s.db.Create(&logs).Error
}

func buildToProto(b *models.Build) *pb.Build {
	if b == nil {
		return nil
	}

	build := &pb.Build{
		Id:        b.ID.String(),
		ProjectId: b.ProjectID.String(),
		CommitSha: b.CommitSHA,
		Status:    modelStatusToProto(b.Status),
		CreatedAt: timestamppb.New(b.CreatedAt),
		UpdatedAt: timestamppb.New(b.UpdatedAt),
		ImageTag:  b.ImageTag,
	}

	if b.StartedAt != nil {
		build.StartedAt = timestamppb.New(*b.StartedAt)
	}
	if b.FinishedAt != nil {
		build.FinishedAt = timestamppb.New(*b.FinishedAt)
	}

	return build
}

// cleanupRunnerWorkspaces calls runner-service to cleanup workspace directories
func cleanupRunnerWorkspaces(ctx context.Context, buildIDs []string, cfg *cfgpkg.Config) error {
	// Get runner-service address from config or use default
	runnerAddr := os.Getenv("RUNNER_SERVICE_ADDR")
	if runnerAddr == "" {
		runnerAddr = "runner-service:8080"
	}

	// Construct URL
	url := fmt.Sprintf("http://%s/api/cleanup-workspaces", runnerAddr)

	// Create request body
	reqBody := map[string]interface{}{
		"build_ids": buildIDs,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("runner-service returned status %d", resp.StatusCode)
	}

	return nil
}

func stepToProto(s *models.BuildStep) *pb.BuildStep {
	if s == nil {
		return nil
	}

	step := &pb.BuildStep{
		Id:       s.ID.String(),
		BuildId:  s.BuildID.String(),
		StepName: s.StepName,
		Status:   string(s.Status),
	}

	if s.DurationMs != nil {
		step.DurationMs = int32(*s.DurationMs)
	}

	return step
}

func logToProto(l *models.BuildLog) *pb.BuildLog {
	if l == nil {
		return nil
	}

	return &pb.BuildLog{
		Id:        l.ID,
		BuildId:   l.BuildID.String(),
		Timestamp: timestamppb.New(l.Timestamp),
		LogLine:   l.LogLine,
	}
}

func modelStatusToProto(status models.BuildStatus) pb.BuildStatus {
	switch status {
	case models.BuildStatusPending:
		return pb.BuildStatus_BUILD_STATUS_PENDING
	case models.BuildStatusRunning:
		return pb.BuildStatus_BUILD_STATUS_RUNNING
	case models.BuildStatusFailed:
		return pb.BuildStatus_BUILD_STATUS_FAILED
	case models.BuildStatusBuildingImage:
		return pb.BuildStatus_BUILD_STATUS_BUILDING_IMAGE
	case models.BuildStatusPushingImage:
		return pb.BuildStatus_BUILD_STATUS_PUSHING_IMAGE
	case models.BuildStatusDeploying:
		return pb.BuildStatus_BUILD_STATUS_DEPLOYING
	case models.BuildStatusSuccess:
		return pb.BuildStatus_BUILD_STATUS_SUCCESS
	case models.BuildStatusDeployFailed:
		return pb.BuildStatus_BUILD_STATUS_DEPLOY_FAILED
	default:
		return pb.BuildStatus_BUILD_STATUS_UNSPECIFIED
	}
}

func protoStatusToModel(status pb.BuildStatus) models.BuildStatus {
	switch status {
	case pb.BuildStatus_BUILD_STATUS_PENDING:
		return models.BuildStatusPending
	case pb.BuildStatus_BUILD_STATUS_RUNNING:
		return models.BuildStatusRunning
	case pb.BuildStatus_BUILD_STATUS_FAILED:
		return models.BuildStatusFailed
	case pb.BuildStatus_BUILD_STATUS_BUILDING_IMAGE:
		return models.BuildStatusBuildingImage
	case pb.BuildStatus_BUILD_STATUS_PUSHING_IMAGE:
		return models.BuildStatusPushingImage
	case pb.BuildStatus_BUILD_STATUS_DEPLOYING:
		return models.BuildStatusDeploying
	case pb.BuildStatus_BUILD_STATUS_SUCCESS:
		return models.BuildStatusSuccess
	case pb.BuildStatus_BUILD_STATUS_DEPLOY_FAILED:
		return models.BuildStatusDeployFailed
	default:
		return models.BuildStatusPending
	}
}
