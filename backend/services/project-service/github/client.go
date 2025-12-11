package github

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	githubAPIURL = "https://api.github.com"
)

// Repository represents a GitHub repository
type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
	Language      string `json:"language"`
	DefaultBranch string `json:"default_branch"`
}

// WebhookConfig represents GitHub webhook configuration
type WebhookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret"`
	InsecureSSL string `json:"insecure_ssl"`
}

// WebhookRequest represents a webhook creation request
type WebhookRequest struct {
	Name   string        `json:"name"`
	Active bool          `json:"active"`
	Events []string      `json:"events"`
	Config WebhookConfig `json:"config"`
}

// WebhookResponse represents a webhook response from GitHub
type WebhookResponse struct {
	ID     int64  `json:"id"`
	Active bool   `json:"active"`
	Error  string `json:"message,omitempty"`
}

// Client is a GitHub API client
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new GitHub client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListUserRepositories lists all repositories for the authenticated user
func (c *Client) ListUserRepositories(ctx context.Context, accessToken string) ([]Repository, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL+"/user/repos?per_page=100&sort=updated", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var repos []Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return repos, nil
}

// CreateWebhook creates a webhook for a repository
func (c *Client) CreateWebhook(ctx context.Context, accessToken, owner, repo, callbackURL string) (*WebhookResponse, string, error) {
	// Generate random webhook secret
	secret, err := generateSecret(32)
	if err != nil {
		return nil, "", fmt.Errorf("generate secret: %w", err)
	}

	webhookReq := WebhookRequest{
		Name:   "web",
		Active: true,
		Events: []string{"push", "pull_request"},
		Config: WebhookConfig{
			URL:         callbackURL,
			ContentType: "json",
			Secret:      secret,
			InsecureSSL: "0",
		},
	}

	body, err := json.Marshal(webhookReq)
	if err != nil {
		return nil, "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/hooks", githubAPIURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(respBody))
	}

	var webhookResp WebhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&webhookResp); err != nil {
		return nil, "", fmt.Errorf("decode response: %w", err)
	}

	return &webhookResp, secret, nil
}

// DeleteWebhook deletes a webhook from a repository
func (c *Client) DeleteWebhook(ctx context.Context, accessToken, owner, repo string, webhookID int64) error {
	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", githubAPIURL, owner, repo, webhookID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}

// ParseRepoFullName parses "owner/repo" format
func ParseRepoFullName(fullName string) (owner, repo string, err error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo full name: %s", fullName)
	}
	return parts[0], parts[1], nil
}

// ParseRepoURL parses a GitHub URL and extracts owner/repo
func ParseRepoURL(repoURL string) (owner, repo string, err error) {
	// Handle both HTTPS and SSH URLs
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	repoURL = strings.TrimSuffix(repoURL, ".git")
	
	if strings.HasPrefix(repoURL, "https://github.com/") {
		path := strings.TrimPrefix(repoURL, "https://github.com/")
		return ParseRepoFullName(path)
	}
	
	if strings.HasPrefix(repoURL, "git@github.com:") {
		path := strings.TrimPrefix(repoURL, "git@github.com:")
		return ParseRepoFullName(path)
	}
	
	return "", "", fmt.Errorf("unsupported repo URL format: %s", repoURL)
}

// generateSecret generates a random hex string
func generateSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

