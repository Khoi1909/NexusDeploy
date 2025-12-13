package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	cryptopkg "github.com/nexusdeploy/backend/pkg/crypto"
	"github.com/nexusdeploy/backend/services/auth-service/models"
	"github.com/nexusdeploy/backend/services/auth-service/oauth"
	pb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

const (
	blacklistKeyPrefix   = "auth:jwt:blacklist:"
	defaultPlan          = "standard"
	customDomainResource = "custom_domain"
	refreshTokenTTL      = 7 * 24 * time.Hour
)

// Claims định nghĩa JWT custom claims.
type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username"`
	Plan     string `json:"plan"`
	Avatar   string `json:"avatar_url"`
	jwt.RegisteredClaims
}

// planLimits contains limits by plan.
type planLimits struct {
	MaxProjects        int32
	MaxBuildsPerMonth  int32
	RateLimitPerWindow int32 // Rate limit requests per window
}

var planMatrix = map[string]planLimits{
	"standard": {
		MaxProjects:        3,
		MaxBuildsPerMonth:  1,
		RateLimitPerWindow: 0, // 0 = no limit
	},
	"premium": {
		MaxProjects:        20,
		MaxBuildsPerMonth:  5,
		RateLimitPerWindow: 0, // 0 = no limit
	},
}

// AuthServiceServer implements gRPC Auth Service.
type AuthServiceServer struct {
	pb.UnimplementedAuthServiceServer

	cfg          *cfgpkg.Config
	db           *gorm.DB
	redis        *redis.Client
	jwtSecret    []byte
	githubClient *oauth.GitHubClient
}

// NewAuthServiceServer tạo server với dependencies cần thiết.
func NewAuthServiceServer(cfg *cfgpkg.Config, db *gorm.DB, redisClient *redis.Client) *AuthServiceServer {
	secret := []byte(cfg.JWTSecret)
	githubClient := oauth.NewGitHubClient(
		cfg.GitHubClientID,
		cfg.GitHubClientSecret,
		cfg.GitHubRedirectURL,
		redisClient,
	)
	return &AuthServiceServer{
		cfg:          cfg,
		db:           db,
		redis:        redisClient,
		jwtSecret:    secret,
		githubClient: githubClient,
	}
}

// getCorrelationID extracts correlation_id from gRPC metadata for logging.
func getCorrelationID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("correlation-id"); len(vals) > 0 {
			return vals[0]
		}
	}
	return "unknown"
}

// StartOAuthFlow tạo URL xác thực GitHub và state.
func (s *AuthServiceServer) StartOAuthFlow(ctx context.Context, req *pb.StartOAuthRequest) (*pb.StartOAuthResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().Str("correlation_id", corrID).Msg("StartOAuthFlow called")

	if s.cfg.GitHubClientID == "" || s.cfg.GitHubClientSecret == "" {
		return &pb.StartOAuthResponse{
			Error: "GitHub OAuth chưa được cấu hình",
		}, nil
	}

	authURL, state, err := s.githubClient.GenerateAuthURL(ctx, req.RedirectUrl)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("generate auth URL failed")
		return &pb.StartOAuthResponse{
			Error: "cannot initiate oauth flow",
		}, nil
	}

	return &pb.StartOAuthResponse{
		AuthUrl: authURL,
		State:   state,
	}, nil
}

// HandleOAuthCallback xử lý callback từ GitHub, đổi code lấy token, upsert user.
func (s *AuthServiceServer) HandleOAuthCallback(ctx context.Context, req *pb.HandleOAuthRequest) (*pb.HandleOAuthResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().Str("correlation_id", corrID).Msg("HandleOAuthCallback called")

	if req.Code == "" || req.State == "" {
		return &pb.HandleOAuthResponse{Error: "code và state là bắt buộc"}, nil
	}

	// Validate state
	if err := s.githubClient.ValidateState(ctx, req.State); err != nil {
		log.Warn().Err(err).Str("correlation_id", corrID).Msg("oauth state validation failed")
		return &pb.HandleOAuthResponse{Error: "invalid or expired state"}, nil
	}

	// Exchange code for GitHub access token
	ghToken, err := s.githubClient.ExchangeCode(ctx, req.Code)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("exchange code failed")
		return &pb.HandleOAuthResponse{Error: "oauth exchange failed"}, nil
	}

	// Fetch user info from GitHub
	ghUser, err := s.githubClient.FetchUserWithEmail(ctx, ghToken)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("fetch github user failed")
		return &pb.HandleOAuthResponse{Error: "github user fetch failed"}, nil
	}

	// Encrypt GitHub token before storing
	encryptedToken, err := cryptopkg.EncryptString(s.cfg.MasterEncryptionKey, ghToken)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("encrypt github token failed")
		return &pb.HandleOAuthResponse{Error: "internal encryption error"}, nil
	}

	// Upsert user in database
	user, err := s.upsertUser(ctx, ghUser, encryptedToken)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("upsert user failed")
		return &pb.HandleOAuthResponse{Error: "persist user failed"}, nil
	}

	// Generate access + refresh tokens
	accessToken, refreshToken, expiresAt, err := s.GenerateTokensForUser(ctx, user)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("generate tokens failed")
		return &pb.HandleOAuthResponse{Error: "generate token failed"}, nil
	}

	return &pb.HandleOAuthResponse{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		UserId:        user.ID,
		Plan:          user.Plan,
		ExpiresAtUnix: expiresAt.Unix(),
	}, nil
}

// ValidateToken parses JWT, checks blacklist and returns user information.
func (s *AuthServiceServer) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	corrID := getCorrelationID(ctx)

	if req.Token == "" {
		return &pb.ValidateTokenResponse{
			Valid: false,
			Error: "token is required",
		}, nil
	}

	claims, err := s.parseToken(req.Token)
	if err != nil {
		log.Warn().Err(err).Str("correlation_id", corrID).Msg("jwt validation failed")
		return &pb.ValidateTokenResponse{
			Valid: false,
			Error: "invalid token",
		}, nil
	}

	// Check blacklist
	if claims.ID != "" {
		key := blacklistKeyPrefix + claims.ID
		exists, err := s.redis.Exists(ctx, key).Result()
		if err != nil {
			return nil, fmt.Errorf("redis check blacklist: %w", err)
		}
		if exists > 0 {
			return &pb.ValidateTokenResponse{
				Valid: false,
				Error: "token revoked",
			}, nil
		}
	}

	return &pb.ValidateTokenResponse{
		Valid:     true,
		UserId:    claims.UserID,
		Username:  claims.Username,
		Plan:      claims.Plan,
		AvatarUrl: claims.Avatar,
	}, nil
}

// GetUserPlan returns the current plan information for the user.
func (s *AuthServiceServer) GetUserPlan(ctx context.Context, req *pb.GetUserPlanRequest) (*pb.GetUserPlanResponse, error) {
	corrID := getCorrelationID(ctx)

	if req.UserId == "" {
		return &pb.GetUserPlanResponse{Error: "user_id is required"}, nil
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.GetUserPlanResponse{Error: "user not found"}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Msg("query user failed")
		return nil, fmt.Errorf("query user: %w", err)
	}

	limits := planMatrix[user.Plan]
	if limits == (planLimits{}) {
		limits = planMatrix[defaultPlan]
	}

	return &pb.GetUserPlanResponse{
		Plan:               user.Plan,
		MaxProjects:        limits.MaxProjects,
		MaxBuildsPerMonth:  limits.MaxBuildsPerMonth,
		RateLimitPerWindow: limits.RateLimitPerWindow,
	}, nil
}

// CheckPermission xác thực quyền dựa trên plan + bảng permissions.
func (s *AuthServiceServer) CheckPermission(ctx context.Context, req *pb.CheckPermissionRequest) (*pb.CheckPermissionResponse, error) {
	corrID := getCorrelationID(ctx)

	if req.UserId == "" || req.ResourceType == "" || req.Action == "" {
		return &pb.CheckPermissionResponse{
			Allowed: false,
			Reason:  "user_id, resource_type và action là bắt buộc",
		}, nil
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.CheckPermissionResponse{
				Allowed: false,
				Reason:  "user not found",
			}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Msg("query user for permission failed")
		return nil, fmt.Errorf("query user for permission: %w", err)
	}

	// Check explicit permission.
	var perm models.Permission
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND resource = ? AND action = ?", user.ID, req.ResourceType, req.Action).
		First(&perm).Error
	if err == nil {
		return &pb.CheckPermissionResponse{Allowed: true, Reason: "explicit permission granted"}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("query permission: %w", err)
	}

	// Plan based rules.
	switch req.ResourceType {
	case customDomainResource:
		if user.Plan != "premium" {
			return &pb.CheckPermissionResponse{
				Allowed: false,
				Reason:  "Custom domain requires Premium plan",
			}, nil
		}
	}

	return &pb.CheckPermissionResponse{
		Allowed: true,
		Reason:  "allowed by plan",
	}, nil
}

// UpdatePlan cập nhật plan của user (admin/internal use)
func (s *AuthServiceServer) UpdatePlan(ctx context.Context, req *pb.UpdatePlanRequest) (*pb.UpdatePlanResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().
		Str("correlation_id", corrID).
		Str("user_id", req.UserId).
		Str("new_plan", req.Plan).
		Msg("UpdatePlan called")

	if req.UserId == "" {
		return &pb.UpdatePlanResponse{
			Success: false,
			Error:   "user_id is required",
		}, nil
	}

	if req.Plan != "standard" && req.Plan != "premium" {
		return &pb.UpdatePlanResponse{
			Success: false,
			Error:   "invalid plan. Must be 'standard' or 'premium'",
		}, nil
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.UpdatePlanResponse{
				Success: false,
				Error:   "user not found",
			}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Msg("query user failed")
		return nil, fmt.Errorf("query user: %w", err)
	}

	oldPlan := user.Plan
	newPlan := req.Plan

	// Validation: If downgrading, check if user exceeds new plan limits (FR7.6.3)
	if oldPlan == "premium" && newPlan == "standard" {
		// Get current plan limits
		newLimits := planMatrix[newPlan]
		if newLimits == (planLimits{}) {
			newLimits = planMatrix[defaultPlan]
		}

		// Check projects count
		// Note: We can't check this directly here as we don't have access to project_db
		// This validation should be done by the caller (API Gateway) that has access to both services
		// or we could add a helper method that calls Project Service
		log.Info().
			Str("correlation_id", corrID).
			Str("user_id", req.UserId).
			Str("old_plan", oldPlan).
			Str("new_plan", newPlan).
			Int32("max_projects", newLimits.MaxProjects).
			Msg("Downgrading plan - validation should be done by caller")
	}

	// Update plan
	if err := s.db.WithContext(ctx).Model(&user).Update("plan", newPlan).Error; err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("update plan failed")
		return nil, fmt.Errorf("update plan: %w", err)
	}

	log.Info().
		Str("correlation_id", corrID).
		Str("user_id", req.UserId).
		Str("old_plan", oldPlan).
		Str("new_plan", newPlan).
		Msg("Plan updated successfully")

	return &pb.UpdatePlanResponse{
		Success: true,
	}, nil
}

// GetUserInfo trả thông tin cơ bản của user.
func (s *AuthServiceServer) GetUserInfo(ctx context.Context, req *pb.GetUserInfoRequest) (*pb.GetUserInfoResponse, error) {
	corrID := getCorrelationID(ctx)

	if req.UserId == "" {
		return &pb.GetUserInfoResponse{Error: "user_id is required"}, nil
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.GetUserInfoResponse{Error: "user not found"}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Msg("query user info failed")
		return nil, fmt.Errorf("query user info: %w", err)
	}

	return &pb.GetUserInfoResponse{
		UserId:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		AvatarUrl: user.AvatarURL,
		Plan:      user.Plan,
		GithubId:  user.GithubID,
	}, nil
}

// RefreshToken thực hiện refresh token rotation theo SRS 6.4.
func (s *AuthServiceServer) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().Str("correlation_id", corrID).Msg("RefreshToken called")

	if req.RefreshToken == "" {
		return &pb.RefreshTokenResponse{Error: "refresh_token is required"}, nil
	}

	// Tìm refresh token trong DB
	tokenHash := hashRefreshToken(req.RefreshToken)
	var storedToken models.RefreshToken
	if err := s.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&storedToken).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.RefreshTokenResponse{Error: "invalid refresh token"}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Msg("query refresh token failed")
		return nil, fmt.Errorf("query refresh token: %w", err)
	}

	// Check expiration
	if time.Now().After(storedToken.ExpiresAt) {
		// Xoá token hết hạn
		_ = s.db.WithContext(ctx).Delete(&storedToken)
		return &pb.RefreshTokenResponse{Error: "refresh token expired"}, nil
	}

	// Lấy user
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", storedToken.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.RefreshTokenResponse{Error: "user not found"}, nil
		}
		return nil, fmt.Errorf("query user: %w", err)
	}

	// Rotation: Xoá token cũ
	if err := s.db.WithContext(ctx).Delete(&storedToken).Error; err != nil {
		log.Warn().Err(err).Str("correlation_id", corrID).Msg("delete old refresh token failed")
	}

	// Tạo cặp token mới
	accessToken, newRefreshToken, expiresAt, err := s.GenerateTokensForUser(ctx, &user)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("generate new tokens failed")
		return &pb.RefreshTokenResponse{Error: "generate token failed"}, nil
	}

	return &pb.RefreshTokenResponse{
		AccessToken:   accessToken,
		RefreshToken:  newRefreshToken,
		ExpiresAtUnix: expiresAt.Unix(),
	}, nil
}

// RevokeToken thêm JWT vào blacklist dựa trên JTI (gRPC version).
func (s *AuthServiceServer) RevokeToken(ctx context.Context, req *pb.RevokeTokenRequest) (*pb.RevokeTokenResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().Str("correlation_id", corrID).Msg("RevokeToken called")

	if req.AccessToken == "" {
		return &pb.RevokeTokenResponse{}, nil
	}

	if err := s.revokeTokenInternal(ctx, req.AccessToken); err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Msg("revoke token failed")
		// Không trả lỗi cho client vì revoke thường là best-effort
	}

	return &pb.RevokeTokenResponse{}, nil
}

// revokeTokenInternal là logic nội bộ để blacklist token.
func (s *AuthServiceServer) revokeTokenInternal(ctx context.Context, token string) error {
	claims, err := s.parseToken(token)
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}
	if claims.ID == "" {
		return errors.New("token missing jti")
	}
	exp := claims.ExpiresAt
	if exp == nil {
		return errors.New("token missing expiry")
	}
	ttl := time.Until(exp.Time)
	if ttl <= 0 {
		return nil
	}
	key := blacklistKeyPrefix + claims.ID
	if err := s.redis.Set(ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("redis set blacklist: %w", err)
	}
	return nil
}

// GetGitHubToken trả về GitHub access token đã giải mã cho internal service use.
// Chỉ dùng nội bộ giữa các service (Gateway -> Auth -> Project/Runner).
func (s *AuthServiceServer) GetGitHubToken(ctx context.Context, req *pb.GetGitHubTokenRequest) (*pb.GetGitHubTokenResponse, error) {
	corrID := getCorrelationID(ctx)
	log.Info().Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("GetGitHubToken called")

	if req.UserId == "" {
		log.Warn().Str("correlation_id", corrID).Msg("GetGitHubToken called with empty user_id")
		return &pb.GetGitHubTokenResponse{Error: "user_id is required"}, nil
	}

	// Query user from DB
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", req.UserId).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Warn().Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("User not found when getting GitHub token")
			return &pb.GetGitHubTokenResponse{Error: "user not found"}, nil
		}
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("Database query failed when getting GitHub token")
		return nil, fmt.Errorf("query user: %w", err)
	}

	// Check if token exists
	if user.GithubTokenEncrypted == "" {
		log.Warn().Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("GitHub token not found in user record")
		return &pb.GetGitHubTokenResponse{Error: "github token not found"}, nil
	}

	// Decrypt GitHub token
	decryptedToken, err := cryptopkg.DecryptString(s.cfg.MasterEncryptionKey, user.GithubTokenEncrypted)
	if err != nil {
		log.Error().Err(err).Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("Failed to decrypt GitHub token")
		return &pb.GetGitHubTokenResponse{Error: "failed to decrypt token"}, nil
	}

	log.Info().Str("correlation_id", corrID).Str("user_id", req.UserId).Msg("Successfully retrieved GitHub token")
	return &pb.GetGitHubTokenResponse{
		GithubToken: decryptedToken,
	}, nil
}

// GenerateTokensForUser tạo JWT + refresh token và lưu vào DB.
func (s *AuthServiceServer) GenerateTokensForUser(ctx context.Context, user *models.User) (accessToken string, refreshToken string, expiresAt time.Time, err error) {
	if user == nil {
		return "", "", time.Time{}, fmt.Errorf("user nil")
	}

	now := time.Now().UTC()
	expiresAt = now.Add(s.cfg.JWTExpiration)
	jti := uuid.NewString()

	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Plan:     user.Plan,
		Avatar:   user.AvatarURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.ServiceName,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err = token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}

	refreshToken, err = generateRefreshToken()
	if err != nil {
		return "", "", time.Time{}, err
	}

	if err := s.storeRefreshToken(ctx, user.ID, refreshToken, now.Add(refreshTokenTTL)); err != nil {
		return "", "", time.Time{}, err
	}

	return accessToken, refreshToken, expiresAt, nil
}

// upsertUser tạo hoặc cập nhật user từ GitHub info.
func (s *AuthServiceServer) upsertUser(ctx context.Context, ghUser *oauth.GitHubUser, encryptedToken string) (*models.User, error) {
	var user models.User
	err := s.db.WithContext(ctx).Where("github_id = ?", ghUser.ID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = models.User{
			GithubID:             ghUser.ID,
			Username:             chooseNonEmpty(ghUser.Login, ghUser.Name),
			Email:                ghUser.Email,
			AvatarURL:            ghUser.AvatarURL,
			Plan:                 defaultPlan,
			GithubTokenEncrypted: encryptedToken,
		}
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	if err != nil {
		return nil, err
	}

	updateData := map[string]interface{}{
		"username":               chooseNonEmpty(ghUser.Login, user.Username),
		"email":                  chooseNonEmpty(ghUser.Email, user.Email),
		"avatar_url":             chooseNonEmpty(ghUser.AvatarURL, user.AvatarURL),
		"github_token_encrypted": encryptedToken,
	}
	if err := s.db.WithContext(ctx).Model(&user).Updates(updateData).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("github_id = ?", ghUser.ID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// parseToken helper parse JWT với custom claims.
func (s *AuthServiceServer) parseToken(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *AuthServiceServer) storeRefreshToken(ctx context.Context, userID string, token string, expires time.Time) error {
	hash := hashRefreshToken(token)
	data := &models.RefreshToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expires,
	}
	return s.db.WithContext(ctx).Where("token_hash = ?", hash).Assign(data).FirstOrCreate(data).Error
}

func generateRefreshToken() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func chooseNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
