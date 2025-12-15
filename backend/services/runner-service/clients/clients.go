package clients

import (
	"context"
	"fmt"
	"time"

	grpcpkg "github.com/nexusdeploy/backend/pkg/grpc"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

// Clients holds all gRPC clients needed by Runner Service
type Clients struct {
	Build   buildpb.BuildServiceClient
	Project projectpb.ProjectServiceClient

	buildConn   *grpc.ClientConn
	projectConn *grpc.ClientConn
	log         zerolog.Logger
}

// ClientsConfig holds configuration for gRPC clients
type ClientsConfig struct {
	BuildServiceAddr   string
	ProjectServiceAddr string
	Timeout            time.Duration
	MaxRetries         int
	TLSEnabled         bool
	TLSCertPath        string
	InsecureSkipVerify bool
}

// NewClients creates and connects all gRPC clients
func NewClients(ctx context.Context, cfg ClientsConfig, log zerolog.Logger) (*Clients, error) {
	// Connect to Build Service
	buildConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.BuildServiceAddr,
		Timeout:            cfg.Timeout,
		MaxRetries:         cfg.MaxRetries,
		ServiceName:        "build-service",
		TLSEnabled:         cfg.TLSEnabled,
		TLSCertPath:        cfg.TLSCertPath,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to build service: %w", err)
	}

	// Connect to Project Service
	projectConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.ProjectServiceAddr,
		Timeout:            cfg.Timeout,
		MaxRetries:         cfg.MaxRetries,
		ServiceName:        "project-service",
		TLSEnabled:         cfg.TLSEnabled,
		TLSCertPath:        cfg.TLSCertPath,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
	if err != nil {
		buildConn.Close()
		return nil, fmt.Errorf("connect to project service: %w", err)
	}

	log.Info().
		Str("build_service", cfg.BuildServiceAddr).
		Str("project_service", cfg.ProjectServiceAddr).
		Msg("Connected to gRPC services")

	return &Clients{
		Build:       buildpb.NewBuildServiceClient(buildConn),
		Project:     projectpb.NewProjectServiceClient(projectConn),
		buildConn:   buildConn,
		projectConn: projectConn,
		log:         log,
	}, nil
}

// Close closes all gRPC connections
func (c *Clients) Close() error {
	var errs []error

	if c.buildConn != nil {
		if err := c.buildConn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close build conn: %w", err))
		}
	}

	if c.projectConn != nil {
		if err := c.projectConn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close project conn: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	c.log.Info().Msg("Closed gRPC connections")
	return nil
}

// UpdateBuildStatus updates the build status via Build Service
func (c *Clients) UpdateBuildStatus(ctx context.Context, buildID string, status buildpb.BuildStatus, logLines []string) error {
	resp, err := c.Build.UpdateBuildStatus(ctx, &buildpb.UpdateBuildStatusRequest{
		BuildId:  buildID,
		Status:   status,
		LogLines: logLines,
	})
	if err != nil {
		return fmt.Errorf("update build status: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("build service error: %s", resp.Error)
	}
	return nil
}

// AppendBuildLogs appends logs to a build
func (c *Clients) AppendBuildLogs(ctx context.Context, buildID string, logLines []string) error {
	resp, err := c.Build.AppendBuildLogs(ctx, &buildpb.AppendBuildLogsRequest{
		BuildId:  buildID,
		LogLines: logLines,
	})
	if err != nil {
		return fmt.Errorf("append build logs: %w", err)
	}
	if resp.Error != "" {
		return fmt.Errorf("build service error: %s", resp.Error)
	}
	return nil
}

// GetProjectSecrets fetches decrypted secrets for a project
func (c *Clients) GetProjectSecrets(ctx context.Context, projectID string) (map[string]string, error) {
	resp, err := c.Project.GetSecrets(ctx, &projectpb.GetSecretsRequest{
		ProjectId: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("get secrets: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("project service error: %s", resp.Error)
	}
	return resp.Secrets, nil
}

// GetProject fetches project configuration
func (c *Clients) GetProject(ctx context.Context, projectID, userID string) (*projectpb.Project, error) {
	resp, err := c.Project.GetProject(ctx, &projectpb.GetProjectRequest{
		ProjectId: projectID,
		UserId:    userID,
	})
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("project service error: %s", resp.Error)
	}
	return resp.Project, nil
}
