// Package handlers chứa các HTTP handlers cho API Gateway.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	apimw "github.com/nexusdeploy/backend/services/api-gateway/middleware"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

// AuthHandler xử lý các request liên quan đến authentication.
type AuthHandler struct {
	client        authpb.AuthServiceClient
	projectClient projectpb.ProjectServiceClient // For validation when changing plans
}

// NewAuthHandler tạo một AuthHandler mới.
func NewAuthHandler(client authpb.AuthServiceClient, projectClient projectpb.ProjectServiceClient) *AuthHandler {
	return &AuthHandler{
		client:        client,
		projectClient: projectClient,
	}
}

// LoginResponse là response trả về cho client sau khi login thành công.
type LoginResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	ExpiresAtUnix int64  `json:"expires_at_unix"`
	UserID        string `json:"user_id"`
	Plan          string `json:"plan"`
}

// ErrorResponse là response trả về khi có lỗi.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HandleGitHubLogin xử lý request bắt đầu OAuth flow.
// GET /auth/github/login -> redirect đến GitHub
func (h *AuthHandler) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)

	// Truyền correlation ID vào gRPC metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	resp, err := h.client.StartOAuthFlow(ctx, &authpb.StartOAuthRequest{
		RedirectUrl: r.URL.Query().Get("redirect_url"),
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("StartOAuthFlow gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "oauth_error", Message: "failed to initiate oauth"})
		return
	}

	if resp.Error != "" {
		log.Warn().Str("correlation_id", corrID).Str("error", resp.Error).Msg("StartOAuthFlow returned error")
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "oauth_error", Message: resp.Error})
		return
	}

	// Redirect user to GitHub authorization URL
	http.Redirect(w, r, resp.AuthUrl, http.StatusFound)
}

// HandleGitHubCallback xử lý callback từ GitHub sau khi user authorize.
// GET /auth/github/callback?code=...&state=... -> trả JSON tokens
func (h *AuthHandler) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "missing code or state"})
		return
	}

	// Truyền correlation ID vào gRPC metadata
	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	resp, err := h.client.HandleOAuthCallback(ctx, &authpb.HandleOAuthRequest{
		Code:  code,
		State: state,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("HandleOAuthCallback gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "oauth_error", Message: "failed to handle callback"})
		return
	}

	if resp.Error != "" {
		log.Warn().Str("correlation_id", corrID).Str("error", resp.Error).Msg("HandleOAuthCallback returned error")
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "oauth_error", Message: resp.Error})
		return
	}

	// Redirect về frontend kèm token ở query
	// Ưu tiên: redirect_url từ response > redirect_uri query param > FRONTEND_URL env > request origin
	redirectBase := resp.RedirectUrl
	if redirectBase == "" {
		redirectBase = r.URL.Query().Get("redirect_uri")
	}
	if redirectBase == "" {
		redirectBase = os.Getenv("FRONTEND_URL")
	}
	if redirectBase == "" {
		// Auto-detect from request origin (scheme + host)
		if r.Header.Get("X-Forwarded-Proto") != "" && r.Header.Get("X-Forwarded-Host") != "" {
			redirectBase = fmt.Sprintf("%s://%s", r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
		} else if r.Header.Get("Referer") != "" {
			// Extract origin from Referer header
			if refererURL, err := url.Parse(r.Header.Get("Referer")); err == nil {
				redirectBase = fmt.Sprintf("%s://%s", refererURL.Scheme, refererURL.Host)
			}
		}
	}
	if redirectBase == "" {
		redirectBase = "https://khqi.io.vn"
	}

	target, err := url.Parse(redirectBase)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("invalid redirect uri, fallback https://khqi.io.vn")
		target, _ = url.Parse("https://khqi.io.vn")
	}

	// Nếu path rỗng hoặc root, mặc định /auth/callback
	if target.Path == "" || target.Path == "/" {
		target.Path = "/auth/callback"
	}

	q := target.Query()
	q.Set("access_token", resp.AccessToken)
	q.Set("refresh_token", resp.RefreshToken)
	q.Set("expires_at", fmt.Sprintf("%d", resp.ExpiresAtUnix))
	q.Set("user_id", resp.UserId)
	q.Set("plan", resp.Plan)
	target.RawQuery = q.Encode()

	http.Redirect(w, r, target.String(), http.StatusFound)
}

// RefreshTokenRequest là request body cho refresh token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// HandleRefresh xử lý request làm mới access token.
// POST /auth/refresh
func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method_not_allowed"})
		return
	}

	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "invalid JSON body"})
		return
	}

	if req.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "refresh_token is required"})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	resp, err := h.client.RefreshToken(ctx, &authpb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("RefreshToken gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "refresh_error", Message: "failed to refresh token"})
		return
	}

	if resp.Error != "" {
		log.Warn().Str("correlation_id", corrID).Str("error", resp.Error).Msg("RefreshToken returned error")
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "refresh_error", Message: resp.Error})
		return
	}

	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken:   resp.AccessToken,
		RefreshToken:  resp.RefreshToken,
		ExpiresAtUnix: resp.ExpiresAtUnix,
	})
}

// HandleLogout xử lý request đăng xuất (revoke token).
// POST /auth/logout
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method_not_allowed"})
		return
	}

	// Lấy token từ Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "Authorization header required"})
		return
	}

	token, err := extractBearerToken(authHeader)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: err.Error()})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	_, err = h.client.RevokeToken(ctx, &authpb.RevokeTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("RevokeToken gRPC error")
		// Revoke is best-effort, still return success
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetUserInfo xử lý request lấy thông tin user hiện tại.
// GET /api/user/info
func (h *AuthHandler) HandleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)
	userID := apimw.GetUserID(ctx)

	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "user ID not found in context"})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	resp, err := h.client.GetUserInfo(ctx, &authpb.GetUserInfoRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", userID).Msg("GetUserInfo gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "internal_error", Message: "failed to get user info"})
		return
	}

	if resp.Error != "" {
		log.Warn().Str("correlation_id", corrID).Str("error", resp.Error).Msg("GetUserInfo returned error")
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "user_error", Message: resp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         resp.UserId,
		"username":   resp.Username,
		"email":      resp.Email,
		"avatar_url": resp.AvatarUrl,
		"plan":       resp.Plan,
		"github_id":  resp.GithubId,
	})
}

// HandleGetUserPlan xử lý request lấy thông tin plan của user hiện tại.
// GET /api/user/plan
func (h *AuthHandler) HandleGetUserPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)
	userID := apimw.GetUserID(ctx)

	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "user ID not found in context"})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	resp, err := h.client.GetUserPlan(ctx, &authpb.GetUserPlanRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", userID).Msg("GetUserPlan gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "internal_error", Message: "failed to get user plan"})
		return
	}

	if resp.Error != "" {
		log.Warn().Str("correlation_id", corrID).Str("error", resp.Error).Msg("GetUserPlan returned error")
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "plan_error", Message: resp.Error})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan":                  resp.Plan,
		"max_projects":          resp.MaxProjects,
		"max_builds_per_month":  resp.MaxBuildsPerMonth,
		"rate_limit_per_window": resp.RateLimitPerWindow,
	})
}

// HandleUpdatePlan xử lý request cập nhật plan của user.
// PUT /api/user/plan
func (h *AuthHandler) HandleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	corrID := commonmw.GetCorrelationID(ctx)
	userID := apimw.GetUserID(ctx)

	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: "user ID not found in context"})
		return
	}

	// Parse request body
	var reqBody struct {
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_request", Message: "invalid JSON body"})
		return
	}

	if reqBody.Plan != "standard" && reqBody.Plan != "premium" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid_plan", Message: "plan must be 'standard' or 'premium'"})
		return
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "correlation-id", corrID)

	// Step 1: Get current plan for validation
	currentPlanResp, err := h.client.GetUserPlan(ctx, &authpb.GetUserPlanRequest{
		UserId: userID,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", userID).Msg("GetUserPlan gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "internal_error", Message: "failed to get current plan"})
		return
	}
	if currentPlanResp.Error != "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "plan_error", Message: currentPlanResp.Error})
		return
	}

	currentPlan := currentPlanResp.Plan
	newPlan := reqBody.Plan

	// Step 2: Validate downgrade - check if user exceeds new plan limits (FR7.6.3)
	if currentPlan == "premium" && newPlan == "standard" {
		// Standard plan limits: 3 projects max
		maxProjects := int32(3)

		// Count user's current projects
		if h.projectClient != nil {
			projectsResp, err := h.projectClient.ListProjects(ctx, &projectpb.ListProjectsRequest{
				UserId: userID,
			})
			if err == nil && projectsResp.Error == "" {
				currentProjectCount := int32(len(projectsResp.Projects))
				if currentProjectCount > maxProjects {
					writeJSON(w, http.StatusBadRequest, ErrorResponse{
						Error:   "downgrade_not_allowed",
						Message: fmt.Sprintf("Cannot downgrade: you currently have %d projects, but the %s plan only allows a maximum of %d projects. Please delete some projects before downgrading.", currentProjectCount, newPlan, maxProjects),
					})
					return
				}
			} else if err != nil {
				log.Warn().Err(err).Str("correlation_id", corrID).Msg("Failed to validate project count for downgrade, proceeding anyway")
			}
		}
	}

	// Step 3: Update plan
	updateResp, err := h.client.UpdatePlan(ctx, &authpb.UpdatePlanRequest{
		UserId: userID,
		Plan:   newPlan,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", userID).Msg("UpdatePlan gRPC error")
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "internal_error", Message: "failed to update plan"})
		return
	}

	if !updateResp.Success {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "update_failed", Message: updateResp.Error})
		return
	}

	log.Info().
		Str("correlation_id", corrID).
		Str("user_id", userID).
		Str("old_plan", currentPlan).
		Str("new_plan", newPlan).
		Msg("Plan updated successfully")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Plan đã được cập nhật từ %s sang %s", currentPlan, newPlan),
		"plan":    newPlan,
	})
}

// writeJSON helper để write JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("failed to write JSON response")
	}
}

// extractBearerToken lấy token từ Authorization header.
func extractBearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return "", &invalidAuthHeaderError{}
	}
	token := header[len(prefix):]
	if token == "" {
		return "", &invalidAuthHeaderError{}
	}
	return token, nil
}

type invalidAuthHeaderError struct{}

func (e *invalidAuthHeaderError) Error() string {
	return "invalid Authorization header format"
}
