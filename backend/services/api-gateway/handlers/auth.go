// Package handlers chứa các HTTP handlers cho API Gateway.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	pb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

// AuthHandler xử lý các request liên quan đến authentication.
type AuthHandler struct {
	client pb.AuthServiceClient
}

// NewAuthHandler tạo một AuthHandler mới.
func NewAuthHandler(client pb.AuthServiceClient) *AuthHandler {
	return &AuthHandler{client: client}
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

	resp, err := h.client.StartOAuthFlow(ctx, &pb.StartOAuthRequest{
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

	resp, err := h.client.HandleOAuthCallback(ctx, &pb.HandleOAuthRequest{
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
	redirectBase := r.URL.Query().Get("redirect_uri")
	if redirectBase == "" {
		redirectBase = os.Getenv("FRONTEND_URL")
	}
	if redirectBase == "" {
		redirectBase = "http://localhost:3001"
	}

	target, err := url.Parse(redirectBase)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("invalid redirect uri, fallback localhost:3002")
		target, _ = url.Parse("http://localhost:3001")
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

	resp, err := h.client.RefreshToken(ctx, &pb.RefreshTokenRequest{
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

	_, err = h.client.RevokeToken(ctx, &pb.RevokeTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("RevokeToken gRPC error")
		// Revoke is best-effort, still return success
	}

	w.WriteHeader(http.StatusNoContent)
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
