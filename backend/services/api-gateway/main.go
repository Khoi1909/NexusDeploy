package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	cfgpkg "github.com/nexusdeploy/backend/pkg/config"
	grpcpkg "github.com/nexusdeploy/backend/pkg/grpc"
	"github.com/nexusdeploy/backend/pkg/logger"
	commonmw "github.com/nexusdeploy/backend/pkg/middleware"
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
		Address:     cfg.AuthServiceAddr,
		Timeout:     5 * time.Second,
		MaxRetries:  3,
		ServiceName: "auth-service",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to auth service")
	}
	defer authConn.Close()

	projectConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:     cfg.ProjectServiceAddr,
		Timeout:     5 * time.Second,
		MaxRetries:  3,
		ServiceName: "project-service",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to project service")
	}
	defer projectConn.Close()

	buildConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:     cfg.BuildServiceAddr,
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		ServiceName: "build-service",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to build service")
	}
	defer buildConn.Close()

	deploymentConn, err := grpcpkg.NewClient(ctx, grpcpkg.ClientConfig{
		Address:     cfg.DeploymentServiceAddr,
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		ServiceName: "deployment-service",
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to deployment service")
	}
	defer deploymentConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	projectClient := projectpb.NewProjectServiceClient(projectConn)
	buildClient := buildpb.NewBuildServiceClient(buildConn)
	deploymentClient := deploymentpb.NewDeploymentServiceClient(deploymentConn)

	// Handlers
	authHandler := handlers.NewAuthHandler(authClient)
	projectHandler := handlers.NewProjectHandler(projectClient)
	buildHandler := handlers.NewBuildHandler(buildClient)
	deploymentHandler := handlers.NewDeploymentHandler(deploymentClient, buildClient, projectClient)

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
			log.Info().
				Str(commonmw.CorrelationIDKey, commonmw.GetCorrelationID(r.Context())).
				Str("event", event.Event).
				Str("delivery_id", event.DeliveryID).
				Msg("Triggering build from GitHub webhook")
			// Note: In production, extract project_id from webhook payload
			// and call buildClient.TriggerBuild()
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
