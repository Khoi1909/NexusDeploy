package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"google.golang.org/grpc"
)

// DeploymentServiceClient defines the methods of Deployment Service
type DeploymentServiceClient interface {
	Deploy(ctx context.Context, in *deploymentpb.DeployRequest, opts ...grpc.CallOption) (*deploymentpb.DeployResponse, error)
	StopDeployment(ctx context.Context, in *deploymentpb.StopDeploymentRequest, opts ...grpc.CallOption) (*deploymentpb.StopDeploymentResponse, error)
	RestartDeployment(ctx context.Context, in *deploymentpb.RestartDeploymentRequest, opts ...grpc.CallOption) (*deploymentpb.RestartDeploymentResponse, error)
	GetDeploymentStatus(ctx context.Context, in *deploymentpb.GetDeploymentStatusRequest, opts ...grpc.CallOption) (*deploymentpb.GetDeploymentStatusResponse, error)
}

// BuildServiceClientForDeployment defines methods needed from Build Service
type BuildServiceClientForDeployment interface {
	ListBuilds(ctx context.Context, in *buildpb.ListBuildsRequest, opts ...grpc.CallOption) (*buildpb.ListBuildsResponse, error)
}

// ProjectServiceClientForDeployment defines methods needed from Project Service
type ProjectServiceClientForDeployment interface {
	GetProject(ctx context.Context, in *projectpb.GetProjectRequest, opts ...grpc.CallOption) (*projectpb.GetProjectResponse, error)
	GetSecrets(ctx context.Context, in *projectpb.GetSecretsRequest, opts ...grpc.CallOption) (*projectpb.GetSecretsResponse, error)
}

// AuthServiceClientForDeployment defines methods needed from Auth Service
type AuthServiceClientForDeployment interface {
	GetUserPlan(ctx context.Context, in *authpb.GetUserPlanRequest, opts ...grpc.CallOption) (*authpb.GetUserPlanResponse, error)
}

// DeploymentHandler handles deployment-related requests
type DeploymentHandler struct {
	DeploymentClient DeploymentServiceClient
	BuildClient      BuildServiceClientForDeployment
	ProjectClient    ProjectServiceClientForDeployment
	AuthClient       AuthServiceClientForDeployment
	RegistryURL      string
}

// NewDeploymentHandler creates a new DeploymentHandler
func NewDeploymentHandler(
	deploymentClient DeploymentServiceClient,
	buildClient BuildServiceClientForDeployment,
	projectClient ProjectServiceClientForDeployment,
	authClient AuthServiceClientForDeployment,
) *DeploymentHandler {
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		registryURL = "localhost:5000" // Default local registry
	}

	return &DeploymentHandler{
		DeploymentClient: deploymentClient,
		BuildClient:      buildClient,
		ProjectClient:    projectClient,
		AuthClient:       authClient,
		RegistryURL:      registryURL,
	}
}

// ==================== REST Response Types ====================

type Deployment struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	ContainerID string `json:"container_id"`
	Status      string `json:"status"`
	PublicURL   string `json:"public_url"`
}

// ==================== Deployment Endpoints ====================

// Deploy handles POST /api/projects/{id}/deploy
func (h *DeploymentHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractPathParam(r.URL.Path, "/api/projects/")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	// Remove trailing /deploy if present
	projectID = strings.TrimSuffix(projectID, "/deploy")

	ctx := r.Context()

	// Step 1: Get latest successful build
	// Fetch multiple builds to find the latest successful one (latest build might be running/pending)
	buildsResp, err := h.BuildClient.ListBuilds(ctx, &buildpb.ListBuildsRequest{
		ProjectId: projectID,
		UserId:    userID,
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if buildsResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": buildsResp.Error})
		return
	}

	if len(buildsResp.Builds) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no builds found for this project"})
		return
	}

	// Find latest successful build
	var latestBuild *buildpb.Build
	for _, b := range buildsResp.Builds {
		if b.Status == buildpb.BuildStatus_BUILD_STATUS_SUCCESS {
			latestBuild = b
			break
		}
	}

	if latestBuild == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no successful build found. Please trigger a build first."})
		return
	}

	// Step 2: Get project config
	projectResp, err := h.ProjectClient.GetProject(ctx, &projectpb.GetProjectRequest{
		ProjectId: projectID,
		UserId:    userID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if projectResp.Error != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": projectResp.Error})
		return
	}

	project := projectResp.Project

	// Step 3: Get secrets (decrypted)
	secretsResp, err := h.ProjectClient.GetSecrets(ctx, &projectpb.GetSecretsRequest{
		ProjectId: projectID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	// Build secrets map
	secretsMap := make(map[string]string)
	if secretsResp.Secrets != nil {
		for k, v := range secretsResp.Secrets {
			secretsMap[k] = v
		}
	}

	// Step 4: Generate image tag
	// Format: {registry_url}/{project_id}:{short_sha}
	shortSHA := latestBuild.CommitSha
	if shortSHA == "" {
		shortSHA = latestBuild.Id // Fallback to build ID
	}
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	// Use "nexus/" prefix to match runner-service local image tag
	imageTag := fmt.Sprintf("nexus/%s:%s", projectID, shortSHA)

	// Step 5: Create deployment spec
	// Domain will be auto-generated by deployment-service using TRAEFIK_DOMAIN_SUFFIX
	// Don't set Domain here to let deployment-service handle it
	deploymentSpec := &deploymentpb.DeploymentSpec{
		ProjectId: projectID,
		BuildId:   latestBuild.Id,
		ImageTag:  imageTag,
		Port:      project.Port,
		// Domain: leave empty to let deployment-service generate from TRAEFIK_DOMAIN_SUFFIX
		Secrets: secretsMap,
		Resources: &deploymentpb.ResourceLimits{
			MemoryMb: 512, // Default 512MB
			CpuCores: 1,   // Default 1 core
		},
		UserId: userID,
	}

	// Step 8: Call Deployment Service
	deployResp, err := h.DeploymentClient.Deploy(ctx, &deploymentpb.DeployRequest{
		Spec: deploymentSpec,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if deployResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": deployResp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": Deployment{
			ID:          deployResp.DeploymentId,
			ProjectID:   projectID,
			ContainerID: deployResp.ContainerId,
			Status:      deployResp.Status,
			PublicURL:   deployResp.PublicUrl,
		},
	})
}

// StopDeployment handles POST /api/projects/{id}/stop
func (h *DeploymentHandler) StopDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractPathParam(r.URL.Path, "/api/projects/")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	// Remove trailing /stop if present
	projectID = strings.TrimSuffix(projectID, "/stop")

	ctx := r.Context()

	// Get current deployment status to find deployment_id
	statusResp, err := h.DeploymentClient.GetDeploymentStatus(ctx, &deploymentpb.GetDeploymentStatusRequest{
		ProjectId: projectID,
		// DeploymentId can be empty, service will find by project_id
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if statusResp.Error != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": statusResp.Error})
		return
	}

	// Stop deployment
	stopResp, err := h.DeploymentClient.StopDeployment(ctx, &deploymentpb.StopDeploymentRequest{
		DeploymentId: statusResp.DeploymentId,
		ProjectId:    projectID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if stopResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": stopResp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": stopResp.Success,
	})
}

// RestartDeployment handles POST /api/projects/{id}/restart
func (h *DeploymentHandler) RestartDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractPathParam(r.URL.Path, "/api/projects/")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	// Remove trailing /restart if present
	projectID = strings.TrimSuffix(projectID, "/restart")

	ctx := r.Context()

	// Get current deployment status
	statusResp, err := h.DeploymentClient.GetDeploymentStatus(ctx, &deploymentpb.GetDeploymentStatusRequest{
		ProjectId: projectID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if statusResp.Error != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": statusResp.Error})
		return
	}

	// Restart deployment
	restartResp, err := h.DeploymentClient.RestartDeployment(ctx, &deploymentpb.RestartDeploymentRequest{
		DeploymentId: statusResp.DeploymentId,
		ProjectId:    projectID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if restartResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": restartResp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"container_id": restartResp.ContainerId,
	})
}

// GetDeploymentStatus handles GET /api/projects/{id}/deployment
func (h *DeploymentHandler) GetDeploymentStatus(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractPathParam(r.URL.Path, "/api/projects/")
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	// Remove trailing /deployment if present
	projectID = strings.TrimSuffix(projectID, "/deployment")

	ctx := r.Context()

	statusResp, err := h.DeploymentClient.GetDeploymentStatus(ctx, &deploymentpb.GetDeploymentStatusRequest{
		ProjectId: projectID,
	})
	if err != nil {
		statusCode, message, _ := commonmw.HandleGRPCError(err)
		// If deployment not found (404), return 200 with null deployment (valid state)
		if statusCode == http.StatusNotFound || strings.Contains(message, "not found") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"deployment": nil,
			})
			return
		}
		writeJSON(w, statusCode, map[string]string{"error": message})
		return
	}

	if statusResp.Error != "" {
		// If deployment not found, return 200 with null (valid state)
		if strings.Contains(statusResp.Error, "not found") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"deployment": nil,
			})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": statusResp.Error})
		return
	}

	// Nếu không có deployment_id hoặc status UNSPECIFIED -> xem như chưa có deployment
	if statusResp.DeploymentId == "" || statusResp.Status == deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deployment": nil,
		})
		return
	}

	deployment := Deployment{
		ID:          statusResp.DeploymentId,
		ProjectID:   projectID,
		ContainerID: statusResp.ContainerId,
		Status:      deploymentStatusToString(statusResp.Status),
		PublicURL:   statusResp.PublicUrl,
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": deployment,
	})
}

// ==================== Helper Functions ====================

func deploymentStatusToString(status deploymentpb.DeploymentStatus) string {
	switch status {
	case deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_PENDING:
		return "pending"
	case deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING:
		return "running"
	case deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_STOPPED:
		return "stopped"
	case deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_FAILED:
		return "failed"
	case deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RESTARTING:
		return "restarting"
	default:
		return "unknown"
	}
}
