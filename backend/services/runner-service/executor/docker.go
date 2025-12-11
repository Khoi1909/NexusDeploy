package executor

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog"
)

// DockerExecutor handles Docker operations for CI/CD
type DockerExecutor struct {
	client       *client.Client
	log          zerolog.Logger
	registryURL  string
	registryUser string
	registryPass string
	workDir      string
}

// ExecutorConfig holds configuration for the executor
type ExecutorConfig struct {
	RegistryURL  string
	RegistryUser string
	RegistryPass string
	WorkDir      string // Base directory for build workspaces
}

// NewDockerExecutor creates a new Docker executor
func NewDockerExecutor(cfg ExecutorConfig, log zerolog.Logger) (*DockerExecutor, error) {
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

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "/tmp/nexus-builds"
	}

	// Ensure work directory exists
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}

	log.Info().
		Str("registry", cfg.RegistryURL).
		Str("work_dir", workDir).
		Msg("Docker executor initialized")

	return &DockerExecutor{
		client:       cli,
		log:          log,
		registryURL:  cfg.RegistryURL,
		registryUser: cfg.RegistryUser,
		registryPass: cfg.RegistryPass,
		workDir:      workDir,
	}, nil
}

// Close closes the Docker client
func (e *DockerExecutor) Close() error {
	return e.client.Close()
}

// BuildContext holds all information needed for a build
type BuildContext struct {
	BuildID      string
	ProjectID    string
	RepoURL      string
	Branch       string
	CommitSHA    string
	BuildCommand string
	StartCommand string
	Preset       string
	Port         int
	Secrets      map[string]string
	GitHubToken  string // For private repos
}

// BuildResult contains the result of a build
type BuildResult struct {
	ImageTag string
	Logs     []string
	Success  bool
	Error    error
	Duration time.Duration
	WorkDir  string
}

// LogCallback is called for each log line
type LogCallback func(line string)

// CloneRepository clones the repository to a workspace
func (e *DockerExecutor) CloneRepository(ctx context.Context, bc *BuildContext, logCb LogCallback) (string, error) {
	// Create workspace directory
	workspace := filepath.Join(e.workDir, bc.BuildID)
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}

	logCb(fmt.Sprintf("[clone] Creating workspace: %s", workspace))

	// Build git clone URL with token for private repos
	repoURL := bc.RepoURL
	if bc.GitHubToken != "" && strings.Contains(repoURL, "github.com") {
		// Insert token into HTTPS URL
		repoURL = strings.Replace(repoURL, "https://github.com",
			fmt.Sprintf("https://%s@github.com", bc.GitHubToken), 1)
	}

	// Clone repository directly into workspace using git clone with . as target
	// This avoids creating a subdirectory
	logCb(fmt.Sprintf("[clone] Cloning %s branch %s", bc.RepoURL, bc.Branch))

	// Use git clone directly into workspace directory
	cloneArgs := []string{"clone", "--depth", "1", "--branch", bc.Branch, repoURL, "."}
	cloneCmd := exec.CommandContext(ctx, "git", cloneArgs...)
	cloneCmd.Dir = workspace
	output, err := cloneCmd.CombinedOutput()
	targetBranch := bc.Branch

	if err != nil {
		// Try default branch (usually main or master)
		logCb(fmt.Sprintf("[clone] Branch %s not found, trying default branch...", bc.Branch))
		for _, defaultBranch := range []string{"main", "master"} {
			cloneArgs = []string{"clone", "--depth", "1", "--branch", defaultBranch, repoURL, "."}
			cloneCmd = exec.CommandContext(ctx, "git", cloneArgs...)
			cloneCmd.Dir = workspace
			output, err = cloneCmd.CombinedOutput()
			if err == nil {
				targetBranch = defaultBranch
				logCb(fmt.Sprintf("[clone] Cloned default branch: %s", defaultBranch))
				break
			}
		}
		if err != nil {
			// If branch-specific clone fails, try without branch (will get default)
			logCb("[clone] Branch-specific clone failed, trying without branch...")
			cloneArgs = []string{"clone", "--depth", "1", repoURL, "."}
			cloneCmd = exec.CommandContext(ctx, "git", cloneArgs...)
			cloneCmd.Dir = workspace
			output, err = cloneCmd.CombinedOutput()
			if err != nil {
				logCb(fmt.Sprintf("[clone] Error: %s", string(output)))
				return "", fmt.Errorf("git clone: %w", err)
			}
			// Determine which branch was cloned
			branchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
			branchCmd.Dir = workspace
			if branchOutput, err := branchCmd.Output(); err == nil {
				targetBranch = strings.TrimSpace(string(branchOutput))
				logCb("[clone] Cloned branch: " + targetBranch)
			}
		}
	}

	if len(output) > 0 {
		logCb(fmt.Sprintf("[clone] %s", strings.TrimSpace(string(output))))
	}
	logCb(fmt.Sprintf("[clone] Checked out branch: %s", targetBranch))

	// Checkout specific commit if provided
	if bc.CommitSHA != "" {
		logCb(fmt.Sprintf("[clone] Checking out commit %s", bc.CommitSHA))
		checkoutCmd := exec.CommandContext(ctx, "git", "checkout", bc.CommitSHA)
		checkoutCmd.Dir = workspace
		if output, err := checkoutCmd.CombinedOutput(); err != nil {
			logCb(fmt.Sprintf("[clone] Checkout error: %s", string(output)))
			// Non-fatal, continue with branch HEAD
		}
	}

	// Verify package.json exists (for debugging)
	if _, err := os.Stat(filepath.Join(workspace, "package.json")); err == nil {
		logCb("[clone] Verified: package.json found in workspace")
	} else {
		logCb("[clone] Warning: package.json not found in workspace (this may cause build to fail)")
	}

	logCb(fmt.Sprintf("[clone] Successfully cloned to %s", workspace))

	return workspace, nil
}

// RunBuildCommand executes the build command in a container
func (e *DockerExecutor) RunBuildCommand(ctx context.Context, bc *BuildContext, workspace string, logCb LogCallback) error {
	if bc.BuildCommand == "" {
		logCb("[build] No build command specified, skipping")
		return nil
	}

	// Verify workspace has files before mounting
	if files, err := os.ReadDir(workspace); err != nil {
		return fmt.Errorf("workspace directory not accessible: %w", err)
	} else if len(files) == 0 {
		return fmt.Errorf("workspace directory is empty")
	}

	// Check for package.json specifically
	packageJsonPath := filepath.Join(workspace, "package.json")
	if _, err := os.Stat(packageJsonPath); err != nil {
		logCb(fmt.Sprintf("[build] Warning: package.json not found at %s", packageJsonPath))
		// List files for debugging
		if files, err := os.ReadDir(workspace); err == nil {
			logCb(fmt.Sprintf("[build] Workspace contains %d items", len(files)))
			for i, file := range files {
				if i < 5 { // Show first 5 files
					logCb(fmt.Sprintf("[build]   - %s", file.Name()))
				}
			}
		}
	} else {
		logCb(fmt.Sprintf("[build] Verified: package.json exists at %s", packageJsonPath))
	}

	logCb(fmt.Sprintf("[build] Running: %s", bc.BuildCommand))
	logCb(fmt.Sprintf("[build] Mounting workspace: %s -> /app", workspace))

	// Get base image based on preset
	baseImage := e.getBaseImage(bc.Preset)
	logCb(fmt.Sprintf("[build] Using base image: %s", baseImage))

	// Pull the base image
	reader, err := e.client.ImagePull(ctx, baseImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Build environment variables
	envVars := make([]string, 0, len(bc.Secrets)+2)
	for k, v := range bc.Secrets {
		envVars = append(envVars, fmt.Sprintf("%s=%s", k, v))
	}

	// Add Node.js memory limit for Node.js projects
	if strings.ToLower(bc.Preset) == "nodejs" || strings.ToLower(bc.Preset) == "node" {
		envVars = append(envVars, "NODE_OPTIONS=--max-old-space-size=4096")
	}

	// Create container and copy workspace files
	// Since workspace is in a named volume, we need to copy files into the container
	resp, err := e.client.ContainerCreate(ctx,
		&container.Config{
			Image:      baseImage,
			Cmd:        []string{"sh", "-c", "sleep infinity"}, // Keep running to copy files
			WorkingDir: "/app",
			Env:        envVars,
		},
		&container.HostConfig{
			Resources: container.Resources{
				Memory:   4 * 1024 * 1024 * 1024, // 4GB (increased for Node.js builds)
				NanoCPUs: 1000000000,             // 1 CPU
			},
		},
		nil, nil,
		fmt.Sprintf("nexus-build-%s", bc.BuildID),
	)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}

	containerID := resp.ID

	// Start container to copy files
	if err := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return fmt.Errorf("start container: %w", err)
	}

	// Copy workspace files to container
	logCb("[build] Copying workspace files to container...")
	if err := e.copyWorkspaceToContainer(ctx, containerID, workspace, logCb); err != nil {
		e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return fmt.Errorf("copy workspace to container: %w", err)
	}

	// Install dependencies before build
	installCmd := e.getInstallCommand(bc.Preset, workspace)
	if installCmd != "" {
		logCb(fmt.Sprintf("[build] Installing dependencies: %s", installCmd))
		// Exec install command in running container
		execResp, err := e.client.ContainerExecCreate(ctx, containerID, types.ExecConfig{
			Cmd:          []string{"sh", "-c", installCmd},
			WorkingDir:   "/app",
			Env:          envVars,
			AttachStdout: true,
			AttachStderr: true,
		})
		if err != nil {
			e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			return fmt.Errorf("create exec for install: %w", err)
		}

		// Attach to exec and stream logs
		attachResp, err := e.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
		if err != nil {
			e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			return fmt.Errorf("attach exec for install: %w", err)
		}
		defer attachResp.Close()

		// Stream install logs
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := attachResp.Reader.Read(buf)
				if err != nil {
					break
				}
				if n > 0 {
					start := 0
					if n > 8 {
						start = 8
					}
					line := strings.TrimSpace(string(buf[start:n]))
					if line != "" {
						logCb(line)
					}
				}
			}
		}()

		// Wait for install to complete
		execInspect, err := e.client.ContainerExecInspect(ctx, execResp.ID)
		for execInspect.Running {
			time.Sleep(100 * time.Millisecond)
			execInspect, err = e.client.ContainerExecInspect(ctx, execResp.ID)
			if err != nil {
				break
			}
		}

		if execInspect.ExitCode != 0 {
			e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
			return fmt.Errorf("install dependencies failed with exit code %d", execInspect.ExitCode)
		}
		logCb("[build] Dependencies installed successfully")
	}

	// Stop container temporarily to update command
	if err := e.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		logCb(fmt.Sprintf("[build] Warning: failed to stop container: %v", err))
	}

	// Commit container with files and installed dependencies to a temporary image
	tempImageTag := fmt.Sprintf("nexus-build-temp-%s:latest", bc.BuildID)
	commitResp, err := e.client.ContainerCommit(ctx, containerID, container.CommitOptions{
		Reference: tempImageTag,
		Config: &container.Config{
			Cmd:        []string{"sh", "-c", bc.BuildCommand},
			WorkingDir: "/app",
			Env:        envVars,
		},
	})
	if err != nil {
		e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return fmt.Errorf("commit container: %w", err)
	}

	// Remove old container
	e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})

	// Create final container from committed image
	resp, err = e.client.ContainerCreate(ctx,
		&container.Config{
			Image:      commitResp.ID,
			Cmd:        []string{"sh", "-c", bc.BuildCommand},
			WorkingDir: "/app",
			Env:        envVars,
		},
		&container.HostConfig{
			Resources: container.Resources{
				Memory:   512 * 1024 * 1024,
				NanoCPUs: 1000000000,
			},
		},
		nil, nil,
		fmt.Sprintf("nexus-build-%s", bc.BuildID),
	)
	if err != nil {
		return fmt.Errorf("create build container: %w", err)
	}

	containerID = resp.ID
	defer func() {
		// Cleanup container
		e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	}()

	// Start container
	if err := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	// Stream logs
	go e.streamContainerLogs(ctx, containerID, logCb)

	// Wait for container to finish
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("container wait: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("build failed with exit code %d", status.StatusCode)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	logCb("[build] Build completed successfully")
	return nil
}

// copyWorkspaceToContainer copies workspace files to container using tar archive
func (e *DockerExecutor) copyWorkspaceToContainer(ctx context.Context, containerID string, workspace string, logCb LogCallback) error {
	// Create tar archive of workspace
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Get relative path from workspace
		relPath, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Write file content if not directory
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("create tar archive: %w", err)
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	// Copy tar archive to container
	if err := e.client.CopyToContainer(ctx, containerID, "/app", &buf, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}

	logCb("[build] Workspace files copied to container")
	return nil
}

// getInstallCommand returns the install command for a preset
func (e *DockerExecutor) getInstallCommand(preset string, workspace string) string {
	preset = strings.ToLower(preset)

	switch preset {
	case "nodejs", "node":
		// Check if package.json exists
		if _, err := os.Stat(filepath.Join(workspace, "package.json")); err == nil {
			return "npm install"
		}
	case "python":
		// Check if requirements.txt exists
		if _, err := os.Stat(filepath.Join(workspace, "requirements.txt")); err == nil {
			return "pip install --no-cache-dir -r requirements.txt"
		}
		// Check if pyproject.toml exists (for poetry)
		if _, err := os.Stat(filepath.Join(workspace, "pyproject.toml")); err == nil {
			return "pip install poetry && poetry install --no-dev"
		}
	case "go", "golang":
		// Check if go.mod exists
		if _, err := os.Stat(filepath.Join(workspace, "go.mod")); err == nil {
			return "go mod download"
		}
	case "ruby":
		// Check if Gemfile exists
		if _, err := os.Stat(filepath.Join(workspace, "Gemfile")); err == nil {
			return "bundle install"
		}
	case "java":
		// Check if pom.xml exists (Maven)
		if _, err := os.Stat(filepath.Join(workspace, "pom.xml")); err == nil {
			return "mvn dependency:resolve"
		}
		// Check if build.gradle exists (Gradle)
		if _, err := os.Stat(filepath.Join(workspace, "build.gradle")); err == nil {
			return "gradle dependencies"
		}
	}

	return "" // No install command needed
}

// BuildDockerImage builds a Docker image for the application
func (e *DockerExecutor) BuildDockerImage(ctx context.Context, bc *BuildContext, workspace string, logCb LogCallback) (string, error) {
	imageTag := e.getImageTag(bc)
	logCb(fmt.Sprintf("[docker] Building image: %s", imageTag))

	// Check if Dockerfile exists, otherwise create one based on preset
	dockerfilePath := filepath.Join(workspace, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logCb("[docker] No Dockerfile found, generating from preset")
		dockerfile := e.generateDockerfile(bc)
		if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
			return "", fmt.Errorf("write dockerfile: %w", err)
		}
	}

	// Build using docker build command (simpler than Docker SDK for buildkit)
	args := []string{"build", "-t", imageTag, workspace}
	cmd := exec.CommandContext(ctx, "docker", args...)

	output, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(output), "\n") {
		if line != "" {
			logCb(fmt.Sprintf("[docker] %s", line))
		}
	}
	if err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}

	logCb(fmt.Sprintf("[docker] Image built successfully: %s", imageTag))
	return imageTag, nil
}

// PushImage pushes the image to the registry
func (e *DockerExecutor) PushImage(ctx context.Context, imageTag string, logCb LogCallback) error {
	// Skip push if no registry configured
	if e.registryURL == "" {
		logCb("[push] No registry configured, skipping push")
		logCb("[push] Image available locally: " + imageTag)
		return nil
	}

	logCb(fmt.Sprintf("[push] Pushing image: %s", imageTag))

	// Login to registry if credentials provided
	if e.registryUser != "" && e.registryPass != "" {
		logCb("[push] Logging in to registry...")
		loginCmd := exec.CommandContext(ctx, "docker", "login", "-u", e.registryUser, "-p", e.registryPass, e.registryURL)
		if output, err := loginCmd.CombinedOutput(); err != nil {
			logCb(fmt.Sprintf("[push] Warning: Failed to login to registry: %s", string(output)))
			logCb("[push] Attempting push without login (may fail if authentication required)")
		} else {
			logCb("[push] Successfully logged in to registry")
		}
	}

	// Push using docker push command
	cmd := exec.CommandContext(ctx, "docker", "push", imageTag)
	output, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(output), "\n") {
		if line != "" {
			logCb(fmt.Sprintf("[push] %s", line))
		}
	}
	if err != nil {
		// Check if it's an authentication error
		outputStr := string(output)
		if strings.Contains(outputStr, "denied") || strings.Contains(outputStr, "unauthorized") || strings.Contains(outputStr, "authentication required") {
			logCb("[push] Warning: Push failed due to authentication/authorization")
			logCb("[push] Image is available locally but not pushed to registry")
			logCb("[push] Build will continue - image can be used for local deployment")
			return nil // Non-fatal - allow build to succeed
		}
		return fmt.Errorf("docker push: %w", err)
	}

	logCb("[push] Image pushed successfully")
	return nil
}

// Cleanup removes the workspace directory
func (e *DockerExecutor) Cleanup(workspace string) error {
	return os.RemoveAll(workspace)
}

// CleanupWorkspaces removes workspace directories for given build IDs
func (e *DockerExecutor) CleanupWorkspaces(buildIDs []string) error {
	for _, buildID := range buildIDs {
		workspace := filepath.Join(e.workDir, buildID)
		if err := os.RemoveAll(workspace); err != nil {
			e.log.Warn().
				Str("build_id", buildID).
				Str("workspace", workspace).
				Err(err).
				Msg("Failed to cleanup workspace")
			// Continue with other workspaces
		} else {
			e.log.Info().
				Str("build_id", buildID).
				Str("workspace", workspace).
				Msg("Workspace cleaned up")
		}
	}
	return nil
}

// streamContainerLogs streams container logs to the callback
func (e *DockerExecutor) streamContainerLogs(ctx context.Context, containerID string, logCb LogCallback) {
	reader, err := e.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		e.log.Error().Err(err).Msg("Failed to get container logs")
		return
	}
	defer reader.Close()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			break
		}
		if n > 0 {
			// Skip Docker stream header (8 bytes)
			start := 0
			if n > 8 {
				start = 8
			}
			line := strings.TrimSpace(string(buf[start:n]))
			if line != "" {
				logCb(line)
			}
		}
	}
}

// getBaseImage returns the base image for a preset
func (e *DockerExecutor) getBaseImage(preset string) string {
	switch strings.ToLower(preset) {
	case "nodejs", "node":
		return "node:20-alpine"
	case "go", "golang":
		return "golang:1.24-alpine"
	case "python":
		return "python:3.12-alpine"
	case "ruby":
		return "ruby:3.3-alpine"
	case "java":
		return "eclipse-temurin:21-alpine"
	default:
		return "alpine:latest"
	}
}

// getImageTag generates the Docker image tag
func (e *DockerExecutor) getImageTag(bc *BuildContext) string {
	// Use BuildID as tag if CommitSHA is empty
	tag := bc.BuildID
	if bc.CommitSHA != "" {
		shortSHA := bc.CommitSHA
		if len(shortSHA) > 8 {
			shortSHA = shortSHA[:8]
		}
		tag = shortSHA
	} else {
		// Use first 8 chars of BuildID as tag
		if len(bc.BuildID) > 8 {
			tag = bc.BuildID[:8]
		}
	}

	if e.registryURL != "" {
		return fmt.Sprintf("%s/%s:%s", e.registryURL, bc.ProjectID, tag)
	}
	return fmt.Sprintf("nexus/%s:%s", bc.ProjectID, tag)
}

// generateDockerfile creates a Dockerfile based on preset
func (e *DockerExecutor) generateDockerfile(bc *BuildContext) string {
	baseImage := e.getBaseImage(bc.Preset)
	preset := strings.ToLower(bc.Preset)
	port := bc.Port
	if port == 0 {
		port = 8080
	}

	// Determine build and start commands
	buildCmd := bc.BuildCommand
	startCmd := bc.StartCommand

	var dockerfile strings.Builder

	// FROM
	dockerfile.WriteString(fmt.Sprintf("FROM %s AS builder\n", baseImage))
	dockerfile.WriteString("WORKDIR /app\n")

	// Copy package files first (for better caching)
	switch preset {
	case "nodejs", "node":
		dockerfile.WriteString("COPY package*.json ./\n")
		dockerfile.WriteString("RUN npm ci --only=production || npm install --production\n")
	case "python":
		dockerfile.WriteString("COPY requirements.txt ./\n")
		dockerfile.WriteString("RUN pip install --no-cache-dir -r requirements.txt || true\n")
	case "go", "golang":
		dockerfile.WriteString("COPY go.mod go.sum ./\n")
		dockerfile.WriteString("RUN go mod download || true\n")
	}

	// Copy source code
	dockerfile.WriteString("COPY . .\n")

	// Build step
	if buildCmd != "" {
		dockerfile.WriteString(fmt.Sprintf("RUN %s\n", buildCmd))
	} else {
		// Default build commands based on preset
		switch preset {
		case "nodejs", "node":
			dockerfile.WriteString("RUN npm run build || true\n")
		case "go", "golang":
			dockerfile.WriteString("RUN go build -o app . || true\n")
		case "python":
			// Python usually doesn't need build step
		}
	}

	// Runtime stage
	dockerfile.WriteString(fmt.Sprintf("\nFROM %s\n", baseImage))
	dockerfile.WriteString("WORKDIR /app\n")

	// Copy built artifacts from builder
	switch preset {
	case "nodejs", "node":
		// For Node.js, copy everything (including node_modules if not in .dockerignore)
		dockerfile.WriteString("COPY --from=builder /app ./\n")
	case "go", "golang":
		// For Go, copy the binary and any other files needed
		dockerfile.WriteString("COPY --from=builder /app/app ./app\n")
		// Copy other files if needed (config files, etc.)
		dockerfile.WriteString("COPY --from=builder /app ./\n")
	case "python":
		// For Python, copy everything
		dockerfile.WriteString("COPY --from=builder /app ./\n")
	default:
		// Default: copy everything
		dockerfile.WriteString("COPY --from=builder /app ./\n")
	}

	// Expose port
	dockerfile.WriteString(fmt.Sprintf("EXPOSE %d\n", port))

	// Start command
	if startCmd == "" {
		switch preset {
		case "nodejs", "node":
			startCmd = "npm start"
		case "go", "golang":
			startCmd = "./app"
		case "python":
			startCmd = "python main.py"
		default:
			startCmd = "./start.sh"
		}
	}

	// Use shell form for CMD to support environment variables and commands with arguments
	// Escape quotes if needed
	startCmdEscaped := strings.ReplaceAll(startCmd, `"`, `\"`)
	dockerfile.WriteString(fmt.Sprintf("CMD [\"sh\", \"-c\", \"%s\"]\n", startCmdEscaped))

	return dockerfile.String()
}
