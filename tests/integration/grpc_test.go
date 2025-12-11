package integration

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
)

const (
	authServiceAddr    = "localhost:50051"
	projectServiceAddr = "localhost:50052"
	timeout            = 5 * time.Second
)

// TestAuthServiceValidateToken tests the ValidateToken RPC
func TestAuthServiceValidateToken(t *testing.T) {
	// Connect to auth-service
	conn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to auth-service: %v", err)
	}
	defer conn.Close()

	client := authpb.NewAuthServiceClient(conn)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Test with valid token
	req := &authpb.ValidateTokenRequest{
		Token: "test-token-123",
	}

	resp, err := client.ValidateToken(ctx, req)
	if err != nil {
		t.Fatalf("ValidateToken RPC failed: %v", err)
	}

	// Verify response
	if !resp.Valid {
		t.Error("Expected valid=true, got false")
	}

	if resp.UserId == "" {
		t.Error("Expected non-empty userId")
	}

	if resp.Plan == "" {
		t.Error("Expected non-empty plan")
	}

	t.Logf("ValidateToken response: valid=%v, userId=%s, plan=%s", resp.Valid, resp.UserId, resp.Plan)
}

// TestAuthServiceGetUserPlan tests the GetUserPlan RPC
func TestAuthServiceGetUserPlan(t *testing.T) {
	conn, err := grpc.Dial(authServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to auth-service: %v", err)
	}
	defer conn.Close()

	client := authpb.NewAuthServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &authpb.GetUserPlanRequest{
		UserId: "test-user-id",
	}

	resp, err := client.GetUserPlan(ctx, req)
	if err != nil {
		t.Fatalf("GetUserPlan RPC failed: %v", err)
	}

	// Verify response
	if resp.Plan == "" {
		t.Error("Expected non-empty plan")
	}

	if resp.MaxProjects <= 0 {
		t.Error("Expected MaxProjects > 0")
	}

	t.Logf("GetUserPlan response: plan=%s, maxProjects=%d, maxBuildsPerMonth=%d",
		resp.Plan, resp.MaxProjects, resp.MaxBuildsPerMonth)
}

// TestProjectServiceListProjects tests the ListProjects RPC
func TestProjectServiceListProjects(t *testing.T) {
	conn, err := grpc.Dial(projectServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to project-service: %v", err)
	}
	defer conn.Close()

	client := projectpb.NewProjectServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &projectpb.ListProjectsRequest{
		UserId: "test-user-id",
	}

	resp, err := client.ListProjects(ctx, req)
	if err != nil {
		t.Fatalf("ListProjects RPC failed: %v", err)
	}

	// Verify response structure
	if resp.Projects == nil {
		t.Error("Expected non-nil projects list")
	}

	if resp.Total < 0 {
		t.Error("Expected Total >= 0")
	}

	t.Logf("ListProjects response: total=%d, projects=%d", resp.Total, len(resp.Projects))
}

// TestProjectServiceCreateProject tests the CreateProject RPC (stub)
func TestProjectServiceCreateProject(t *testing.T) {
	conn, err := grpc.Dial(projectServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect to project-service: %v", err)
	}
	defer conn.Close()

	client := projectpb.NewProjectServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := &projectpb.CreateProjectRequest{
		UserId:   "test-user-id",
		Name:     "Test Project",
		RepoUrl:  "https://github.com/user/repo",
		Preset:   "nodejs",
		Branch:   "main",
	}

	resp, err := client.CreateProject(ctx, req)
	if err != nil {
		t.Fatalf("CreateProject RPC failed: %v", err)
	}

	// Verify response
	if resp.Project == nil {
		t.Error("Expected non-nil project")
	}

	if resp.Project.Id == "" {
		t.Error("Expected non-empty project ID")
	}

	t.Logf("CreateProject response: projectId=%s, name=%s", resp.Project.Id, resp.Project.Name)
}

