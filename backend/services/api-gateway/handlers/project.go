package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProjectServiceClient defines the methods of Project Service
type ProjectServiceClient interface {
	CreateProject(ctx context.Context, in *projectpb.CreateProjectRequest, opts ...grpc.CallOption) (*projectpb.CreateProjectResponse, error)
	GetProject(ctx context.Context, in *projectpb.GetProjectRequest, opts ...grpc.CallOption) (*projectpb.GetProjectResponse, error)
	ListProjects(ctx context.Context, in *projectpb.ListProjectsRequest, opts ...grpc.CallOption) (*projectpb.ListProjectsResponse, error)
	UpdateProject(ctx context.Context, in *projectpb.UpdateProjectRequest, opts ...grpc.CallOption) (*projectpb.UpdateProjectResponse, error)
	DeleteProject(ctx context.Context, in *projectpb.DeleteProjectRequest, opts ...grpc.CallOption) (*projectpb.DeleteProjectResponse, error)
	ListRepositories(ctx context.Context, in *projectpb.ListRepositoriesRequest, opts ...grpc.CallOption) (*projectpb.ListRepositoriesResponse, error)
	AddSecret(ctx context.Context, in *projectpb.AddSecretRequest, opts ...grpc.CallOption) (*projectpb.AddSecretResponse, error)
	UpdateSecret(ctx context.Context, in *projectpb.UpdateSecretRequest, opts ...grpc.CallOption) (*projectpb.UpdateSecretResponse, error)
	DeleteSecret(ctx context.Context, in *projectpb.DeleteSecretRequest, opts ...grpc.CallOption) (*projectpb.DeleteSecretResponse, error)
	ListSecrets(ctx context.Context, in *projectpb.ListSecretsRequest, opts ...grpc.CallOption) (*projectpb.ListSecretsResponse, error)
}

// ProjectHandler handles project-related requests
type ProjectHandler struct {
	Client           ProjectServiceClient
	GetGitHubToken   func(ctx context.Context, userID string) (string, error) // Callback to get user's GitHub token
}

// NewProjectHandler creates a new ProjectHandler
func NewProjectHandler(client ProjectServiceClient) *ProjectHandler {
	return &ProjectHandler{Client: client}
}

// ==================== REST Response Types ====================

type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RepoURL      string    `json:"repo_url"`
	Branch       string    `json:"branch"`
	Preset       string    `json:"preset"`
	BuildCommand string    `json:"build_command,omitempty"`
	StartCommand string    `json:"start_command,omitempty"`
	Port         int32     `json:"port"`
	GitHubRepoID int64     `json:"github_repo_id"`
	IsPrivate    bool      `json:"is_private"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
	IsPrivate     bool   `json:"is_private"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	DefaultBranch string `json:"default_branch"`
}

type Secret struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ==================== Project Endpoints ====================

// CreateProject handles POST /api/projects
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Name         string `json:"name"`
		RepoURL      string `json:"repo_url"`
		Branch       string `json:"branch"`
		Preset       string `json:"preset"`
		BuildCommand string `json:"build_command"`
		StartCommand string `json:"start_command"`
		Port         int32  `json:"port"`
		GitHubRepoID int64  `json:"github_repo_id"`
		IsPrivate    bool   `json:"is_private"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	// Get GitHub token for webhook setup
	var githubToken string
	if h.GetGitHubToken != nil {
		githubToken, _ = h.GetGitHubToken(r.Context(), userID)
	}

	resp, err := h.Client.CreateProject(r.Context(), &projectpb.CreateProjectRequest{
		UserId:            userID,
		Name:              req.Name,
		RepoUrl:           req.RepoURL,
		Branch:            req.Branch,
		Preset:            req.Preset,
		BuildCommand:      req.BuildCommand,
		StartCommand:      req.StartCommand,
		Port:              req.Port,
		GithubRepoId:      req.GitHubRepoID,
		IsPrivate:         req.IsPrivate,
		GithubAccessToken: githubToken,
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

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"project": protoToProject(resp.Project),
	})
}

// GetProject handles GET /api/projects/{id}
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.Client.GetProject(r.Context(), &projectpb.GetProjectRequest{
		ProjectId: projectID,
		UserId:    userID,
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": protoToProject(resp.Project),
	})
}

// ListProjects handles GET /api/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	page := parseQueryInt(r, "page", 1)
	pageSize := parseQueryInt(r, "page_size", 20)

	resp, err := h.Client.ListProjects(r.Context(), &projectpb.ListProjectsRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
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

	projects := make([]Project, 0, len(resp.Projects))
	for _, p := range resp.Projects {
		projects = append(projects, protoToProject(p))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    resp.Total,
	})
}

// UpdateProject handles PUT /api/projects/{id}
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
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

	var req struct {
		Name         string `json:"name"`
		Branch       string `json:"branch"`
		Preset       string `json:"preset"`
		BuildCommand string `json:"build_command"`
		StartCommand string `json:"start_command"`
		Port         int32  `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	resp, err := h.Client.UpdateProject(r.Context(), &projectpb.UpdateProjectRequest{
		ProjectId:    projectID,
		UserId:       userID,
		Name:         req.Name,
		Branch:       req.Branch,
		Preset:       req.Preset,
		BuildCommand: req.BuildCommand,
		StartCommand: req.StartCommand,
		Port:         req.Port,
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project": protoToProject(resp.Project),
	})
}

// DeleteProject handles DELETE /api/projects/{id}
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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

	// Get GitHub token for webhook deletion
	var githubToken string
	if h.GetGitHubToken != nil {
		githubToken, _ = h.GetGitHubToken(r.Context(), userID)
	}

	resp, err := h.Client.DeleteProject(r.Context(), &projectpb.DeleteProjectRequest{
		ProjectId:         projectID,
		UserId:            userID,
		GithubAccessToken: githubToken,
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "project deleted",
	})
}

// ==================== Repository Endpoints ====================

// ListRepositories handles GET /api/repos
func (h *ProjectHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get GitHub token
	var githubToken string
	if h.GetGitHubToken != nil {
		var err error
		githubToken, err = h.GetGitHubToken(r.Context(), userID)
		if err != nil || githubToken == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "github_token_required"})
			return
		}
	}

	resp, err := h.Client.ListRepositories(r.Context(), &projectpb.ListRepositoriesRequest{
		UserId:            userID,
		GithubAccessToken: githubToken,
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

	repos := make([]Repository, 0, len(resp.Repositories))
	for _, r := range resp.Repositories {
		repos = append(repos, Repository{
			ID:            r.Id,
			Name:          r.Name,
			FullName:      r.FullName,
			CloneURL:      r.CloneUrl,
			HTMLURL:       r.HtmlUrl,
			IsPrivate:     r.IsPrivate,
			Description:   r.Description,
			Language:      r.Language,
			DefaultBranch: r.DefaultBranch,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"repositories": repos,
	})
}

// ==================== Secret Endpoints ====================

// AddSecret handles POST /api/projects/{id}/secrets
func (h *ProjectHandler) AddSecret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Extract project_id from path: /api/projects/{id}/secrets
	projectID := extractProjectIDFromSecretsPath(r.URL.Path)
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	resp, err := h.Client.AddSecret(r.Context(), &projectpb.AddSecretRequest{
		ProjectId: projectID,
		UserId:    userID,
		Name:      req.Name,
		Value:     req.Value,
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

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"secret": protoToSecret(resp.Secret),
	})
}

// ListSecrets handles GET /api/projects/{id}/secrets
func (h *ProjectHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projectID := extractProjectIDFromSecretsPath(r.URL.Path)
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id required"})
		return
	}

	resp, err := h.Client.ListSecrets(r.Context(), &projectpb.ListSecretsRequest{
		ProjectId: projectID,
		UserId:    userID,
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

	secrets := make([]Secret, 0, len(resp.Secrets))
	for _, s := range resp.Secrets {
		secrets = append(secrets, protoToSecret(s))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"secrets": secrets,
	})
}

// DeleteSecret handles DELETE /api/projects/{project_id}/secrets/{secret_id}
func (h *ProjectHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}

	userID := apimw.GetUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Parse path: /api/projects/{project_id}/secrets/{secret_id}
	projectID, secretID := extractProjectAndSecretID(r.URL.Path)
	if projectID == "" || secretID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project_id and secret_id required"})
		return
	}

	resp, err := h.Client.DeleteSecret(r.Context(), &projectpb.DeleteSecretRequest{
		SecretId:  secretID,
		ProjectId: projectID,
		UserId:    userID,
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "secret deleted",
	})
}

// ==================== Helper Functions ====================

func parseQueryInt(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
		return parsed
	}
	return defaultValue
}

func extractPathParam(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	// Handle paths like /api/projects/123/secrets
	if idx := strings.Index(rest, "/"); idx != -1 {
		return rest[:idx]
	}
	return rest
}

func extractProjectIDFromSecretsPath(path string) string {
	// /api/projects/{project_id}/secrets
	const prefix = "/api/projects/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	idx := strings.Index(rest, "/secrets")
	if idx == -1 {
		return ""
	}
	return rest[:idx]
}

func extractProjectAndSecretID(path string) (projectID, secretID string) {
	// /api/projects/{project_id}/secrets/{secret_id}
	const prefix = "/api/projects/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) >= 3 && parts[1] == "secrets" {
		return parts[0], parts[2]
	}
	return "", ""
}

func toTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

func protoToProject(p *projectpb.Project) Project {
	if p == nil {
		return Project{}
	}
	return Project{
		ID:           p.Id,
		Name:         p.Name,
		RepoURL:      p.RepoUrl,
		Branch:       p.Branch,
		Preset:       p.Preset,
		BuildCommand: p.BuildCommand,
		StartCommand: p.StartCommand,
		Port:         p.Port,
		GitHubRepoID: p.GithubRepoId,
		IsPrivate:    p.IsPrivate,
		CreatedAt:    toTime(p.CreatedAt),
		UpdatedAt:    toTime(p.UpdatedAt),
	}
}

func protoToSecret(s *projectpb.Secret) Secret {
	if s == nil {
		return Secret{}
	}
	return Secret{
		ID:        s.Id,
		ProjectID: s.ProjectId,
		Name:      s.Name,
		CreatedAt: toTime(s.CreatedAt),
		UpdatedAt: toTime(s.UpdatedAt),
	}
}
