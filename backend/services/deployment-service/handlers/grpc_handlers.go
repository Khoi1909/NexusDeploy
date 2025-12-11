package handlers

import (
	"context"
	"strings"

	"github.com/nexusdeploy/backend/pkg/logger"
	"github.com/nexusdeploy/backend/services/deployment-service/docker"
	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DeploymentHandler implements the DeploymentService gRPC interface
type DeploymentHandler struct {
	deploymentpb.UnimplementedDeploymentServiceServer
	executor *docker.Executor
	log      zerolog.Logger
}

// NewDeploymentHandler creates a new deployment handler
func NewDeploymentHandler(executor *docker.Executor, log zerolog.Logger) *DeploymentHandler {
	return &DeploymentHandler{
		executor: executor,
		log:      log,
	}
}

// Deploy deploys a new container from an image
func (h *DeploymentHandler) Deploy(ctx context.Context, req *deploymentpb.DeployRequest) (*deploymentpb.DeployResponse, error) {
	correlationID := logger.GetCorrelationID(ctx)
	h.log.Info().
		Str("correlation_id", correlationID).
		Str("project_id", req.Spec.ProjectId).
		Str("image", req.Spec.ImageTag).
		Msg("Deploy request received")

	if req.Spec == nil {
		return &deploymentpb.DeployResponse{
			Status: "failed",
			Error:  "deployment spec is required",
		}, nil
	}

	deployment, err := h.executor.Deploy(ctx, req.Spec)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("correlation_id", correlationID).
			Str("project_id", req.Spec.ProjectId).
			Msg("Deployment failed")

		return &deploymentpb.DeployResponse{
			Status: "failed",
			Error:  err.Error(),
		}, nil
	}

	h.log.Info().
		Str("correlation_id", correlationID).
		Str("deployment_id", deployment.ID).
		Str("container_id", deployment.ContainerID).
		Str("public_url", deployment.PublicURL).
		Msg("Deployment successful")

	return &deploymentpb.DeployResponse{
		DeploymentId: deployment.ID,
		ContainerId:  deployment.ContainerID,
		Status:       statusToString(deployment.Status),
		PublicUrl:    deployment.PublicURL,
	}, nil
}

// StopDeployment stops and removes a deployment
func (h *DeploymentHandler) StopDeployment(ctx context.Context, req *deploymentpb.StopDeploymentRequest) (*deploymentpb.StopDeploymentResponse, error) {
	correlationID := logger.GetCorrelationID(ctx)
	h.log.Info().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Str("project_id", req.ProjectId).
		Msg("Stop deployment request received")

	err := h.executor.Stop(ctx, req.DeploymentId, req.ProjectId)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("correlation_id", correlationID).
			Str("deployment_id", req.DeploymentId).
			Msg("Stop deployment failed")

		return &deploymentpb.StopDeploymentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	h.log.Info().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Msg("Deployment stopped")

	return &deploymentpb.StopDeploymentResponse{
		Success: true,
	}, nil
}

// GetDeploymentStatus returns the status of a deployment
func (h *DeploymentHandler) GetDeploymentStatus(ctx context.Context, req *deploymentpb.GetDeploymentStatusRequest) (*deploymentpb.GetDeploymentStatusResponse, error) {
	correlationID := logger.GetCorrelationID(ctx)
	h.log.Debug().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Str("project_id", req.ProjectId).
		Msg("Get deployment status request")

	// Nếu không có deployment_id và project_id, trả về luôn, tránh spam log
	if req.DeploymentId == "" && req.ProjectId == "" {
		return &deploymentpb.GetDeploymentStatusResponse{
			Status: deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED,
			Error:  "deployment_id or project_id is required",
		}, nil
	}

	deployment, err := h.executor.GetStatus(ctx, req.DeploymentId, req.ProjectId)
	if err != nil {
		// Không spam cảnh báo nếu đơn giản là chưa có deployment
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			h.log.Debug().
				Str("correlation_id", correlationID).
				Str("deployment_id", req.DeploymentId).
				Str("project_id", req.ProjectId).
				Msg("Deployment not found (returning empty)")

			return &deploymentpb.GetDeploymentStatusResponse{
				Status: deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED,
				// Error để trống để caller hiểu đây là trạng thái trống
			}, nil
		}

		h.log.Warn().
			Err(err).
			Str("correlation_id", correlationID).
			Str("deployment_id", req.DeploymentId).
			Str("project_id", req.ProjectId).
			Msg("GetDeploymentStatus error")

		return &deploymentpb.GetDeploymentStatusResponse{
			Status: deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_UNSPECIFIED,
			Error:  err.Error(),
		}, nil
	}

	return &deploymentpb.GetDeploymentStatusResponse{
		DeploymentId: deployment.ID,
		ContainerId:  deployment.ContainerID,
		Status:       deployment.Status,
		PublicUrl:    deployment.PublicURL,
		StartedAt:    timestamppb.New(deployment.StartedAt),
	}, nil
}

// RestartDeployment restarts a deployment
func (h *DeploymentHandler) RestartDeployment(ctx context.Context, req *deploymentpb.RestartDeploymentRequest) (*deploymentpb.RestartDeploymentResponse, error) {
	correlationID := logger.GetCorrelationID(ctx)
	h.log.Info().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Str("project_id", req.ProjectId).
		Msg("Restart deployment request received")

	deployment, err := h.executor.Restart(ctx, req.DeploymentId, req.ProjectId)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("correlation_id", correlationID).
			Str("deployment_id", req.DeploymentId).
			Msg("Restart deployment failed")

		return &deploymentpb.RestartDeploymentResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	h.log.Info().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Str("container_id", deployment.ContainerID).
		Msg("Deployment restarted")

	return &deploymentpb.RestartDeploymentResponse{
		Success:     true,
		ContainerId: deployment.ContainerID,
	}, nil
}

// GetRuntimeLogs returns the last N lines of container logs
func (h *DeploymentHandler) GetRuntimeLogs(ctx context.Context, req *deploymentpb.GetRuntimeLogsRequest) (*deploymentpb.GetRuntimeLogsResponse, error) {
	correlationID := logger.GetCorrelationID(ctx)
	h.log.Debug().
		Str("correlation_id", correlationID).
		Str("deployment_id", req.DeploymentId).
		Int32("tail_lines", req.TailLines).
		Msg("Get runtime logs request")

	logs, err := h.executor.GetLogs(ctx, req.DeploymentId, req.ProjectId, req.TailLines)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("correlation_id", correlationID).
			Str("deployment_id", req.DeploymentId).
			Msg("Get runtime logs failed")

		return &deploymentpb.GetRuntimeLogsResponse{
			Error: err.Error(),
		}, nil
	}

	return &deploymentpb.GetRuntimeLogsResponse{
		LogLines: logs,
	}, nil
}

// Helper function to convert status enum to string
func statusToString(status deploymentpb.DeploymentStatus) string {
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
