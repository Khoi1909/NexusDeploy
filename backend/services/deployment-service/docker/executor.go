package docker

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	"github.com/rs/zerolog"
)

// Deployment represents an active deployment
type Deployment struct {
	ID          string
	ProjectID   string
	ContainerID string
	ImageTag    string
	HostPort    int32
	Status      deploymentpb.DeploymentStatus
	PublicURL   string
	StartedAt   time.Time
	Error       string
}

// Executor handles Docker operations for deployments
type Executor struct {
	client              *client.Client
	log                 zerolog.Logger
	traefikNetwork      string
	traefikEntrypoint   string
	traefikDomainSuffix string

	// In-memory store for deployments (MVP)
	deployments map[string]*Deployment
	mu          sync.RWMutex

	// Port allocation
	portRangeStart int32
	portRangeEnd   int32
	usedPorts      map[int32]bool
}

// ExecutorConfig holds configuration for the executor
type ExecutorConfig struct {
	TraefikNetwork      string
	TraefikEntrypoint   string
	TraefikDomainSuffix string
	PortRangeStart      int32
	PortRangeEnd        int32
}

// NewExecutor creates a new Docker executor
func NewExecutor(cfg ExecutorConfig, log zerolog.Logger) (*Executor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	// Ping to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping docker: %w", err)
	}

	log.Info().
		Str("network", cfg.TraefikNetwork).
		Str("entrypoint", cfg.TraefikEntrypoint).
		Str("domain_suffix", cfg.TraefikDomainSuffix).
		Msg("Docker executor initialized")

	// Default port range if not provided
	if cfg.PortRangeStart == 0 && cfg.PortRangeEnd == 0 {
		cfg.PortRangeStart = 12000
		cfg.PortRangeEnd = 12999
	}
	if cfg.PortRangeStart == 0 {
		cfg.PortRangeStart = 12000
	}
	if cfg.PortRangeEnd == 0 {
		cfg.PortRangeEnd = 12999
	}

	executor := &Executor{
		client:              cli,
		log:                 log,
		traefikNetwork:      cfg.TraefikNetwork,
		traefikEntrypoint:   cfg.TraefikEntrypoint,
		traefikDomainSuffix: cfg.TraefikDomainSuffix,
		portRangeStart:      cfg.PortRangeStart,
		portRangeEnd:        cfg.PortRangeEnd,
		deployments:         make(map[string]*Deployment),
		usedPorts:           make(map[int32]bool),
	}

	// Recover existing deployments from Docker containers
	recoverCtx, recoverCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer recoverCancel()
	if err := executor.recoverExistingDeployments(recoverCtx); err != nil {
		log.Warn().Err(err).Msg("Failed to recover existing deployments on startup")
	}

	return executor, nil
}

// Close closes the Docker client
func (e *Executor) Close() error {
	return e.client.Close()
}

// IsHealthy checks if Docker is reachable
func (e *Executor) IsHealthy(ctx context.Context) bool {
	_, err := e.client.Ping(ctx)
	return err == nil
}

// Deploy creates and starts a new container
func (e *Executor) Deploy(ctx context.Context, spec *deploymentpb.DeploymentSpec) (*Deployment, error) {
	deploymentID := uuid.New().String()

	e.log.Info().
		Str("deployment_id", deploymentID).
		Str("project_id", spec.ProjectId).
		Str("image", spec.ImageTag).
		Msg("Starting deployment")

	// Create deployment record
	deployment := &Deployment{
		ID:        deploymentID,
		ProjectID: spec.ProjectId,
		ImageTag:  spec.ImageTag,
		Status:    deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_PENDING,
		StartedAt: time.Now(),
	}

	// Pull the image
	e.log.Debug().Str("image", spec.ImageTag).Msg("Pulling image")
	reader, err := e.client.ImagePull(ctx, spec.ImageTag, image.PullOptions{})
	if err != nil {
		deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_FAILED
		deployment.Error = fmt.Sprintf("pull image: %v", err)
		e.storeDeployment(deployment)
		return deployment, fmt.Errorf("pull image: %w", err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Build container configuration
	containerName := e.getContainerName(spec.ProjectId, deploymentID)
	domain := e.getDomain(spec)

	// Environment variables
	envVars := e.buildEnvVars(spec)

	// Resource limits
	resources := e.buildResourceLimits(spec.Resources)

	// Traefik labels for routing
	labels := e.buildTraefikLabels(containerName, domain, spec.Port)

	// Add Nexus labels for recovery
	nexusLabels := e.buildNexusLabels(spec.ProjectId, deploymentID, domain)
	for k, v := range nexusLabels {
		labels[k] = v
	}

	// Port bindings
	portStr := fmt.Sprintf("%d/tcp", spec.Port)
	exposedPorts := nat.PortSet{
		nat.Port(portStr): struct{}{},
	}

	// Allocate host port from configured range
	hostPort, err := e.allocatePort()
	if err != nil {
		deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_FAILED
		deployment.Error = fmt.Sprintf("allocate port: %v", err)
		e.storeDeployment(deployment)
		return deployment, fmt.Errorf("allocate port: %w", err)
	}
	hostPortStr := fmt.Sprintf("%d", hostPort)

	portBindings := nat.PortMap{
		nat.Port(portStr): []nat.PortBinding{{
			HostIP:   "0.0.0.0",
			HostPort: hostPortStr,
		}},
	}

	// Create container
	resp, err := e.client.ContainerCreate(ctx,
		&container.Config{
			Image:        spec.ImageTag,
			Env:          envVars,
			ExposedPorts: exposedPorts,
			Labels:       labels,
		},
		&container.HostConfig{
			Resources:     resources,
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			NetworkMode:   container.NetworkMode(e.traefikNetwork),
			PortBindings:  portBindings,
		},
		&network.NetworkingConfig{},
		nil,
		containerName,
	)
	if err != nil {
		e.releasePort(hostPort)
		deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_FAILED
		deployment.Error = fmt.Sprintf("create container: %v", err)
		e.storeDeployment(deployment)
		return deployment, fmt.Errorf("create container: %w", err)
	}

	deployment.ContainerID = resp.ID
	deployment.HostPort = hostPort

	// Start container
	if err := e.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Cleanup failed container
		e.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		e.releasePort(hostPort)
		deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_FAILED
		deployment.Error = fmt.Sprintf("start container: %v", err)
		e.storeDeployment(deployment)
		return deployment, fmt.Errorf("start container: %w", err)
	}

	// Wait for container to be healthy
	time.Sleep(2 * time.Second) // Give container time to start

	// Update status
	deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING
	deployment.PublicURL = fmt.Sprintf("https://%s", domain)
	e.storeDeployment(deployment)

	e.log.Info().
		Str("deployment_id", deploymentID).
		Str("container_id", resp.ID).
		Str("public_url", deployment.PublicURL).
		Msg("Deployment successful")

	return deployment, nil
}

// Stop stops and removes a deployment
func (e *Executor) Stop(ctx context.Context, deploymentID, projectID string) error {
	e.mu.RLock()
	deployment, exists := e.deployments[deploymentID]
	e.mu.RUnlock()

	if !exists {
		// Try to find by project ID
		e.mu.RLock()
		for _, d := range e.deployments {
			if d.ProjectID == projectID {
				deployment = d
				break
			}
		}
		e.mu.RUnlock()
	}

	if deployment == nil || deployment.ContainerID == "" {
		return fmt.Errorf("deployment not found")
	}

	e.log.Info().
		Str("deployment_id", deploymentID).
		Str("container_id", deployment.ContainerID).
		Msg("Stopping deployment")

	// Stop container with timeout
	timeout := 30
	if err := e.client.ContainerStop(ctx, deployment.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		e.log.Warn().Err(err).Msg("Failed to stop container gracefully")
	}

	// Remove container
	if err := e.client.ContainerRemove(ctx, deployment.ContainerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}

	// Release allocated port
	if deployment.HostPort > 0 {
		e.releasePort(deployment.HostPort)
	}

	// Update status
	e.mu.Lock()
	deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_STOPPED
	e.mu.Unlock()

	e.log.Info().Str("deployment_id", deploymentID).Msg("Deployment stopped")
	return nil
}

// GetStatus returns the status of a deployment
func (e *Executor) GetStatus(ctx context.Context, deploymentID, projectID string) (*Deployment, error) {
	e.mu.RLock()
	deployment, exists := e.deployments[deploymentID]
	if !exists {
		// Try to find by project ID
		for _, d := range e.deployments {
			if d.ProjectID == projectID {
				deployment = d
				break
			}
		}
	}
	e.mu.RUnlock()

	// If not found in memory, try to recover from Docker containers
	if deployment == nil {
		recovered, err := e.recoverDeploymentFromContainer(ctx, projectID)
		if err == nil && recovered != nil {
			e.mu.Lock()
			e.deployments[recovered.ID] = recovered
			e.mu.Unlock()
			deployment = recovered
		} else {
			return nil, fmt.Errorf("deployment not found")
		}
	}

	// Update status from Docker
	if deployment.ContainerID != "" {
		inspect, err := e.client.ContainerInspect(ctx, deployment.ContainerID)
		if err == nil {
			if inspect.State.Running {
				deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING
			} else if inspect.State.Restarting {
				deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RESTARTING
			} else {
				deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_STOPPED
			}
			// Update container ID if it changed
			if inspect.ID != deployment.ContainerID {
				deployment.ContainerID = inspect.ID
			}
		}
	}

	return deployment, nil
}

// recoverDeploymentFromContainer tries to find a running container for the project and restore deployment state
func (e *Executor) recoverDeploymentFromContainer(ctx context.Context, projectID string) (*Deployment, error) {
	e.log.Debug().
		Str("project_id", projectID).
		Msg("Attempting to recover deployment from container")

	// List all containers with nexus project label
	containers, err := e.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", fmt.Sprintf("nexus.project_id=%s", projectID)),
		),
	})
	if err != nil {
		e.log.Warn().Err(err).Str("project_id", projectID).Msg("Failed to list containers for recovery")
		return nil, fmt.Errorf("list containers: %w", err)
	}

	e.log.Debug().
		Str("project_id", projectID).
		Int("container_count", len(containers)).
		Msg("Found containers for recovery")

	// Find the first running container for this project
	for _, c := range containers {
		// Check if container is running or restarting
		if c.State == "running" || c.State == "restarting" {
			// Inspect container to get full details
			inspect, err := e.client.ContainerInspect(ctx, c.ID)
			if err != nil {
				continue
			}

			// Extract deployment info from labels
			deploymentID := inspect.Config.Labels["nexus.deployment_id"]
			if deploymentID == "" {
				// Generate a new deployment ID if not found
				deploymentID = uuid.New().String()
			}

			imageTag := inspect.Config.Image
			publicURL := ""
			var hostPort int32

			// Try to get domain from nexus.domain label first
			if domain, ok := inspect.Config.Labels["nexus.domain"]; ok {
				publicURL = fmt.Sprintf("http://%s", domain)
			} else {
				// Try to extract public URL from Traefik labels
				// Find router name from container name
				routerName := strings.ReplaceAll(c.Names[0][1:], "-", "_")
				if rule, ok := inspect.Config.Labels[fmt.Sprintf("traefik.http.routers.%s.rule", routerName)]; ok {
					// Extract domain from rule like "Host(`project-id.localhost`)"
					if strings.Contains(rule, "Host(`") {
						start := strings.Index(rule, "Host(`") + 6
						end := strings.Index(rule[start:], "`)")
						if end > 0 {
							publicURL = fmt.Sprintf("http://%s", rule[start:start+end])
						}
					}
				}
			}

			// Try to capture host port from port bindings
			for _, bindings := range inspect.NetworkSettings.Ports {
				if len(bindings) > 0 && len(bindings[0].HostPort) > 0 {
					if hp, err := nat.ParsePort(bindings[0].HostPort); err == nil {
						hostPort = int32(hp)
						break
					}
				}
			}

			// Determine status
			status := deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_STOPPED
			if inspect.State.Running {
				status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING
			} else if inspect.State.Restarting {
				status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RESTARTING
			}

			// Parse Created time (format: "2024-01-01T00:00:00.000000000Z")
			startedAt := time.Now()
			if created, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
				startedAt = created
			}

			deployment := &Deployment{
				ID:          deploymentID,
				ProjectID:   projectID,
				ContainerID: c.ID,
				ImageTag:    imageTag,
				HostPort:    hostPort,
				Status:      status,
				PublicURL:   publicURL,
				StartedAt:   startedAt,
			}

			e.log.Info().
				Str("deployment_id", deploymentID).
				Str("project_id", projectID).
				Str("container_id", c.ID).
				Msg("Recovered deployment from container")

			// Mark host port as used
			if hostPort > 0 {
				e.mu.Lock()
				e.usedPorts[hostPort] = true
				e.mu.Unlock()
			}

			return deployment, nil
		}
	}

	e.log.Debug().
		Str("project_id", projectID).
		Msg("No running container found for recovery")
	return nil, fmt.Errorf("no running container found for project")
}

// recoverExistingDeployments recovers all existing deployments from Docker containers on startup
func (e *Executor) recoverExistingDeployments(ctx context.Context) error {
	// List all containers with nexus.project_id label
	containers, err := e.client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "nexus.project_id"),
		),
	})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	recoveredCount := 0
	projectIDs := make(map[string]bool)

	// Group containers by project ID
	for _, c := range containers {
		// Inspect to get labels
		inspect, err := e.client.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		projectID := inspect.Config.Labels["nexus.project_id"]
		if projectID == "" {
			continue
		}

		// Only recover once per project (get the latest running container)
		if projectIDs[projectID] {
			continue
		}

		// Try to recover this project's deployment
		recovered, err := e.recoverDeploymentFromContainer(ctx, projectID)
		if err == nil && recovered != nil {
			e.mu.Lock()
			e.deployments[recovered.ID] = recovered
			e.mu.Unlock()
			projectIDs[projectID] = true
			recoveredCount++
			e.log.Info().
				Str("deployment_id", recovered.ID).
				Str("project_id", projectID).
				Str("container_id", recovered.ContainerID).
				Msg("Recovered deployment on startup")
		}
	}

	e.log.Info().
		Int("count", recoveredCount).
		Msg("Recovery completed")

	return nil
}

// allocatePort finds a free host port in configured range
func (e *Executor) allocatePort() (int32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for p := e.portRangeStart; p <= e.portRangeEnd; p++ {
		if !e.usedPorts[p] {
			e.usedPorts[p] = true
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", e.portRangeStart, e.portRangeEnd)
}

// releasePort frees a previously allocated port
func (e *Executor) releasePort(port int32) {
	if port <= 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.usedPorts, port)
}

// Restart restarts a deployment
func (e *Executor) Restart(ctx context.Context, deploymentID, projectID string) (*Deployment, error) {
	deployment, err := e.GetStatus(ctx, deploymentID, projectID)
	if err != nil {
		return nil, err
	}

	if deployment.ContainerID == "" {
		return nil, fmt.Errorf("no container to restart")
	}

	e.log.Info().
		Str("deployment_id", deploymentID).
		Str("container_id", deployment.ContainerID).
		Msg("Restarting deployment")

	timeout := 10
	if err := e.client.ContainerRestart(ctx, deployment.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return nil, fmt.Errorf("restart container: %w", err)
	}

	deployment.Status = deploymentpb.DeploymentStatus_DEPLOYMENT_STATUS_RUNNING
	return deployment, nil
}

// GetLogs returns the last N lines of container logs
func (e *Executor) GetLogs(ctx context.Context, deploymentID, projectID string, tailLines int32) ([]string, error) {
	deployment, err := e.GetStatus(ctx, deploymentID, projectID)
	if err != nil {
		return nil, err
	}

	if deployment.ContainerID == "" {
		return nil, fmt.Errorf("no container for logs")
	}

	tail := "100"
	if tailLines > 0 {
		tail = fmt.Sprintf("%d", tailLines)
	}

	reader, err := e.client.ContainerLogs(ctx, deployment.ContainerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		return nil, fmt.Errorf("get container logs: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read logs: %w", err)
	}

	// Parse Docker log format (skip 8-byte header per line)
	lines := strings.Split(string(data), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 8 {
			result = append(result, line[8:])
		} else if len(line) > 0 {
			result = append(result, line)
		}
	}

	return result, nil
}

// Helper methods

func (e *Executor) storeDeployment(d *Deployment) {
	e.mu.Lock()
	e.deployments[d.ID] = d
	e.mu.Unlock()
}

func (e *Executor) getContainerName(projectID, deploymentID string) string {
	// Shorten IDs for cleaner names
	shortProject := projectID
	if len(shortProject) > 8 {
		shortProject = shortProject[:8]
	}
	shortDeploy := deploymentID
	if len(shortDeploy) > 8 {
		shortDeploy = shortDeploy[:8]
	}
	return fmt.Sprintf("nexus-app-%s-%s", shortProject, shortDeploy)
}

func (e *Executor) getDomain(spec *deploymentpb.DeploymentSpec) string {
	if spec.Domain != "" {
		return spec.Domain
	}
	// Generate subdomain from project ID
	shortID := spec.ProjectId
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("%s.%s", shortID, e.traefikDomainSuffix)
}

func (e *Executor) buildEnvVars(spec *deploymentpb.DeploymentSpec) []string {
	envVars := make([]string, 0)

	// Add regular env vars
	for k, v := range spec.EnvVars {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Add secrets
	for k, v := range spec.Secrets {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Add PORT env var
	envVars = append(envVars, fmt.Sprintf("PORT=%d", spec.Port))

	return envVars
}

func (e *Executor) buildResourceLimits(limits *deploymentpb.ResourceLimits) container.Resources {
	resources := container.Resources{}

	if limits == nil {
		// Default limits for standard plan
		resources.Memory = 512 * 1024 * 1024 // 512MB
		resources.NanoCPUs = 1000000000      // 1 CPU
		return resources
	}

	if limits.MemoryMb > 0 {
		resources.Memory = limits.MemoryMb * 1024 * 1024
	} else {
		resources.Memory = 512 * 1024 * 1024
	}

	if limits.CpuCores > 0 {
		resources.NanoCPUs = int64(limits.CpuCores) * 1000000000
	} else {
		resources.NanoCPUs = 1000000000
	}

	return resources
}

func (e *Executor) buildTraefikLabels(containerName, domain string, port int32) map[string]string {
	routerName := strings.ReplaceAll(containerName, "-", "_")

	return map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", routerName):                      fmt.Sprintf("Host(`%s`)", domain),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName):               e.traefikEntrypoint,
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName): fmt.Sprintf("%d", port),
		// Add network
		"traefik.docker.network": e.traefikNetwork,
		// NexusDeploy metadata
		"io.nexusdeploy.managed": "true",
		"io.nexusdeploy.domain":  domain,
	}
}

func (e *Executor) buildNexusLabels(projectID, deploymentID, domain string) map[string]string {
	return map[string]string{
		"nexus.project_id":       projectID,
		"nexus.deployment_id":    deploymentID,
		"nexus.domain":           domain,
		"io.nexusdeploy.managed": "true",
	}
}
