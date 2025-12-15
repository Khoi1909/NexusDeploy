package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	grpcpkg "github.com/nexusdeploy/backend/pkg/grpc"
	"github.com/nexusdeploy/backend/pkg/logger"
	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
	aipb "github.com/nexusdeploy/backend/services/ai-service/proto"
	"github.com/nexusdeploy/backend/services/api-gateway/handlers"
	"github.com/nexusdeploy/backend/services/api-gateway/routes"
	authpb "github.com/nexusdeploy/backend/services/auth-service/proto"
	buildpb "github.com/nexusdeploy/backend/services/build-service/proto"
	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	projectpb "github.com/nexusdeploy/backend/services/project-service/proto"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := cfgpkg.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.ServiceName == "" || cfg.ServiceName == "unknown-service" {
		cfg.ServiceName = "api-gateway"
	}

	logger.InitLogger(cfg.ServiceName, cfg.LogLevel, cfg.LogFormat)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// gRPC clients
	authConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.AuthServiceAddr,
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		ServiceName:        "auth-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to auth service")
	}
	defer authConn.Close()

	projectConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.ProjectServiceAddr,
		Timeout:            5 * time.Second,
		MaxRetries:         3,
		ServiceName:        "project-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to project service")
	}
	defer projectConn.Close()

	buildConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.BuildServiceAddr,
		Timeout:            10 * time.Second,
		MaxRetries:         3,
		ServiceName:        "build-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to build service")
	}
	defer buildConn.Close()

	deploymentConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.DeploymentServiceAddr,
		Timeout:            10 * time.Second,
		MaxRetries:         3,
		ServiceName:        "deployment-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to deployment service")
	}
	defer deploymentConn.Close()

	aiConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:            cfg.AIServiceAddr,
		Timeout:            10 * time.Second,
		MaxRetries:         3,
		ServiceName:        "ai-service",
		TLSEnabled:         cfg.GRPCTLSEnabled,
		TLSCertPath:        cfg.GRPCTLSCertPath,
		InsecureSkipVerify: cfg.GRPCInsecureSkipVerify,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to ai service")
	}
	defer aiConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	projectClient := projectpb.NewProjectServiceClient(projectConn)
	buildClient := buildpb.NewBuildServiceClient(buildConn)
	deploymentClient := deploymentpb.NewDeploymentServiceClient(deploymentConn)
	aiClient := aipb.NewAIServiceClient(aiConn)

	// Handlers
	authHandler := handlers.NewAuthHandler(authClient, projectClient)
	projectHandler := handlers.NewProjectHandler(projectClient)
	buildHandler := handlers.NewBuildHandler(buildClient, aiClient)
	deploymentHandler := handlers.NewDeploymentHandler(deploymentClient, buildClient, projectClient, authClient)

	// Wire GetGitHubToken callback - Gateway fetches token from Auth Service
	// and passes to Project Service (frontend never sees the token)
	projectHandler.GetGitHubToken = func(ctx context.Context, userID string) (string, error) {
		resp, err := authClient.GetGitHubToken(ctx, &authpb.GetGitHubTokenRequest{
			UserId: userID,
		})
		if err != nil {
			return "", fmt.Errorf("get github token: %w", err)
		}
		if resp.Error != "" {
			return "", fmt.Errorf(resp.Error)
		}
		return resp.GithubToken, nil
	}

	webhookHandler := handlers.NewWebhookHandler(cfg.GitHubWebhookSecret, handlers.WebhookProcessorFunc(func(r *http.Request, event handlers.WebhookEvent) error {
		// Trigger build when receiving push event from GitHub
		if event.Event == "push" {
			corrID := commonmw.GetCorrelationID(r.Context())
			log.Info().
				Str(commonmw.CorrelationIDKey, corrID).
				Str("event", event.Event).
				Str("delivery_id", event.DeliveryID).
				Msg("Processing push event from GitHub webhook")

			// Parse webhook payload
			var payload struct {
				Repository struct {
					ID       int64  `json:"id"`
					FullName string `json:"full_name"`
					CloneURL string `json:"clone_url"`
				} `json:"repository"`
				HeadCommit struct {
					ID string `json:"id"`
				} `json:"head_commit"`
				Ref string `json:"ref"` // "refs/heads/branch-name"
			}

			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				log.Error().
					Err(err).
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Msg("Failed to parse webhook payload")
				return nil // Don't fail webhook, just log error
			}

			// Extract branch from ref (e.g., "refs/heads/main" -> "main")
			branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
			if branch == payload.Ref {
				// If ref doesn't start with refs/heads/, try refs/tags/ or just use as-is
				branch = strings.TrimPrefix(payload.Ref, "refs/tags/")
			}

			// Find project by repository
			ctx := r.Context()
			projectResp, err := projectClient.GetProjectByRepo(ctx, &projectpb.GetProjectByRepoRequest{
				RepoUrl:      payload.Repository.CloneURL,
				GithubRepoId: payload.Repository.ID,
			})
			if err != nil {
				log.Error().
					Err(err).
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("repo_full_name", payload.Repository.FullName).
					Int64("github_repo_id", payload.Repository.ID).
					Msg("Failed to find project by repository")
				return nil // Don't fail webhook
			}

			if projectResp.Error != "" || projectResp.Project == nil {
				log.Warn().
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("repo_full_name", payload.Repository.FullName).
					Str("error", projectResp.Error).
					Msg("Project not found for repository")
				return nil // Don't fail webhook
			}

			// Check if branch matches project's configured branch
			if projectResp.Project.Branch != "" && projectResp.Project.Branch != branch {
				log.Info().
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("project_id", projectResp.Project.Id).
					Str("project_branch", projectResp.Project.Branch).
					Str("push_branch", branch).
					Msg("Branch mismatch, skipping build trigger")
				return nil
			}

			// Trigger build
			if payload.HeadCommit.ID == "" {
				log.Warn().
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("project_id", projectResp.Project.Id).
					Msg("No head_commit found in payload, skipping build trigger")
				return nil
			}

			buildResp, err := buildClient.TriggerBuild(ctx, &buildpb.TriggerBuildRequest{
				ProjectId: projectResp.Project.Id,
				CommitSha: payload.HeadCommit.ID,
				Branch:    branch,
				RepoUrl:   payload.Repository.CloneURL,
			})
			if err != nil {
				log.Error().
					Err(err).
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("project_id", projectResp.Project.Id).
					Str("commit_sha", payload.HeadCommit.ID).
					Msg("Failed to trigger build from webhook")
				return nil // Don't fail webhook, GitHub will retry if we return error
			}

			if buildResp.Error != "" {
				log.Error().
					Str(commonmw.CorrelationIDKey, corrID).
					Str("delivery_id", event.DeliveryID).
					Str("project_id", projectResp.Project.Id).
					Str("error", buildResp.Error).
					Msg("Build Service returned error when triggering build")
				return nil
			}

			log.Info().
				Str(commonmw.CorrelationIDKey, corrID).
				Str("delivery_id", event.DeliveryID).
				Str("project_id", projectResp.Project.Id).
				Str("build_id", buildResp.Build.Id).
				Str("commit_sha", payload.HeadCommit.ID).
				Str("branch", branch).
				Msg("Successfully triggered build from webhook")
		}
		return nil
	}))

	// WebSocket proxy to notification service
	wsProxy := handlers.NewWebSocketProxy(log.Logger)

	router := routes.NewRouter(routes.RouterConfig{
		Config:            cfg,
		AuthClient:        authClient,
		AuthHandler:       authHandler,
		ProjectHandler:    projectHandler,
		BuildHandler:      buildHandler,
		DeploymentHandler: deploymentHandler,
		WebhookHandler:    webhookHandler,
		WebSocketProxy:    nil, // Don't register in router, handle separately
		RateLimit: routes.RateLimitConfig{
			RequestsPerWindow: 60,
			Window:            time.Minute,
		},
	})

	// Create top-level mux to handle WebSocket before middleware
	topMux := http.NewServeMux()
	if wsProxy != nil {
		topMux.HandleFunc("/ws", wsProxy.HandleWebSocket)
	}
	topMux.Handle("/", router)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:           topMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().
			Str("address", server.Addr).
			Msg("API Gateway listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info().Msg("Shutting down API Gateway...")
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Error().Err(err).Msg("HTTP server shutdown error")
	}
}
