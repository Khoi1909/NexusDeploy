package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	aipb "github.com/nexusdeploy/backend/services/ai-service/proto"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BuildServiceClient defines the methods of Build Service
type BuildServiceClient interface {
	TriggerBuild(ctx context.Context, in *buildpb.TriggerBuildRequest, opts ...grpc.CallOption) (*buildpb.TriggerBuildResponse, error)
	ListBuilds(ctx context.Context, in *buildpb.ListBuildsRequest, opts ...grpc.CallOption) (*buildpb.ListBuildsResponse, error)
	GetBuild(ctx context.Context, in *buildpb.GetBuildRequest, opts ...grpc.CallOption) (*buildpb.GetBuildResponse, error)
	GetBuildLogs(ctx context.Context, in *buildpb.GetBuildLogsRequest, opts ...grpc.CallOption) (*buildpb.GetBuildLogsResponse, error)
	DeleteBuildLogs(ctx context.Context, in *buildpb.DeleteBuildLogsRequest, opts ...grpc.CallOption) (*buildpb.DeleteBuildLogsResponse, error)
}

// AIServiceClient defines the methods of AI Service
type AIServiceClient interface {
	AnalyzeBuild(ctx context.Context, in *aipb.AnalyzeBuildRequest, opts ...grpc.CallOption) (*aipb.AnalyzeBuildResponse, error)
}

// BuildHandler handles build-related requests
type BuildHandler struct {
	Client   BuildServiceClient
	AIClient AIServiceClient
}

// NewBuildHandler creates a new BuildHandler
func NewBuildHandler(client BuildServiceClient, aiClient AIServiceClient) *BuildHandler {
	return &BuildHandler{
		Client:   client,
		AIClient: aiClient,
	}
}

// ==================== REST Response Types ====================

type Build struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"project_id"`
	CommitSHA  string     `json:"commit_sha"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type BuildStep struct {
	ID         string `json:"id"`
	BuildID    string `json:"build_id"`
	StepName   string `json:"step_name"`
	Status     string `json:"status"`
	DurationMs int32  `json:"duration_ms,omitempty"`
}

type BuildLogEntry struct {
	ID        int64     `json:"id"`
	BuildID   string    `json:"build_id"`
	Timestamp time.Time `json:"timestamp"`
	LogLine   string    `json:"log_line"`
}

// ==================== Build Endpoints ====================

// ListBuilds handles GET /api/projects/{id}/builds
func (h *BuildHandler) ListBuilds(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Extract project_id from path: /api/projects/{id}/builds
	projectID := extractProjectIDFromBuildsPath(r.URL.Path)
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	page := parseQueryInt(r, "page", 1)
	pageSize := parseQueryInt(r, "page_size", 20)

	resp, err := h.Client.ListBuilds(r.Context(), &buildpb.ListBuildsRequest{
		ProjectId: projectID,
		UserId:    userID,
		Page:      int32(page),
		PageSize:  int32(pageSize),
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if resp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": resp.Error})
		return
	}

	builds := make([]Build, 0, len(resp.Builds))
	for _, b := range resp.Builds {
		builds = append(builds, protoToBuild(b))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"builds": builds,
		"total":  resp.Total,
	})
}

// GetBuild handles GET /api/builds/{id}
func (h *BuildHandler) GetBuild(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	buildID := extractBuildID(r.URL.Path)
	if buildID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "build_id required"})
		return
	}

	resp, err := h.Client.GetBuild(r.Context(), &buildpb.GetBuildRequest{
		BuildId: buildID,
		UserId:  userID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if resp.Error != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": resp.Error})
		return
	}

	steps := make([]BuildStep, 0, len(resp.Steps))
	for _, s := range resp.Steps {
		steps = append(steps, protoToStep(s))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"build": protoToBuild(resp.Build),
		"steps": steps,
	})
}

// ClearBuildLogs handles DELETE /api/projects/{id}/builds/logs
func (h *BuildHandler) ClearBuildLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Extract project_id from path: /api/projects/{id}/builds/logs
	projectID := extractProjectIDFromBuildsLogsPath(r.URL.Path)
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	// Delete logs for all builds in this project
	deleteResp, err := h.Client.DeleteBuildLogs(r.Context(), &buildpb.DeleteBuildLogsRequest{
		ProjectId: projectID,
		UserId:    userID,
		// Empty build_ids means delete all logs for all builds in the project
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if deleteResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": deleteResp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":         "Build logs cleared",
		"builds_affected": deleteResp.BuildsAffected,
		"logs_deleted":    deleteResp.LogsDeleted,
	})
}

// GetBuildLogs handles GET /api/builds/{id}/logs
func (h *BuildHandler) GetBuildLogs(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	buildID := extractBuildIDFromLogsPath(r.URL.Path)
	if buildID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "build_id required"})
		return
	}

	limit := parseQueryInt(r, "limit", 500)
	afterID := int64(parseQueryInt(r, "after_id", 0))

	resp, err := h.Client.GetBuildLogs(r.Context(), &buildpb.GetBuildLogsRequest{
		BuildId: buildID,
		Limit:   int32(limit),
		AfterId: afterID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if resp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": resp.Error})
		return
	}

	logs := make([]BuildLogEntry, 0, len(resp.Logs))
	for _, l := range resp.Logs {
		logs = append(logs, protoToLog(l))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":     logs,
		"has_more": resp.HasMore,
	})
}

// TriggerBuild handles POST /api/projects/{id}/builds (manual trigger)
func (h *BuildHandler) TriggerBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractProjectIDFromBuildsPath(r.URL.Path)
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	var req struct {
		CommitSHA string `json:"commit_sha"`
		Branch    string `json:"branch"`
		RepoURL   string `json:"repo_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	resp, err := h.Client.TriggerBuild(r.Context(), &buildpb.TriggerBuildRequest{
		ProjectId: projectID,
		UserId:    userID,
		CommitSha: req.CommitSHA,
		Branch:    req.Branch,
		RepoUrl:   req.RepoURL,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if resp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": resp.Error})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"build": protoToBuild(resp.Build),
	})
}

// ==================== Helper Functions ====================

func extractProjectIDFromBuildsPath(path string) string {
	// /api/projects/{project_id}/builds
	const prefix = "/api/projects/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/builds")
	if idx == -1 {
		return ""
	}
	return rest[:idx]
}

func extractBuildID(path string) string {
	// /api/builds/{build_id}
	const prefix = "/api/builds/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(rest, "/"); idx != -1 {
		return rest[:idx]
	}
	return rest
}

func extractBuildIDFromLogsPath(path string) string {
	// /api/builds/{build_id}/logs
	const prefix = "/api/builds/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/logs")
	if idx == -1 {
		return ""
	}
	return rest[:idx]
}

func extractBuildIDFromAnalyzePath(path string) string {
	// /api/builds/{build_id}/analyze
	const prefix = "/api/builds/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/analyze")
	if idx == -1 {
		return ""
	}
	return rest[:idx]
}

func extractProjectIDFromBuildsLogsPath(path string) string {
	// /api/projects/{project_id}/builds/logs
	const prefix = "/api/projects/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/builds/logs")
	if idx == -1 {
		return ""
	}
	return rest[:idx]
}

func toTimePtr(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func protoToBuild(b *buildpb.Build) Build {
	if b == nil {
		return Build{}
	}
	return Build{
		ID:         b.Id,
		ProjectID:  b.ProjectId,
		CommitSHA:  b.CommitSha,
		Status:     statusToString(b.Status),
		StartedAt:  toTimePtr(b.StartedAt),
		FinishedAt: toTimePtr(b.FinishedAt),
		CreatedAt:  toTime(b.CreatedAt),
		UpdatedAt:  toTime(b.UpdatedAt),
	}
}

func protoToStep(s *buildpb.BuildStep) BuildStep {
	if s == nil {
		return BuildStep{}
	}
	return BuildStep{
		ID:         s.Id,
		BuildID:    s.BuildId,
		StepName:   s.StepName,
		Status:     s.Status,
		DurationMs: s.DurationMs,
	}
}

func protoToLog(l *buildpb.BuildLog) BuildLogEntry {
	if l == nil {
		return BuildLogEntry{}
	}
	return BuildLogEntry{
		ID:        l.Id,
		BuildID:   l.BuildId,
		Timestamp: toTime(l.Timestamp),
		LogLine:   l.LogLine,
	}
}

func statusToString(status buildpb.BuildStatus) string {
	switch status {
	case buildpb.BuildStatus_BUILD_STATUS_PENDING:
		return "pending"
	case buildpb.BuildStatus_BUILD_STATUS_RUNNING:
		return "running"
	case buildpb.BuildStatus_BUILD_STATUS_FAILED:
		return "failed"
	case buildpb.BuildStatus_BUILD_STATUS_BUILDING_IMAGE:
		return "building_image"
	case buildpb.BuildStatus_BUILD_STATUS_PUSHING_IMAGE:
		return "pushing_image"
	case buildpb.BuildStatus_BUILD_STATUS_DEPLOYING:
		return "deploying"
	case buildpb.BuildStatus_BUILD_STATUS_SUCCESS:
		return "success"
	case buildpb.BuildStatus_BUILD_STATUS_DEPLOY_FAILED:
		return "deploy_failed"
	default:
		return "unknown"
	}
}

// AnalyzeBuild handles POST /api/builds/{id}/analyze
func (h *BuildHandler) AnalyzeBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Extract build_id from path
	buildID := extractBuildIDFromAnalyzePath(r.URL.Path)
	if buildID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "build_id required"})
		return
	}

	// Get user plan from auth context (set by AuthMiddleware)
	userPlan := apimw.GetPlan(r.Context())
	if userPlan == "" {
		userPlan = "standard" // default fallback
	}

	// Check if build exists and belongs to user
	buildResp, err := h.Client.GetBuild(r.Context(), &buildpb.GetBuildRequest{
		BuildId: buildID,
		UserId:  userID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if buildResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": buildResp.Error})
		return
	}

	if buildResp.Build == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}

	// Only analyze failed builds
	buildStatus := buildResp.Build.Status
	if buildStatus != buildpb.BuildStatus_BUILD_STATUS_FAILED && buildStatus != buildpb.BuildStatus_BUILD_STATUS_DEPLOY_FAILED {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "can only analyze failed builds"})
		return
	}

	// Call AI Service
	if h.AIClient == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI service not available"})
		return
	}

	aiResp, err := h.AIClient.AnalyzeBuild(r.Context(), &aipb.AnalyzeBuildRequest{
		BuildId:  buildID,
		UserPlan: userPlan,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if aiResp.Error != "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": aiResp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"analysis":    aiResp.Analysis,
		"suggestions": aiResp.Suggestions,
		"cached":      aiResp.Cached,
	})
}
