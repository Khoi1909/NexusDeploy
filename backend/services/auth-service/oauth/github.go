// Package oauth cung cấp logic tương tác với GitHub OAuth API.
package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	githubAuthURL    = "https://github.com/login/oauth/authorize"
	githubTokenURL   = "https://github.com/login/oauth/access_token"
	githubUserAPI    = "https://api.github.com/user"
	githubEmailsAPI  = "https://api.github.com/user/emails"
	oauthStatePrefix = "auth:oauth:state:"
	oauthStateTTL    = 10 * time.Minute
	httpTimeout      = 10 * time.Second
)

var (
	ErrStateNotFound    = errors.New("oauth state not found or expired")
	ErrStateMismatch    = errors.New("oauth state mismatch")
	ErrEmptyAccessToken = errors.New("empty access token from GitHub")
	ErrNoEmailReturned  = errors.New("no email returned from GitHub")
)

// GitHubClient xử lý các tương tác với GitHub OAuth.
type GitHubClient struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Redis        *redis.Client
	HTTPClient   *http.Client
}

// NewGitHubClient tạo một GitHubClient mới.
func NewGitHubClient(clientID, clientSecret, redirectURL string, redisClient *redis.Client) *GitHubClient {
	return &GitHubClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Redis:        redisClient,
		HTTPClient:   &http.Client{Timeout: httpTimeout},
	}
}

// GitHubUser chứa thông tin user từ GitHub API.
type GitHubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// GenerateAuthURL tạo URL xác thực GitHub và lưu state vào Redis.
func (c *GitHubClient) GenerateAuthURL(ctx context.Context, customRedirectURL string) (authURL string, state string, err error) {
	state = uuid.NewString()

	// Lưu state vào Redis với TTL
	stateKey := oauthStatePrefix + state
	if err := c.Redis.Set(ctx, stateKey, "1", oauthStateTTL).Err(); err != nil {
		return "", "", fmt.Errorf("store oauth state: %w", err)
	}

	redirectURL := c.RedirectURL
	if customRedirectURL != "" {
		redirectURL = customRedirectURL
	}

	query := url.Values{}
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURL)
	query.Set("state", state)
	query.Set("scope", "repo,user:email")

	authURL = fmt.Sprintf("%s?%s", githubAuthURL, query.Encode())
	return authURL, state, nil
}

// ValidateState kiểm tra và xoá state từ Redis.
func (c *GitHubClient) ValidateState(ctx context.Context, state string) error {
	if state == "" {
		return ErrStateMismatch
	}

	stateKey := oauthStatePrefix + state
	deleted, err := c.Redis.Del(ctx, stateKey).Result()
	if err != nil {
		return fmt.Errorf("delete oauth state: %w", err)
	}
	if deleted == 0 {
		return ErrStateNotFound
	}
	return nil
}

// ExchangeCode đổi authorization code lấy access token.
func (c *GitHubClient) ExchangeCode(ctx context.Context, code string) (string, error) {
	payload := map[string]string{
		"client_id":     c.ClientID,
		"client_secret": c.ClientSecret,
		"code":          code,
		"redirect_uri":  c.RedirectURL,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token exchange failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("github oauth error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", ErrEmptyAccessToken
	}

	return tokenResp.AccessToken, nil
}

// FetchUser lấy thông tin user từ GitHub API.
func (c *GitHubClient) FetchUser(ctx context.Context, accessToken string) (*GitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user API failed with status %d", resp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}

	return &user, nil
}

// FetchPrimaryEmail lấy email chính của user từ GitHub API.
func (c *GitHubClient) FetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEmailsAPI, nil)
	if err != nil {
		return "", fmt.Errorf("create emails request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute emails request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails API failed with status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode emails response: %w", err)
	}

	// Ưu tiên email primary và verified
	for _, e := range emails {
		if e.Primary && e.Verified && e.Email != "" {
			return e.Email, nil
		}
	}

	// Fallback: lấy email đầu tiên
	if len(emails) > 0 && emails[0].Email != "" {
		return emails[0].Email, nil
	}

	return "", ErrNoEmailReturned
}

// FetchUserWithEmail lấy user và tự động fetch email nếu cần.
func (c *GitHubClient) FetchUserWithEmail(ctx context.Context, accessToken string) (*GitHubUser, error) {
	user, err := c.FetchUser(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	if user.Email == "" {
		email, err := c.FetchPrimaryEmail(ctx, accessToken)
		if err != nil {
			return nil, err
		}
		user.Email = email
	}

	return user, nil
}

