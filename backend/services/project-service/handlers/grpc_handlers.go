package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/pkg/crypto"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/nexusdeploy/backend/services/project-service/github"
	"github.com/nexusdeploy/backend/services/project-service/models"
	pb "github.com/nexusdeploy/backend/services/project-service/proto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

var log zerolog.Logger

func init() {
	log = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "project-service").
		Logger()
}

// ProjectServiceServer implements the ProjectService gRPC server
type ProjectServiceServer struct {
	pb.UnimplementedProjectServiceServer
	db           *gorm.DB
	cfg          *cfgpkg.Config
	githubClient *github.Client
	authClient   authpb.AuthServiceClient
	authConn     *grpc.ClientConn
}

// NewProjectServiceServer creates a new ProjectService server
func NewProjectServiceServer(db *gorm.DB, cfg *cfgpkg.Config, authClient authpb.AuthServiceClient, authConn *grpc.ClientConn) *ProjectServiceServer {
	return &ProjectServiceServer{
		db:           db,
		cfg:          cfg,
		githubClient: github.NewClient(),
		authClient:   authClient,
		authConn:     authConn,
	}
}

// ==================== Project CRUD ====================

// CreateProject creates a new project with webhook setup
func (s *ProjectServiceServer) CreateProject(ctx context.Context, req *pb.CreateProjectRequest) (*pb.CreateProjectResponse, error) {
	log.Info().
		Str("user_id", req.UserId).
		Str("name", req.Name).
		Str("repo_url", req.RepoUrl).
		Msg("CreateProject called")

	// Validate request
	if req.UserId == "" || req.Name == "" || req.RepoUrl == "" {
		return &pb.CreateProjectResponse{
			Error: "user_id, name, and repo_url are required",
		}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return &pb.CreateProjectResponse{Error: "invalid user_id format"}, nil
	}

	// Check permission: enforce max_projects limit (FR7.4)
	if s.authClient != nil {
		planResp, err := s.authClient.GetUserPlan(ctx, &authpb.GetUserPlanRequest{
			UserId: req.UserId,
		})
		if err != nil {
			log.Error().Err(err).Str("user_id", req.UserId).Msg("Failed to get user plan for permission check")
			return &pb.CreateProjectResponse{Error: "failed to check user plan"}, nil
		}
		if planResp.Error != "" {
			log.Warn().Str("error", planResp.Error).Str("user_id", req.UserId).Msg("GetUserPlan returned error")
			return &pb.CreateProjectResponse{Error: "failed to check user plan: " + planResp.Error}, nil
		}

		// Count existing projects for this user
		var projectCount int64
		if err := s.db.Model(&models.Project{}).Where("user_id = ?", userID).Count(&projectCount).Error; err != nil {
			log.Error().Err(err).Str("user_id", req.UserId).Msg("Failed to count existing projects")
			return &pb.CreateProjectResponse{Error: "failed to check project count"}, nil
		}

		// Check if user has reached the limit
		maxProjects := planResp.MaxProjects
		if maxProjects > 0 && projectCount >= int64(maxProjects) {
			log.Warn().
				Str("user_id", req.UserId).
				Int64("current_count", projectCount).
				Int32("max_projects", maxProjects).
				Str("plan", planResp.Plan).
				Msg("User reached max projects limit")
			return &pb.CreateProjectResponse{
				Error: fmt.Sprintf("You have reached the project limit for the %s plan (%d projects). Please upgrade your plan to create more projects.", planResp.Plan, maxProjects),
			}, nil
		}
	}

	// Set defaults
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	port := req.Port
	if port == 0 {
		// Random port from 12000-12999 range
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		port = int32(12000 + rng.Intn(1000))
	}

	// Create project in database
	project := &models.Project{
		UserID:       userID,
		Name:         req.Name,
		RepoURL:      req.RepoUrl,
		Branch:       branch,
		Preset:       req.Preset,
		BuildCommand: req.BuildCommand,
		StartCommand: req.StartCommand,
		Port:         int(port),
		GithubRepoID: req.GithubRepoId,
		IsPrivate:    req.IsPrivate,
	}

	if err := s.db.Create(project).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create project")
		return &pb.CreateProjectResponse{Error: "failed to create project"}, nil
	}

	// Setup webhook if access token provided
	if req.GithubAccessToken != "" {
		owner, repo, err := github.ParseRepoURL(req.RepoUrl)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to parse repo URL for webhook setup")
		} else {
			callbackURL := s.cfg.GitHubWebhookCallbackURL
			if callbackURL == "" {
				callbackURL = "http://localhost:8000/webhooks/github"
			}

			webhookResp, secret, err := s.githubClient.CreateWebhook(ctx, req.GithubAccessToken, owner, repo, callbackURL)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to create webhook")
			} else {
				// Save webhook info
				webhook := &models.Webhook{
					ProjectID:       project.ID,
					GithubWebhookID: webhookResp.ID,
					Secret:          secret,
				}
				if err := s.db.Create(webhook).Error; err != nil {
					log.Warn().Err(err).Msg("Failed to save webhook info")
				}
			}
		}
	}

	return &pb.CreateProjectResponse{
		Project: projectToProto(project),
	}, nil
}

// GetProject retrieves a project by ID
func (s *ProjectServiceServer) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.GetProjectResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Str("user_id", req.UserId).
		Msg("GetProject called")

	if req.ProjectId == "" {
		return &pb.GetProjectResponse{Error: "project_id is required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.GetProjectResponse{Error: "invalid project_id format"}, nil
	}

	var project models.Project
	if err := s.db.First(&project, "id = ?", projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &pb.GetProjectResponse{Error: "project not found"}, nil
		}
		log.Error().Err(err).Msg("Failed to get project")
		return &pb.GetProjectResponse{Error: "failed to get project"}, nil
	}

	// Check permission (user owns project)
	if req.UserId != "" {
		userID, _ := uuid.Parse(req.UserId)
		if project.UserID != userID {
			return &pb.GetProjectResponse{Error: "permission denied"}, nil
		}
	}

	return &pb.GetProjectResponse{
		Project: projectToProto(&project),
	}, nil
}

// GetProjectByRepo retrieves a project by repository URL or GitHub repo ID
func (s *ProjectServiceServer) GetProjectByRepo(ctx context.Context, req *pb.GetProjectByRepoRequest) (*pb.GetProjectResponse, error) {
	log.Info().
		Str("repo_url", req.RepoUrl).
		Int64("github_repo_id", req.GithubRepoId).
		Msg("GetProjectByRepo called")

	var project models.Project
	var err error

	// Prefer match by github_repo_id (exact match, faster)
	if req.GithubRepoId != 0 {
		if err = s.db.First(&project, "github_repo_id = ?", req.GithubRepoId).Error; err == nil {
			return &pb.GetProjectResponse{
				Project: projectToProto(&project),
			}, nil
		}
		if err != gorm.ErrRecordNotFound {
			log.Error().Err(err).Int64("github_repo_id", req.GithubRepoId).Msg("Failed to query project by github_repo_id")
			return &pb.GetProjectResponse{Error: "failed to query project"}, nil
		}
	}

	// Fallback to match by repo_url (normalize URLs for comparison)
	if req.RepoUrl != "" {
		// Normalize repo_url: remove .git suffix, convert to lowercase
		normalizedURL := strings.ToLower(strings.TrimSuffix(req.RepoUrl, ".git"))

		var projects []models.Project
		if err = s.db.Find(&projects).Error; err != nil {
			log.Error().Err(err).Msg("Failed to query projects")
			return &pb.GetProjectResponse{Error: "failed to query projects"}, nil
		}

		// Match by normalized repo URL
		for _, p := range projects {
			projectURL := strings.ToLower(strings.TrimSuffix(p.RepoURL, ".git"))
			if projectURL == normalizedURL {
				return &pb.GetProjectResponse{
					Project: projectToProto(&p),
				}, nil
			}
		}
	}

	log.Warn().
		Str("repo_url", req.RepoUrl).
		Int64("github_repo_id", req.GithubRepoId).
		Msg("Project not found by repo")
	return &pb.GetProjectResponse{Error: "project not found"}, nil
}

// ListProjects lists all projects for a user
func (s *ProjectServiceServer) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	log.Info().
		Str("user_id", req.UserId).
		Int32("page", req.Page).
		Int32("page_size", req.PageSize).
		Msg("ListProjects called")

	if req.UserId == "" {
		return &pb.ListProjectsResponse{Error: "user_id is required"}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return &pb.ListProjectsResponse{Error: "invalid user_id format"}, nil
	}

	// Set pagination defaults
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Query projects
	var projects []models.Project
	var total int64

	s.db.Model(&models.Project{}).Where("user_id = ?", userID).Count(&total)

	if err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(pageSize)).
		Find(&projects).Error; err != nil {
		log.Error().Err(err).Msg("Failed to list projects")
		return &pb.ListProjectsResponse{Error: "failed to list projects"}, nil
	}

	protoProjects := make([]*pb.Project, len(projects))
	for i, p := range projects {
		protoProjects[i] = projectToProto(&p)
	}

	return &pb.ListProjectsResponse{
		Projects: protoProjects,
		Total:    int32(total),
	}, nil
}

// UpdateProject updates a project's settings
func (s *ProjectServiceServer) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.UpdateProjectResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Str("user_id", req.UserId).
		Msg("UpdateProject called")

	if req.ProjectId == "" || req.UserId == "" {
		return &pb.UpdateProjectResponse{Error: "project_id and user_id are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.UpdateProjectResponse{Error: "invalid project_id format"}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return &pb.UpdateProjectResponse{Error: "invalid user_id format"}, nil
	}

	// Find project
	var project models.Project
	if err := s.db.First(&project, "id = ? AND user_id = ?", projectID, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &pb.UpdateProjectResponse{Error: "project not found or permission denied"}, nil
		}
		return &pb.UpdateProjectResponse{Error: "failed to get project"}, nil
	}

	// Update fields
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Branch != "" {
		updates["branch"] = req.Branch
	}
	if req.Preset != "" {
		updates["preset"] = req.Preset
	}
	if req.BuildCommand != "" {
		updates["build_command"] = req.BuildCommand
	}
	if req.StartCommand != "" {
		updates["start_command"] = req.StartCommand
	}
	if req.Port > 0 {
		updates["port"] = req.Port
	}

	if len(updates) > 0 {
		if err := s.db.Model(&project).Updates(updates).Error; err != nil {
			log.Error().Err(err).Msg("Failed to update project")
			return &pb.UpdateProjectResponse{Error: "failed to update project"}, nil
		}
	}

	// Reload project
	s.db.First(&project, "id = ?", projectID)

	return &pb.UpdateProjectResponse{
		Project: projectToProto(&project),
	}, nil
}

// DeleteProject deletes a project and its webhook
func (s *ProjectServiceServer) DeleteProject(ctx context.Context, req *pb.DeleteProjectRequest) (*pb.DeleteProjectResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Str("user_id", req.UserId).
		Msg("DeleteProject called")

	if req.ProjectId == "" || req.UserId == "" {
		return &pb.DeleteProjectResponse{Success: false, Error: "project_id and user_id are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.DeleteProjectResponse{Success: false, Error: "invalid project_id format"}, nil
	}

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return &pb.DeleteProjectResponse{Success: false, Error: "invalid user_id format"}, nil
	}

	// Find project with webhook
	var project models.Project
	if err := s.db.Preload("Webhooks").First(&project, "id = ? AND user_id = ?", projectID, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &pb.DeleteProjectResponse{Success: false, Error: "project not found or permission denied"}, nil
		}
		return &pb.DeleteProjectResponse{Success: false, Error: "failed to get project"}, nil
	}

	// Delete webhook from GitHub if access token provided
	if req.GithubAccessToken != "" && len(project.Webhooks) > 0 {
		owner, repo, err := github.ParseRepoURL(project.RepoURL)
		if err == nil {
			for _, wh := range project.Webhooks {
				if err := s.githubClient.DeleteWebhook(ctx, req.GithubAccessToken, owner, repo, wh.GithubWebhookID); err != nil {
					log.Warn().Err(err).Int64("webhook_id", wh.GithubWebhookID).Msg("Failed to delete webhook from GitHub")
				}
			}
		}
	}

	// Try to delete builds first (if they exist in the same database)
	// This handles cases where builds table exists but doesn't have CASCADE constraint
	sqlDB, err := s.db.DB()
	if err == nil {
		// Try to delete builds for this project (ignore errors if table doesn't exist)
		_, _ = sqlDB.Exec("DELETE FROM builds WHERE project_id = $1", projectID)
	}

	// Delete project (cascade deletes secrets and webhooks)
	if err := s.db.Delete(&project).Error; err != nil {
		log.Error().Err(err).Str("project_id", req.ProjectId).Msg("Failed to delete project")
		// Return more detailed error message
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "constraint") {
			return &pb.DeleteProjectResponse{Success: false, Error: "failed to delete project: foreign key constraint violation. Please delete associated builds and deployments first."}, nil
		}
		return &pb.DeleteProjectResponse{Success: false, Error: "failed to delete project: " + err.Error()}, nil
	}

	return &pb.DeleteProjectResponse{Success: true}, nil
}

// ==================== GitHub Integration ====================

// ListRepositories lists GitHub repositories for a user
func (s *ProjectServiceServer) ListRepositories(ctx context.Context, req *pb.ListRepositoriesRequest) (*pb.ListRepositoriesResponse, error) {
	log.Info().Str("user_id", req.UserId).Msg("ListRepositories called")

	if req.GithubAccessToken == "" {
		log.Warn().Str("user_id", req.UserId).Msg("ListRepositories called with empty GitHub access token")
		return &pb.ListRepositoriesResponse{Error: "github_access_token is required"}, nil
	}

	repos, err := s.githubClient.ListUserRepositories(ctx, req.GithubAccessToken)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", req.UserId).
			Msg("Failed to list GitHub repositories")

		// Extract HTTP status code if available from error message
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "GitHub API error:") {
			// Error already contains status code from GitHub client
			return &pb.ListRepositoriesResponse{
				Error: "failed to list repositories: " + errorMsg,
			}, nil
		}

		return &pb.ListRepositoriesResponse{
			Error: "failed to list repositories: " + errorMsg,
		}, nil
	}

	log.Info().
		Str("user_id", req.UserId).
		Int("repo_count", len(repos)).
		Msg("Successfully listed GitHub repositories")

	protoRepos := make([]*pb.Repository, len(repos))
	for i, r := range repos {
		protoRepos[i] = &pb.Repository{
			Id:            r.ID,
			Name:          r.Name,
			FullName:      r.FullName,
			CloneUrl:      r.CloneURL,
			HtmlUrl:       r.HTMLURL,
			IsPrivate:     r.Private,
			Description:   r.Description,
			Language:      r.Language,
			DefaultBranch: r.DefaultBranch,
		}
	}

	return &pb.ListRepositoriesResponse{Repositories: protoRepos}, nil
}

// SetupWebhook sets up a GitHub webhook for a project
func (s *ProjectServiceServer) SetupWebhook(ctx context.Context, req *pb.SetupWebhookRequest) (*pb.SetupWebhookResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Str("callback_url", req.CallbackUrl).
		Msg("SetupWebhook called")

	if req.ProjectId == "" || req.CallbackUrl == "" || req.GithubAccessToken == "" {
		return &pb.SetupWebhookResponse{Success: false, Error: "project_id, callback_url, and github_access_token are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.SetupWebhookResponse{Success: false, Error: "invalid project_id format"}, nil
	}

	// Get project
	var project models.Project
	if err := s.db.First(&project, "id = ?", projectID).Error; err != nil {
		return &pb.SetupWebhookResponse{Success: false, Error: "project not found"}, nil
	}

	// Parse repo URL
	owner, repo, err := github.ParseRepoURL(project.RepoURL)
	if err != nil {
		return &pb.SetupWebhookResponse{Success: false, Error: "invalid repo URL"}, nil
	}

	// Create webhook
	webhookResp, secret, err := s.githubClient.CreateWebhook(ctx, req.GithubAccessToken, owner, repo, req.CallbackUrl)
	if err != nil {
		return &pb.SetupWebhookResponse{Success: false, Error: "failed to create webhook: " + err.Error()}, nil
	}

	// Save webhook
	webhook := &models.Webhook{
		ProjectID:       projectID,
		GithubWebhookID: webhookResp.ID,
		Secret:          secret,
	}
	if err := s.db.Create(webhook).Error; err != nil {
		log.Error().Err(err).Msg("Failed to save webhook")
		return &pb.SetupWebhookResponse{Success: false, Error: "failed to save webhook"}, nil
	}

	return &pb.SetupWebhookResponse{
		Success:   true,
		WebhookId: webhookResp.ID,
	}, nil
}

// DeleteWebhook deletes a webhook from GitHub and database
func (s *ProjectServiceServer) DeleteWebhook(ctx context.Context, req *pb.DeleteWebhookRequest) (*pb.DeleteWebhookResponse, error) {
	log.Info().Str("project_id", req.ProjectId).Msg("DeleteWebhook called")

	if req.ProjectId == "" || req.GithubAccessToken == "" {
		return &pb.DeleteWebhookResponse{Success: false, Error: "project_id and github_access_token are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.DeleteWebhookResponse{Success: false, Error: "invalid project_id format"}, nil
	}

	// Get project with webhooks
	var project models.Project
	if err := s.db.Preload("Webhooks").First(&project, "id = ?", projectID).Error; err != nil {
		return &pb.DeleteWebhookResponse{Success: false, Error: "project not found"}, nil
	}

	owner, repo, err := github.ParseRepoURL(project.RepoURL)
	if err != nil {
		return &pb.DeleteWebhookResponse{Success: false, Error: "invalid repo URL"}, nil
	}

	// Delete webhooks
	for _, wh := range project.Webhooks {
		if err := s.githubClient.DeleteWebhook(ctx, req.GithubAccessToken, owner, repo, wh.GithubWebhookID); err != nil {
			log.Warn().Err(err).Msg("Failed to delete webhook from GitHub")
		}
		s.db.Delete(&wh)
	}

	return &pb.DeleteWebhookResponse{Success: true}, nil
}

// ==================== Secrets Management ====================

// AddSecret adds an encrypted secret to a project
func (s *ProjectServiceServer) AddSecret(ctx context.Context, req *pb.AddSecretRequest) (*pb.AddSecretResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Str("name", req.Name).
		Msg("AddSecret called")

	if req.ProjectId == "" || req.Name == "" || req.Value == "" {
		return &pb.AddSecretResponse{Error: "project_id, name, and value are required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.AddSecretResponse{Error: "invalid project_id format"}, nil
	}

	// Verify project exists and user has permission
	var project models.Project
	if req.UserId != "" {
		userID, _ := uuid.Parse(req.UserId)
		if err := s.db.First(&project, "id = ? AND user_id = ?", projectID, userID).Error; err != nil {
			return &pb.AddSecretResponse{Error: "project not found or permission denied"}, nil
		}
	} else {
		if err := s.db.First(&project, "id = ?", projectID).Error; err != nil {
			return &pb.AddSecretResponse{Error: "project not found"}, nil
		}
	}

	// Encrypt value
	encryptedValue, err := crypto.EncryptString(req.Value, s.cfg.EncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt secret")
		return &pb.AddSecretResponse{Error: "failed to encrypt secret"}, nil
	}

	// Create secret
	secret := &models.Secret{
		ProjectID:      projectID,
		Name:           req.Name,
		EncryptedValue: []byte(encryptedValue),
	}

	if err := s.db.Create(secret).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create secret")
		return &pb.AddSecretResponse{Error: "failed to create secret"}, nil
	}

	return &pb.AddSecretResponse{
		Secret: secretToProto(secret),
	}, nil
}

// UpdateSecret updates an existing secret
func (s *ProjectServiceServer) UpdateSecret(ctx context.Context, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error) {
	log.Info().
		Str("secret_id", req.SecretId).
		Str("project_id", req.ProjectId).
		Msg("UpdateSecret called")

	if req.SecretId == "" || req.Value == "" {
		return &pb.UpdateSecretResponse{Error: "secret_id and value are required"}, nil
	}

	secretID, err := uuid.Parse(req.SecretId)
	if err != nil {
		return &pb.UpdateSecretResponse{Error: "invalid secret_id format"}, nil
	}

	// Find secret
	var secret models.Secret
	if err := s.db.First(&secret, "id = ?", secretID).Error; err != nil {
		return &pb.UpdateSecretResponse{Error: "secret not found"}, nil
	}

	// Verify permission via project
	if req.UserId != "" {
		userID, _ := uuid.Parse(req.UserId)
		var project models.Project
		if err := s.db.First(&project, "id = ? AND user_id = ?", secret.ProjectID, userID).Error; err != nil {
			return &pb.UpdateSecretResponse{Error: "permission denied"}, nil
		}
	}

	// Encrypt new value
	encryptedValue, err := crypto.EncryptString(req.Value, s.cfg.EncryptionKey)
	if err != nil {
		return &pb.UpdateSecretResponse{Error: "failed to encrypt secret"}, nil
	}

	// Update
	secret.EncryptedValue = []byte(encryptedValue)
	if err := s.db.Save(&secret).Error; err != nil {
		return &pb.UpdateSecretResponse{Error: "failed to update secret"}, nil
	}

	return &pb.UpdateSecretResponse{
		Secret: secretToProto(&secret),
	}, nil
}

// DeleteSecret deletes a secret
func (s *ProjectServiceServer) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	log.Info().
		Str("secret_id", req.SecretId).
		Str("project_id", req.ProjectId).
		Msg("DeleteSecret called")

	if req.SecretId == "" {
		return &pb.DeleteSecretResponse{Success: false, Error: "secret_id is required"}, nil
	}

	secretID, err := uuid.Parse(req.SecretId)
	if err != nil {
		return &pb.DeleteSecretResponse{Success: false, Error: "invalid secret_id format"}, nil
	}

	// Find secret
	var secret models.Secret
	if err := s.db.First(&secret, "id = ?", secretID).Error; err != nil {
		return &pb.DeleteSecretResponse{Success: false, Error: "secret not found"}, nil
	}

	// Verify permission
	if req.UserId != "" {
		userID, _ := uuid.Parse(req.UserId)
		var project models.Project
		if err := s.db.First(&project, "id = ? AND user_id = ?", secret.ProjectID, userID).Error; err != nil {
			return &pb.DeleteSecretResponse{Success: false, Error: "permission denied"}, nil
		}
	}

	if err := s.db.Delete(&secret).Error; err != nil {
		return &pb.DeleteSecretResponse{Success: false, Error: "failed to delete secret"}, nil
	}

	return &pb.DeleteSecretResponse{Success: true}, nil
}

// ListSecrets lists secrets for a project (values masked)
func (s *ProjectServiceServer) ListSecrets(ctx context.Context, req *pb.ListSecretsRequest) (*pb.ListSecretsResponse, error) {
	log.Info().
		Str("project_id", req.ProjectId).
		Msg("ListSecrets called")

	if req.ProjectId == "" {
		return &pb.ListSecretsResponse{Error: "project_id is required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.ListSecretsResponse{Error: "invalid project_id format"}, nil
	}

	// Verify permission
	if req.UserId != "" {
		userID, _ := uuid.Parse(req.UserId)
		var project models.Project
		if err := s.db.First(&project, "id = ? AND user_id = ?", projectID, userID).Error; err != nil {
			return &pb.ListSecretsResponse{Error: "project not found or permission denied"}, nil
		}
	}

	var secrets []models.Secret
	if err := s.db.Where("project_id = ?", projectID).Find(&secrets).Error; err != nil {
		return &pb.ListSecretsResponse{Error: "failed to list secrets"}, nil
	}

	protoSecrets := make([]*pb.Secret, len(secrets))
	for i, s := range secrets {
		protoSecrets[i] = secretToProto(&s)
	}

	return &pb.ListSecretsResponse{Secrets: protoSecrets}, nil
}

// GetSecrets returns decrypted secrets for Runner Service (internal use)
func (s *ProjectServiceServer) GetSecrets(ctx context.Context, req *pb.GetSecretsRequest) (*pb.GetSecretsResponse, error) {
	log.Info().Str("project_id", req.ProjectId).Msg("GetSecrets called (internal)")

	if req.ProjectId == "" {
		return &pb.GetSecretsResponse{Error: "project_id is required"}, nil
	}

	projectID, err := uuid.Parse(req.ProjectId)
	if err != nil {
		return &pb.GetSecretsResponse{Error: "invalid project_id format"}, nil
	}

	var secrets []models.Secret
	if err := s.db.Where("project_id = ?", projectID).Find(&secrets).Error; err != nil {
		return &pb.GetSecretsResponse{Error: "failed to get secrets"}, nil
	}

	result := make(map[string]string)
	for _, secret := range secrets {
		decrypted, err := crypto.DecryptString(string(secret.EncryptedValue), s.cfg.EncryptionKey)
		if err != nil {
			log.Error().Err(err).Str("name", secret.Name).Msg("Failed to decrypt secret")
			continue
		}
		result[secret.Name] = decrypted
	}

	return &pb.GetSecretsResponse{Secrets: result}, nil
}

// ==================== Helper Functions ====================

func projectToProto(p *models.Project) *pb.Project {
	return &pb.Project{
		Id:           p.ID.String(),
		UserId:       p.UserID.String(),
		Name:         p.Name,
		RepoUrl:      p.RepoURL,
		Branch:       p.Branch,
		Preset:       p.Preset,
		BuildCommand: p.BuildCommand,
		StartCommand: p.StartCommand,
		Port:         int32(p.Port),
		GithubRepoId: p.GithubRepoID,
		IsPrivate:    p.IsPrivate,
		CreatedAt:    timestamppb.New(p.CreatedAt),
		UpdatedAt:    timestamppb.New(p.UpdatedAt),
	}
}

func secretToProto(s *models.Secret) *pb.Secret {
	return &pb.Secret{
		Id:        s.ID.String(),
		ProjectId: s.ProjectID.String(),
		Name:      s.Name,
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}
}
