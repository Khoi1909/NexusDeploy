package handlers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	"github.com/nexusdeploy/backend/services/auth-service/models"
	pb "github.com/nexusdeploy/backend/services/auth-service/proto"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:testdb-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	createStatements := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			github_id INTEGER NOT NULL UNIQUE,
			username TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			avatar_url TEXT,
			plan TEXT NOT NULL,
			github_token_encrypted TEXT,
			created_at DATETIME,
			updated_at DATETIME
		);`,
		`CREATE TABLE refresh_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME
		);`,
		`CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);`,
		`CREATE TABLE permissions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			resource TEXT NOT NULL,
			action TEXT NOT NULL
		);`,
		`CREATE INDEX idx_permissions_user ON permissions(user_id);`,
	}
	for _, stmt := range createStatements {
		require.NoError(t, db.Exec(stmt).Error)
	}
	return db
}

func setupTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestGenerateAndValidateToken(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &cfgpkg.Config{
		ServiceName:   "auth-service",
		JWTSecret:     "super-secret-key-which-is-long-enough",
		JWTExpiration: time.Minute,
	}

	server := NewAuthServiceServer(cfg, db, redisClient)

	user := &models.User{
		ID:                   "user-123",
		GithubID:             123456,
		Username:             "tester",
		Email:                "tester@example.com",
		AvatarURL:            "https://example.com/avatar.png",
		Plan:                 "standard",
		GithubTokenEncrypted: "encrypted-token",
	}
	require.NoError(t, db.Create(user).Error)

	access, refresh, expiresAt, err := server.GenerateTokensForUser(context.Background(), user)
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)
	require.WithinDuration(t, time.Now().Add(cfg.JWTExpiration), expiresAt, 3*time.Second)

	resp, err := server.ValidateToken(context.Background(), &pb.ValidateTokenRequest{Token: access})
	require.NoError(t, err)
	require.True(t, resp.Valid)
	require.Equal(t, user.ID, resp.UserId)
	require.Equal(t, user.Plan, resp.Plan)

	// Revoke token then validate again
	_, err = server.RevokeToken(context.Background(), &pb.RevokeTokenRequest{AccessToken: access})
	require.NoError(t, err)
	resp, err = server.ValidateToken(context.Background(), &pb.ValidateTokenRequest{Token: access})
	require.NoError(t, err)
	require.False(t, resp.Valid)
	require.Equal(t, "token revoked", resp.Error)
}

func TestCheckPermissionPlanRules(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &cfgpkg.Config{
		ServiceName:   "auth-service",
		JWTSecret:     "another-secret-key-1234567890",
		JWTExpiration: time.Minute,
	}
	server := NewAuthServiceServer(cfg, db, redisClient)

	user := &models.User{
		ID:                   "user-plan",
		GithubID:             999,
		Username:             "standard-user",
		Email:                "std@example.com",
		Plan:                 "standard",
		GithubTokenEncrypted: "enc",
	}
	require.NoError(t, db.Create(user).Error)

	resp, err := server.CheckPermission(context.Background(), &pb.CheckPermissionRequest{
		UserId:       user.ID,
		ResourceType: customDomainResource,
		ResourceId:   "domain",
		Action:       "use",
	})
	require.NoError(t, err)
	require.False(t, resp.Allowed)

	// Premium user should be allowed
	require.NoError(t, db.Model(&user).Update("plan", "premium").Error)
	resp, err = server.CheckPermission(context.Background(), &pb.CheckPermissionRequest{
		UserId:       user.ID,
		ResourceType: customDomainResource,
		ResourceId:   "domain",
		Action:       "use",
	})
	require.NoError(t, err)
	require.True(t, resp.Allowed)
}
